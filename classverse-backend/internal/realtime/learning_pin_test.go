package realtime

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestLearningPinAuthorizationAndState(t *testing.T) {
	r, _ := newMapRoom(t, "classroom", true)
	student := r.testJoin(t, "Student")
	drain(student)
	r.handle(commandCmd{c: student, name: "learning_pin", target: uuid.New()})
	if e := errorsOf(t, student); len(e) != 1 || e[0] != CodeForbidden {
		t.Fatalf("student pin=%v", e)
	}
	r.members[student.userID].p.Role = "teacher"
	r.pin = &LearningPin{ModuleID: uuid.New(), Title: "Belajar", BuildingKey: "sains", Level: "SMA"}
	other := r.testJoin(t, "Late join")
	messages := drain(other)[TypeRoomState]
	if len(messages) == 0 {
		t.Fatal("late join missing pin state")
	}
	var state RoomStateMsg
	if err := json.Unmarshal(messages[len(messages)-1], &state); err != nil {
		t.Fatal(err)
	}
	if state.Pin == nil || state.Pin.ModuleID != r.pin.ModuleID {
		t.Fatalf("pin=%+v", state.Pin)
	}
	r.applyLearningPin(learningPinResult{c: student, pin: nil})
	if r.pin != nil {
		t.Fatal("unpin retained stale link")
	}
	campus, _ := newMapRoom(t, "campus", true)
	teacher := campus.testJoin(t, "Teacher")
	campus.members[teacher.userID].p.Role = "teacher"
	drain(teacher)
	campus.handle(commandCmd{c: teacher, name: "learning_pin", target: uuid.New()})
	if e := errorsOf(t, teacher); len(e) != 1 || e[0] != CodeForbidden {
		t.Fatalf("campus pin=%v", e)
	}
}
