package turn

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rcarmo/gi/internal/inference"
	"github.com/rcarmo/gi/internal/store"
	goai "github.com/rcarmo/go-ai"
)

// A Steer request arriving while the original run still owns cleanup must not
// claim its selected row as a second turn. The normal cleanup handoff may
// deliver that row once, even when the request itself reports a conflict.
func TestQueueSteerDuringCleanupKeepsDeliveryOwnedByHandoff(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/gi.db"
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	other, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	const sid = "steer-during-cleanup"
	if _, err := s.CreateSession(ctx, sid, sid, map[string]any{"model": "mock-tool", "status": "idle"}); err != nil {
		t.Fatal(err)
	}
	started, cancelled, releaseProvider := make(chan struct{}), make(chan struct{}), make(chan struct{})
	cleanupHeld, releaseCleanup := make(chan struct{}), make(chan struct{})
	var providerRelease, cleanupRelease sync.Once
	defer providerRelease.Do(func() { close(releaseProvider) })
	defer cleanupRelease.Do(func() { close(releaseCleanup) })
	var first sync.Once
	withStreamWithToolsStub(t, func(ctx context.Context, _ string, _ *goai.Context, _ func(map[string]any)) (*inference.StreamResult, error) {
		initial := false
		first.Do(func() { initial = true })
		if initial {
			close(started)
			<-ctx.Done()
			close(cancelled)
			<-releaseProvider
			return nil, ctx.Err()
		}
		return &inference.StreamResult{Message: &goai.Message{Role: goai.RoleAssistant, StopReason: goai.StopReasonStop, Content: []goai.ContentBlock{{Type: "text", Text: "done"}}}}, nil
	})
	e := New(s)
	defer e.Close()
	e.beforeCleanupNextWorkHook = func(_ context.Context, sessionID string) {
		if sessionID != sid {
			return
		}
		select {
		case <-cleanupHeld:
		default:
			close(cleanupHeld)
		}
		<-releaseCleanup
	}
	active, err := e.SubmitPrompt(ctx, RunInput{SessionID: sid, Prompt: "held", Model: "mock-tool"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("provider did not start")
	}
	media, err := s.CreateMedia(ctx, sid, "queued.txt", "text/plain", []byte("kept"), nil)
	if err != nil {
		t.Fatal(err)
	}
	queued, err := e.SubmitPrompt(ctx, RunInput{SessionID: sid, Prompt: "selected", Intent: "queue", Model: "mock-tool", Metadata: map[string]any{"probe": "retain", "media": []any{map[string]any{"media_id": media.ID, "id": fmt.Sprintf("media:%d", media.ID), "session_id": sid}}}})
	if err != nil || !queued.Queued {
		t.Fatalf("queue admission=%#v: %v", queued, err)
	}
	if err := e.CancelActiveTurn(ctx, sid, active.TurnID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("provider did not observe cancellation")
	}
	// Before worker teardown the observed run still owns the claim. A
	// distinct frontend must fail closed and leave the queued row intact.
	competitor := New(other)
	defer competitor.Close()
	if err := competitor.SteerQueuedTurn(ctx, sid, queued.TurnID, active.TurnID); !errors.Is(err, store.ErrQueueConflict) {
		t.Fatalf("Steer accepted while cleanup still owns claim: %v", err)
	}
	if turn, err := s.GetTurn(ctx, queued.TurnID); err != nil || turn.Status != "queued" || turn.Metadata["media"] == nil {
		t.Fatalf("conflicting Steer changed queued text/media: %#v %v", turn, err)
	}
	providerRelease.Do(func() { close(releaseProvider) })
	select {
	case <-cleanupHeld:
	case <-time.After(5 * time.Second):
		t.Fatal("cleanup did not hold before handoff")
	}
	// After claim release, but before automatic handoff, the same Steer
	// request may claim and launch the selected row. No in-memory mutex can
	// stand in for the durable claim fence across the two engines.
	if err := competitor.SteerQueuedTurn(ctx, sid, queued.TurnID, active.TurnID); err != nil {
		t.Fatalf("released observed run did not launch the selected row: %v", err)
	}
	if err := competitor.SteerQueuedTurn(ctx, sid, queued.TurnID, active.TurnID); !errors.Is(err, store.ErrQueueConflict) {
		t.Fatalf("duplicate Steer did not conflict: %v", err)
	}
	cleanupRelease.Do(func() { close(releaseCleanup) })
	waitForCondition(t, 5*time.Second, func() bool {
		turn, err := s.GetTurn(ctx, queued.TurnID)
		_, _, claimErr := s.GetSessionActiveTurn(ctx, sid)
		return err == nil && turn.Status == "completed" && claimErr == sql.ErrNoRows
	}, "selected queued turn delivered once")
	messages, err := s.ListMessages(ctx, sid)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, message := range messages {
		if message.Role == "user" && message.Content == "selected" {
			count++
			if message.Payload["turn_id"] != queued.TurnID || message.Payload["media"] == nil {
				t.Fatalf("selected row identity/media lost: %#v", message)
			}
		}
	}
	if count != 1 {
		t.Fatalf("selected prompt delivered %d times", count)
	}
}
