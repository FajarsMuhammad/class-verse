package realtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

// LearningPinner persists and validates catalog links in the open room session.
type LearningPinner interface {
	SaveLearningPin(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) (*LearningPin, error)
}
type learningPinJob struct {
	roomID, actor, module, step uuid.UUID
	done                        func(*LearningPin, error)
}
type learningPinResult struct {
	c   *conn
	seq uint64
	pin *LearningPin
	err error
}

func (r *room) pinLearning(m *member, c commandCmd, reject func(string, string)) {
	if r.campus {
		reject(CodeForbidden, "Sematan hanya tersedia di ruang")
		return
	}
	module, step := c.target, c.step
	if c.name == "learning_unpin" {
		module, step = uuid.Nil, uuid.Nil
	} else if module == uuid.Nil {
		reject(CodeBadMessage, "Pilih modul terbit")
		return
	}
	r.hub.storeLearningPin(learningPinJob{roomID: r.id, actor: m.p.ID, module: module, step: step, done: func(pin *LearningPin, err error) {
		// Reply must reach the actor; never mutate its state from the worker.
		select {
		case r.inbox <- learningPinResult{c: c.c, seq: c.seq, pin: pin, err: err}:
		case <-r.done:
		default:
			r.log.Warn("learning pin reply dropped: room inbox full")
		}
	}})
}
func (r *room) applyLearningPin(c learningPinResult) {
	if c.err != nil {
		if m := r.members[c.c.userID]; m != nil && m.conn == c.c {
			c.c.sendError(ErrorMsg{Code: CodeForbidden, Message: "Sematan gagal. Pilih modul terbit dan langkah yang sesuai.", Seq: c.seq})
		}
		return
	}
	r.pin = c.pin
	r.sendStates()
}
func (h *Hub) storeLearningPin(j learningPinJob) {
	if h.sessions == nil {
		j.done(nil, errUnavailable)
		return
	}
	select {
	case h.jobs <- sessionJob{kind: jobLearningPin, roomID: j.roomID, pin: &j}:
	default:
		j.done(nil, errUnavailable)
	}
}

// SaveLearningPin validates organization, role, publication and step pairing inside the update.
func (s *Store) SaveLearningPin(ctx context.Context, room, actor, module, step uuid.UUID) (*LearningPin, error) {
	n, err := s.q.SaveLearningPin(ctx, query.SaveLearningPinParams{RoomID: room, ActorID: actor, ModuleID: uuid.NullUUID{UUID: module, Valid: module != uuid.Nil}, StepID: uuid.NullUUID{UUID: step, Valid: step != uuid.Nil}})
	if err != nil {
		return nil, fmt.Errorf("save learning pin: %w", err)
	}
	if n != 1 {
		return nil, ErrNoAccess
	}
	if module == uuid.Nil {
		return nil, nil
	}
	return s.learningPin(ctx, room)
}
func (s *Store) learningPin(ctx context.Context, room uuid.UUID) (*LearningPin, error) {
	p, err := s.q.GetLearningPin(ctx, room)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &LearningPin{ModuleID: p.PinModuleID.UUID, StepID: p.PinStepID.UUID, Title: p.Title, BuildingKey: p.BuildingKey, Level: p.Level}, nil
}
