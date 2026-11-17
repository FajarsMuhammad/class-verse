package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/maps"
)

// Opt-in loopback transport test. Auth/admission are fixtures; no DB, TLS or voice.
func TestWebSocketLoad500(t *testing.T) {
	if os.Getenv("KV_LOAD_TEST") != "1" {
		t.Skip("set KV_LOAD_TEST=1 for 500 real loopback WebSockets")
	}
	for _, rooms := range []int{1, 13} {
		t.Run(fmt.Sprintf("rooms=%d", rooms), func(t *testing.T) { runSocketLoad(t, rooms) })
	}
}

type loadClient struct {
	conn *websocket.Conn
	pos  MoveMsg
	dx   int
}

func runSocketLoad(t *testing.T, rooms int) {
	t.Helper()
	raw, err := maps.Read("campus")
	if err != nil {
		t.Fatal(err)
	}
	md, err := ParseMap("campus", raw)
	if err != nil {
		t.Fatal(err)
	}
	if rooms > 1 {
		md.Key = "classroom"
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := NewHub(map[string]*MapData{md.Key: md}, nil, time.Now, log)
	defer func() { _ = hub.Shutdown(context.Background()) }()
	tokens := fakeTokens{}
	admits := admissionMap{}
	roomIDs := make([]uuid.UUID, rooms)
	for i := range roomIDs {
		roomIDs[i] = uuid.New()
		capacity := 500
		if rooms > 1 {
			capacity = 40
		}
		admits[roomIDs[i]] = Admission{RoomID: roomIDs[i], MapKey: md.Key, Capacity: capacity, AllChat: true}
	}
	for i := 0; i < 500; i++ {
		tokens[fmt.Sprint(i)] = uuid.New()
	}
	gw, err := NewGateway(hub, tokens, admits, testOrigin, log)
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	gw.Routes(router)
	server := httptest.NewServer(router)
	defer server.Close()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	clients := make([]loadClient, 0, 500)
	var readers sync.WaitGroup
	var bytes, snaps, failures, rejects atomic.Int64
	var latencyMu sync.Mutex
	var latencies []time.Duration
	defer func() {
		cancel()
		for _, c := range clients {
			_ = c.conn.CloseNow()
		}
		readers.Wait()
	}()
	for i := 0; i < 500; i++ {
		if rooms > 1 {
			hub.setLocation(tokens[fmt.Sprint(i)], roomIDs[i%rooms])
		}
		c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/ws/rooms/"+roomIDs[i%rooms].String(), &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{testOrigin}}})
		if err != nil {
			t.Fatal(err)
		}
		c.SetReadLimit(1 << 20)
		send(t, c, TypeAuth, 1, AuthMsg{Token: fmt.Sprint(i)})
		env, err := next(t, c, TypeWelcome)
		if err != nil {
			_ = c.CloseNow()
			t.Fatal(err)
		}
		var w WelcomeMsg
		if err = json.Unmarshal(env.D, &w); err != nil {
			t.Fatal(err)
		}
		var pos MoveMsg
		for _, p := range w.Participants {
			if p.ID == tokens[fmt.Sprint(i)] {
				pos = MoveMsg{X: p.X, Y: p.Y, Dir: DirRight}
				break
			}
		}
		dx := 8
		if !md.PathClear(pos.X, pos.Y, pos.X+dx, pos.Y) {
			dx = -8
		}
		if !md.PathClear(pos.X, pos.Y, pos.X+dx, pos.Y) {
			t.Fatal("fixture spawn has no safe walk")
		}
		clients = append(clients, loadClient{c, pos, dx})
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				_, raw, err := c.Read(ctx)
				if err != nil {
					if ctx.Err() == nil {
						failures.Add(1)
					}
					return
				}
				bytes.Add(int64(len(raw)))
				var env Envelope
				if json.Unmarshal(raw, &env) != nil {
					failures.Add(1)
					continue
				}
				if env.T == TypePong {
					var p PongMsg
					if json.Unmarshal(env.D, &p) == nil {
						latencyMu.Lock()
						latencies = append(latencies, time.Since(time.Unix(0, p.TS)))
						latencyMu.Unlock()
					}
				}
				if env.T == TypeSnapshot {
					snaps.Add(1)
				}
				if env.T == TypeError {
					rejects.Add(1)
				}
			}
		}()
	}
	bytes.Store(0)
	snaps.Store(0)
	start := time.Now()
	var writers sync.WaitGroup
	for _, client := range clients {
		writers.Add(1)
		go func(c loadClient) {
			defer writers.Done()
			ticker := time.NewTicker(80 * time.Millisecond)
			defer ticker.Stop()
			stop := time.NewTimer(30 * time.Second)
			defer stop.Stop()
			seq := uint64(2)
			forward := true
			for {
				select {
				case <-ctx.Done():
					return
				case <-stop.C:
					return
				case <-ticker.C:
					p := c.pos
					if forward {
						p.X += c.dx
					}
					forward = !forward
					data, _ := json.Marshal(p)
					raw, _ := json.Marshal(Envelope{T: TypeMove, Seq: seq, D: data})
					seq++
					writeCtx, done := context.WithTimeout(ctx, 2*time.Second)
					err := c.conn.Write(writeCtx, websocket.MessageText, raw)
					if err == nil && seq%12 == 0 {
						data, _ = json.Marshal(PingMsg{TS: time.Now().UnixNano()})
						raw, _ = json.Marshal(Envelope{T: TypePing, Seq: seq, D: data})
						seq++
						err = c.conn.Write(writeCtx, websocket.MessageText, raw)
					}
					done()
					if err != nil {
						failures.Add(1)
						return
					}
				}
			}
		}(client)
	}
	writers.Wait()
	latencyMu.Lock()
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	if len(latencies) > 0 {
		t.Logf("ping round-trip p50=%s p95=%s p99=%s (%d samples)", latencies[len(latencies)/2], latencies[len(latencies)*95/100], latencies[len(latencies)*99/100], len(latencies))
	}
	latencyMu.Unlock()
	elapsed := time.Since(start).Seconds()
	t.Logf("500 sockets, %d rooms: %.2fs, %.2f MB/s received JSON, %.0f snapshots/s, disconnect/write errors=%d, rejected commands=%d", rooms, elapsed, float64(bytes.Load())/elapsed/1e6, float64(snaps.Load())/elapsed, failures.Load(), rejects.Load())
	if failures.Load() != 0 || rejects.Load() != 0 {
		t.Fatal("load test had connection/command failures")
	}
}
