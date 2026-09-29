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

// Selection by cleanup is not a claim. A second store connection may still
// restore the queued text while cleanup pauses just before its claim; cleanup
// must then decline to launch the cancelled row.
func TestEscapeRestoreDuringCleanupPreClaimHandoff(t *testing.T) {
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
	const sid = "escape-claim-handoff"
	if _, err := s.CreateSession(ctx, sid, sid, map[string]any{"model": "mock-tool", "status": "idle"}); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	releaseProvider := make(chan struct{})
	releaseClaim := make(chan struct{})
	claimStarted := make(chan struct{})
	var closeProvider sync.Once
	var closeClaim sync.Once
	defer closeProvider.Do(func() { close(releaseProvider) })
	defer closeClaim.Do(func() { close(releaseClaim) })
	var firstRequest sync.Once
	withStreamWithToolsStub(t, func(ctx context.Context, _ string, _ *goai.Context, _ func(map[string]any)) (*inference.StreamResult, error) {
		first := false
		firstRequest.Do(func() { first = true })
		if !first {
			return &inference.StreamResult{Message: &goai.Message{Role: goai.RoleAssistant, StopReason: goai.StopReasonStop, Content: []goai.ContentBlock{{Type: "text", Text: "done"}}}}, nil
		}
		close(started)
		<-ctx.Done()
		<-releaseProvider
		return nil, ctx.Err()
	})
	e := New(s)
	defer e.Close()
	queuedID := ""
	e.beforeLaunchClaimHook = func(_ context.Context, _, turnID string) {
		if turnID == queuedID && queuedID != "" {
			select {
			case <-claimStarted:
			default:
				close(claimStarted)
			}
			<-releaseClaim
		}
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
	queued, err := e.SubmitPrompt(ctx, RunInput{SessionID: sid, Prompt: "follow-up", Intent: "queue", Model: "mock-tool"})
	if err != nil || !queued.Queued {
		t.Fatalf("queue admission=%#v: %v", queued, err)
	}
	queuedID = queued.TurnID
	draft, err := other.SaveTUITextDraft(ctx, sid, 0, store.TUITextSnapshot{Text: "draft", Cursor: 5})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.CancelActiveTurn(ctx, sid, active.TurnID); err != nil {
		t.Fatal(err)
	}
	closeProvider.Do(func() { close(releaseProvider) })
	select {
	case <-claimStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("cleanup did not select queued turn")
	}
	updated, ids, err := other.RestoreQueuedTUITextDraft(ctx, sid, draft)
	if err != nil || len(ids) != 1 || ids[0] != queued.TurnID || updated.Text != "follow-up\n\ndraft" {
		t.Fatalf("restore during pre-claim handoff=%#v ids=%q: %v", updated, ids, err)
	}
	closeClaim.Do(func() { close(releaseClaim) })
	waitForCondition(t, 5*time.Second, func() bool {
		turn, err := s.GetTurn(ctx, queued.TurnID)
		_, _, claimErr := s.GetSessionActiveTurn(ctx, sid)
		return err == nil && turn.Status == "cancelled" && claimErr == sql.ErrNoRows
	}, "restored turn never launched after cleanup selection")
	saved, err := other.LoadTUITextDraft(ctx, sid)
	if err != nil || saved.Text != "follow-up\n\ndraft" {
		t.Fatalf("restored text not durable=%#v: %v", saved, err)
	}
	messages, err := s.ListMessages(ctx, sid)
	if err != nil {
		t.Fatal(err)
	}
	for _, message := range messages {
		if message.Role == "user" && message.Content == "follow-up" {
			t.Fatalf("restored follow-up redelivered after cleanup selection: %#v", message)
		}
	}
}

func TestEscapeRestoreRejectsOtherFrontendClaim(t *testing.T) {
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
	const sid = "escape-foreign-claim"
	if _, err := s.CreateSession(ctx, sid, sid, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTurnWithStatus(ctx, "follow-up", sid, "queued", "queued text", nil); err != nil {
		t.Fatal(err)
	}
	draft, err := s.SaveTUITextDraft(ctx, sid, 0, store.TUITextSnapshot{Text: "draft", Cursor: 5})
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := other.ClaimSessionActiveTurn(ctx, sid, "follow-up", "foreign-worker", "foreign-claim")
	if err != nil || !claimed {
		t.Fatalf("foreign claim=%t: %v", claimed, err)
	}
	if _, _, err := s.RestoreQueuedTUITextDraft(ctx, sid, draft); !errors.Is(err, store.ErrQueueConflict) {
		t.Fatalf("restore after other frontend claim: %v", err)
	}
	turn, err := s.GetTurn(ctx, "follow-up")
	if err != nil || turn.Status != "queued" {
		t.Fatalf("claimed delivery changed=%#v: %v", turn, err)
	}
	stored, err := s.LoadTUITextDraft(ctx, sid)
	if err != nil || stored.Text != "draft" {
		t.Fatalf("draft changed on conflict=%#v: %v", stored, err)
	}
}
