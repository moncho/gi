package store

import (
	"context"
	"reflect"
	"testing"
)

func TestListPendingTUIMessagesOrdersAndScopesDelivery(t *testing.T) {
	ctx := context.Background()
	s, err := Open("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, id := range []string{"pending-display", "other-display"} {
		if _, err := s.CreateSession(ctx, id, id, nil); err != nil {
			t.Fatal(err)
		}
	}
	const sid = "pending-display"
	for _, row := range []struct{ id, prompt string }{{"first", "first\nline"}, {"second", "second"}} {
		if _, err := s.CreateTurnWithStatus(ctx, row.id, sid, "queued", row.prompt, nil); err != nil {
			t.Fatal(err)
		}
	}
	for _, text := range []string{"steer one", "steer two"} {
		if _, err := s.EnqueueSteering(ctx, sid, "", "user", text, nil, nil, "one-at-a-time"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.CreateTurnWithStatus(ctx, "other", "other-display", "queued", "foreign", nil); err != nil {
		t.Fatal(err)
	}
	check := func(want []PendingTUIMessage) {
		t.Helper()
		got, err := s.ListPendingTUIMessages(ctx, sid)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("pending=%#v want=%#v: %v", got, want, err)
		}
	}
	initial := []PendingTUIMessage{{"Steering", "steer one"}, {"Steering", "steer two"}, {"Follow-up", "first\nline"}, {"Follow-up", "second"}}
	check(initial)
	if err := s.ReorderQueuedTurns(ctx, sid, []string{"first", "second"}, []string{"second", "first"}); err != nil {
		t.Fatal(err)
	}
	check([]PendingTUIMessage{initial[0], initial[1], initial[3], initial[2]})
	if err := s.CancelQueuedTurn(ctx, sid, "second"); err != nil {
		t.Fatal(err)
	}
	check([]PendingTUIMessage{initial[0], initial[1], initial[2]})
	// A claimed or delivered steering row has left the pending queue. An
	// internal continuation with no prompt is not an editable follow-up.
	if _, err := s.db.ExecContext(ctx, `update steering_queue set status='claimed' where content='steer one'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTurnWithStatus(ctx, "continuation", sid, "queued", "", map[string]any{"continue": true}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTurnWithStatus(ctx, "media-only", sid, "queued", "", map[string]any{"media": []any{map[string]any{"media_id": 7, "session_id": sid}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnqueueSteering(ctx, sid, "", "user", "", map[string]any{"media": []any{map[string]any{"media_id": 8, "session_id": sid}}}, nil, "one-at-a-time"); err != nil {
		t.Fatal(err)
	}
	check([]PendingTUIMessage{initial[1], {Kind: "Steering", Text: "[attachment]"}, initial[2], {Kind: "Follow-up", Text: "[attachment]"}})
	if _, err := s.db.ExecContext(ctx, `update steering_queue set status='dequeued' where content='steer two'`); err != nil {
		t.Fatal(err)
	}
	check([]PendingTUIMessage{{Kind: "Steering", Text: "[attachment]"}, initial[2], {Kind: "Follow-up", Text: "[attachment]"}})
	other, err := s.ListPendingTUIMessages(ctx, "other-display")
	if err != nil || !reflect.DeepEqual(other, []PendingTUIMessage{{"Follow-up", "foreign"}}) {
		t.Fatalf("other session=%#v: %v", other, err)
	}
}
