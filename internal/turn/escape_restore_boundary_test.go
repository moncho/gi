package turn

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rcarmo/gi/internal/inference"
	"github.com/rcarmo/gi/internal/store"
	goai "github.com/rcarmo/go-ai"
)

// Cancellation requests return before the provider acknowledges cancellation.
// The active claim still owns the run and cleanup has not handed off queued
// work. A TUI Escape handler cannot infer a restoration-safe boundary from a
// successful CancelActiveTurn call.
func TestCancelActiveTurnReturnsBeforeWorkerCleanupAndRestore(t *testing.T) {
	for _, tc := range []struct {
		name         string
		withSteering bool
	}{{"text-only", false}, {"pending-steering", true}} {
		t.Run(tc.name, func(t *testing.T) {
			testCancelRestoreBoundary(t, tc.withSteering)
		})
	}
}

func testCancelRestoreBoundary(t *testing.T, withSteering bool) {
	ctx := context.Background()
	s := openTestStore(t)
	defer s.Close()
	const sid = "escape-restore-boundary"
	if _, err := s.CreateSession(ctx, sid, sid, map[string]any{"model": "mock-tool", "status": "idle"}); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	cancelled := make(chan struct{})
	release := make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	var firstRequest sync.Once
	withStreamWithToolsStub(t, func(ctx context.Context, _ string, _ *goai.Context, _ func(map[string]any)) (*inference.StreamResult, error) {
		first := false
		firstRequest.Do(func() { first = true })
		if !first {
			return &inference.StreamResult{Message: &goai.Message{Role: goai.RoleAssistant, StopReason: goai.StopReasonStop, Content: []goai.ContentBlock{{Type: "text", Text: "done"}}}}, nil
		}
		select {
		case <-started:
		default:
			close(started)
		}
		<-ctx.Done()
		close(cancelled)
		<-release // model/tool teardown may take time after cancellation
		return nil, ctx.Err()
	})
	e := New(s)
	defer e.Close()
	active, err := e.SubmitPrompt(ctx, RunInput{SessionID: sid, Prompt: "held request", Model: "mock-tool"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("active provider did not start")
	}
	queued, err := e.SubmitPrompt(ctx, RunInput{SessionID: sid, Prompt: "later follow-up", Intent: "queue", Model: "mock-tool"})
	if err != nil || !queued.Queued {
		t.Fatalf("follow-up admission=%#v: %v", queued, err)
	}
	if withSteering {
		steered, err := e.SubmitPrompt(ctx, RunInput{SessionID: sid, Prompt: "pending steer", Model: "mock-tool"})
		if err != nil || steered.Queued || steered.TurnID != active.TurnID {
			t.Fatalf("active steering admission=%#v: %v", steered, err)
		}
	}
	if err := e.CancelActiveTurn(ctx, sid, active.TurnID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("cancellation not observed by provider")
	}
	claim, _, err := s.GetSessionActiveTurn(ctx, sid)
	if err != nil || claim != active.TurnID {
		t.Fatalf("cancel returned before provider release, but claim=%q: %v", claim, err)
	}
	first, err := s.GetTurn(ctx, active.TurnID)
	if err != nil || first.Status != "cancelling" {
		t.Fatalf("active status after cancel=%#v: %v", first, err)
	}
	follow, err := s.GetTurn(ctx, queued.TurnID)
	if err != nil || follow.Status != "queued" {
		t.Fatalf("queued work before cleanup=%#v: %v", follow, err)
	}
	// The existing text-only restore transaction does not fence on a
	// cancelling worker: a naive Escape cancel-then-restore succeeds while
	// that worker is still able to finish and own cleanup.
	draft, err := s.SaveTUITextDraft(ctx, sid, 0, store.TUITextSnapshot{Text: "newer draft", Cursor: 11})
	if err != nil {
		t.Fatal(err)
	}
	restored, ids, err := s.RestoreQueuedTUITextDraft(ctx, sid, draft)
	followStatus := "cancelled"
	if withSteering {
		if !errors.Is(err, store.ErrQueueConflict) || len(ids) != 0 {
			t.Fatalf("active Steer must reject restore, got %#v ids=%q: %v", restored, ids, err)
		}
		followStatus = "completed"
		if saved, loadErr := s.LoadTUITextDraft(ctx, sid); loadErr != nil || saved.Text != "newer draft" {
			t.Fatalf("conflicting restore changed draft=%#v: %v", saved, loadErr)
		}
	} else if err != nil || len(ids) != 1 || ids[0] != queued.TurnID || restored.Text != "later follow-up\n\nnewer draft" {
		t.Fatalf("unfenced restore=%#v ids=%q: %v", restored, ids, err)
	}
	follow, err = s.GetTurn(ctx, queued.TurnID)
	if err != nil || follow.Status != map[bool]string{true: "queued", false: "cancelled"}[withSteering] {
		t.Fatalf("follow-up status before cleanup=%#v: %v", follow, err)
	}
	close(release)
	waitForCondition(t, 5*time.Second, func() bool {
		first, firstErr := s.GetTurn(ctx, active.TurnID)
		follow, followErr := s.GetTurn(ctx, queued.TurnID)
		_, _, claimErr := s.GetSessionActiveTurn(ctx, sid)
		depth, depthErr := s.SteeringQueueLength(ctx, sid)
		return firstErr == nil && followErr == nil && depthErr == nil && depth == 0 && first.Status == "cancelled" && follow.Status == followStatus && claimErr == sql.ErrNoRows
	}, "cancelled run and queue cleanup")
	messages, err := s.ListMessages(ctx, sid)
	if err != nil {
		t.Fatal(err)
	}
	followUsers, steeringUsers := 0, 0
	for _, message := range messages {
		if message.Role != "user" {
			continue
		}
		switch message.Content {
		case "later follow-up":
			followUsers++
		case "pending steer":
			steeringUsers++
		}
	}
	if withSteering {
		if followUsers != 1 || steeringUsers != 1 {
			t.Fatalf("failed restore must deliver both queued items once: follow=%d steer=%d", followUsers, steeringUsers)
		}
	} else if followUsers != 0 || steeringUsers != 0 {
		t.Fatalf("restored queued turn was delivered: follow=%d steer=%d", followUsers, steeringUsers)
	}
}
