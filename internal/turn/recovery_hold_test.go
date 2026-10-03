package turn

import (
	"context"
	"testing"
	"time"

	goai "github.com/rcarmo/go-ai"

	"github.com/rcarmo/gi/internal/inference"
)

// A turn interrupted after it may have sent a request or run tools is held
// for /retry instead of being replayed on the next start (#21); a turn that
// had not started work yet is requeued.
func TestStartupRecoveryHoldsInterruptedTurns(t *testing.T) {
	s := openTestStore(t)
	defer s.Close()
	ctx := context.Background()
	calls := make(chan string, 4)
	withStreamWithToolsStub(t, func(ctx context.Context, modelID string, convCtx *goai.Context, broadcast func(map[string]any)) (*inference.StreamResult, error) {
		calls <- goai.GetTextContent(&convCtx.Messages[len(convCtx.Messages)-1])
		return &inference.StreamResult{Message: &goai.Message{Role: goai.RoleAssistant, StopReason: goai.StopReasonStop, Content: []goai.ContentBlock{{Type: "text", Text: "ok"}}}}, nil
	})
	stale := func(session, turn, phase string, started bool) {
		t.Helper()
		if _, err := s.CreateSession(ctx, session, "Test", map[string]any{"model": "bootstrap", "status": "running"}); err != nil {
			t.Fatal(err)
		}
		rec, err := s.CreateTurnWithStatus(ctx, turn, session, "running", "prompt "+turn, map[string]any{"intent": "prompt", "model": "mock-recover"})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.UpdateTurnStatusAndPhase(ctx, rec.ID, "running", phase); err != nil {
			t.Fatal(err)
		}
		if started {
			if err := s.AppendTurnEvent(ctx, rec.ID, session, "tool.started", map[string]any{"tool": "shell"}); err != nil {
				t.Fatal(err)
			}
		}
		if ok, err := s.ClaimSessionActiveTurn(ctx, session, rec.ID, "runner", rec.ID); err != nil || !ok {
			t.Fatalf("claim: %v %v", ok, err)
		}
		if _, err := s.DB().ExecContext(ctx, `update session_active_turns set updated_at = '2000-01-01T00:00:00Z' where session_id = ?`, session); err != nil {
			t.Fatal(err)
		}
	}
	stale("s_inference", "t_inference", "inference", false)
	stale("s_compact", "t_compact", "compacting", true)
	stale("s_setup", "t_setup", "setup", false)

	e := New(s)
	defer e.Close()
	for _, id := range []string{"t_inference", "t_compact"} {
		rec, err := s.GetTurn(ctx, id)
		if err != nil || rec.Status != "failed" || rec.Phase != "held_for_retry_or_skip" {
			t.Fatalf("%s: %+v %v", id, rec, err)
		}
	}
	f, err := s.GetTurnFailure(ctx, "t_inference")
	if err != nil || f.FailureKind != InterruptedFailureKind || f.HoldState == "none" {
		t.Fatalf("failure %+v %v", f, err)
	}
	// Only the turn that had not started is replayed.
	select {
	case prompt := <-calls:
		if prompt != "prompt t_setup" {
			t.Fatalf("replayed %q", prompt)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("setup-phase turn was not requeued")
	}
	select {
	case prompt := <-calls:
		t.Fatalf("held turn replayed: %q", prompt)
	case <-time.After(300 * time.Millisecond):
	}
}
