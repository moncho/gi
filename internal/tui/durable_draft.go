package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rcarmo/gi/internal/store"
)

type terminalDraftState struct {
	pair   store.TUIComposerDraft
	local  store.TUITextSnapshot
	frozen bool
	err    error
}

func (c *chatTUI) draftContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Second)
}
func (c *chatTUI) editorSnapshot() store.TUITextSnapshot {
	return store.TUITextSnapshot{Text: c.input.Text(), Cursor: c.input.cursorPos}
}
func (c *chatTUI) draftNotice(message string) { c.showQueueCommand([]string{"draft: " + message}) }
func (c *chatTUI) setDraftError(d *terminalDraftState, err error) {
	if d.err == nil && err != nil {
		c.draftNotice("not saved; local text retained. /draft to inspect: " + err.Error())
	}
	d.err = err
}

func (c *chatTUI) loadDurableDraft() {
	if !c.durableDrafts || c.store == nil || c.sessionID == "" {
		return
	}
	c.ensureInput()
	if c.textDrafts == nil {
		c.textDrafts = map[string]*terminalDraftState{}
	}
	d := c.textDrafts[c.sessionID]
	if d != nil && (d.frozen || d.err != nil) {
		c.applyDraftSnapshot(d.local)
		return
	}
	ctx, cancel := c.draftContext()
	defer cancel()
	pair, err := c.store.LoadTUIComposerDraft(ctx, c.sessionID)
	if d == nil {
		d = &terminalDraftState{local: c.editorSnapshot()}
		c.textDrafts[c.sessionID] = d
	}
	if err != nil {
		c.setDraftError(d, err)
		return
	}
	d.pair = pair
	d.local = pair.Text.TUITextSnapshot
	c.applyDraftSnapshot(d.local)
	c.applyMediaDraft(c.sessionID, pair.Media)
	if pair.Text.Claim != nil {
		c.draftNotice("held submission recovered; /draft check. Nothing resent")
	}
}
func (c *chatTUI) applyDraftSnapshot(snapshot store.TUITextSnapshot) {
	c.draftApplying = true
	c.input.SetText(snapshot.Text)
	c.input.cursorPos = snapshot.Cursor
	c.draftApplying = false
}

func (c *chatTUI) saveDurableDraft() bool {
	if !c.durableDrafts || c.draftApplying || c.editorAskActive || c.input == nil {
		return true
	}
	d := c.textDrafts[c.sessionID]
	if d == nil {
		return false
	}
	snapshot := c.editorSnapshot()
	// Slash text is a transient command, never a replacement journal draft.
	if strings.HasPrefix(strings.TrimSpace(snapshot.Text), "/") {
		return true
	}
	d.local = snapshot
	if d.frozen {
		return true
	} // UI thread keeps newer edits; completion merges them
	if d.err != nil {
		return false
	}
	if d.local == d.pair.Text.TUITextSnapshot {
		return true
	}
	ctx, cancel := c.draftContext()
	defer cancel()
	saved, err := c.store.SaveTUITextDraft(ctx, c.sessionID, d.pair.Text.Revision, d.local)
	if err != nil {
		c.setDraftError(d, err)
		return false
	}
	d.pair.Text = saved
	return true
}

func (c *chatTUI) submitDurableDraft(text string) {
	d := c.textDrafts[c.sessionID]
	if d == nil {
		c.draftNotice("journal unavailable; text retained")
		return
	}
	if d.frozen {
		c.draftNotice("admission pending; newer draft retained")
		return
	}
	if strings.TrimSpace(c.cfg.DefaultModel) == "" {
		c.showQueueCommand(c.firstUseModelPromptLines())
		return
	}
	if d.pair.Text.Claim != nil {
		c.draftNotice("held submission; /draft check before another send")
		return
	}
	// Programmatic commands such as /attach path prompt must not smuggle their
	// replacement prompt past the exact stored-editor ownership boundary.
	if text != strings.TrimSpace(c.input.Text()) {
		c.draftNotice("put the prompt in the editor before sending; refs retained")
		return
	}
	if !c.saveDurableDraft() {
		return
	}
	scope := c.selectionScope()
	ctx, cancel := c.draftContext()
	pair, err := c.store.ClaimTUIComposerDraft(ctx, scope.id, d.pair.Text.Revision)
	cancel()
	if err != nil {
		c.setDraftError(d, err)
		return
	}
	d.pair = pair
	d.frozen = true
	d.local = pair.Text.TUITextSnapshot
	c.applyDraftSnapshot(d.local)
	c.applyMediaDraft(scope.id, pair.Media)
	c.recordInputHistory(text)
	c.queueSnapshot = nil
	wasRunning := c.running
	c.appendTranscript("you: " + text)
	model := c.cfg.DefaultModel
	token := pair.Text.Claim.Token
	// Admission is local/synchronous (like /queue and /retry), not inference.
	// Do not introduce an in-memory-only successor draft while a submit callback
	// races journal settlement. The native turn executes asynchronously.
	ctx, cancel = c.draftContext()
	result, settled, err := c.engine.SubmitTUIComposer(ctx, scope.id, token, pair.Text.Revision, model)
	cancel()
	if err == nil && result != nil {
		if wasRunning {
			c.queuedDrafts = append(c.queuedDrafts, text)
		}
		c.running = result.Status == "running" || result.Status == "queued"
	}
	c.finishDurableDraft(scope, d, pair, settled, err)
}

func (c *chatTUI) finishDurableDraft(scope sessionScope, d *terminalDraftState, claimed, settled store.TUIComposerDraft, submitErr error) {
	// The object owns the origin even after A→B→A. Never write another editor;
	// only this state object can receive the admission completion.
	if c.textDrafts[scope.id] != d {
		return
	}
	if c.sessionID == scope.id && !c.editorAskActive && !strings.HasPrefix(strings.TrimSpace(c.input.Text()), "/") {
		d.local = c.editorSnapshot()
	}
	ctx, cancel := c.draftContext()
	fresh, readErr := c.store.LoadTUIComposerDraft(ctx, scope.id)
	cancel()
	d.frozen = false
	if readErr != nil {
		d.err = readErr
		if c.sessionID == scope.id {
			c.draftNotice("admission result unreadable; draft held; /draft")
		}
		return
	}
	local := d.local
	// Match the returned revision, not merely the text: another writer could
	// clear/retype the same bytes between admission and this UI callback.
	expectedEmpty := fresh.Text.Text == "" && fresh.Text.Cursor == 0
	restored := fresh.Text.Claim == nil && fresh.Text.TUITextSnapshot == claimed.Text.Claim.TUITextSnapshot
	safe := fresh.Text.Revision == settled.Text.Revision && (expectedEmpty || restored)
	d.pair = fresh
	if local != (store.TUITextSnapshot{}) {
		if !safe || restored {
			d.err = store.ErrTUIDraftConflict
		} else {
			ctx, cancel := c.draftContext()
			saved, err := c.store.SaveTUITextDraft(ctx, scope.id, fresh.Text.Revision, local)
			cancel()
			if err != nil {
				d.err = err
			} else {
				d.pair.Text = saved
			}
		}
	} else if safe {
		d.local = fresh.Text.TUITextSnapshot
	} else {
		d.err = store.ErrTUIDraftConflict
	}
	if c.sessionID == scope.id {
		c.applyMediaDraft(scope.id, fresh.Media)
		if d.err == nil && !c.editorAskActive && !strings.HasPrefix(strings.TrimSpace(c.input.Text()), "/") {
			c.applyDraftSnapshot(d.local)
		} else if d.err != nil {
			c.draftNotice("saved draft changed during admission; local text kept; /draft")
		}
		if submitErr != nil {
			c.draftNotice("submission not confirmed: " + submitErr.Error() + "; /draft check")
			if fresh.Text.Claim == nil && len(fresh.Media.Pending) > 0 {
				c.showQueueCommand([]string{"attachments: admission rejected; references retained"})
			}
		}
		if c.app != nil {
			c.app.MarkDirty()
		}
	}
}

const draftUsage = "draft: /draft | reload | check | release|restore|discard <token>"

func (c *chatTUI) draftCommand(fields []string) []string {
	if !c.durableDrafts {
		return []string{"draft: durable editor not enabled"}
	}
	d := c.textDrafts[c.sessionID]
	if d == nil {
		return []string{"draft: unavailable"}
	}
	if len(fields) == 1 {
		lines := []string{fmt.Sprintf("draft: revision %d", d.pair.Text.Revision)}
		if d.err != nil {
			lines = append(lines, "  local unsaved/conflict; reload replaces local text")
		}
		if d.frozen {
			lines = append(lines, "  live submission pending")
		}
		if cl := d.pair.Text.Claim; cl != nil {
			kind := "unknown"
			if cl.Rejected {
				kind = "rejected"
			} else if !cl.Dispatched {
				kind = "not dispatched"
			}
			lines = append(lines, "  held: "+kind, "  token (join wrapped lines):")
			lines = append(lines, c.wrapLines(cl.Token, c.currentContentWidth())...)
		}
		return append(lines, draftUsage)
	}
	if d.frozen {
		return []string{"draft: wait for live admission; no recovery mutation"}
	}
	ctx, cancel := c.draftContext()
	defer cancel()
	switch fields[1] {
	case "reload":
		if len(fields) != 2 {
			return []string{draftUsage}
		}
		pair, err := c.store.LoadTUIComposerDraft(ctx, c.sessionID)
		if err != nil {
			return []string{"draft: reload failed: " + err.Error()}
		}
		d.pair = pair
		d.local = pair.Text.TUITextSnapshot
		d.err = nil
		c.applyDraftSnapshot(d.local)
		c.applyMediaDraft(c.sessionID, pair.Media)
		return []string{"draft: loaded stored text; nothing submitted"}
	case "check":
		if len(fields) != 2 {
			return []string{draftUsage}
		}
		if d.err != nil {
			return []string{"draft: resolve local conflict with reload before checking admission"}
		}
		if d.pair.Text.Claim == nil {
			return []string{"draft: no held submission"}
		}
		pair, err := c.store.ReconcileTUIComposerDraftAtRevision(ctx, c.sessionID, d.pair.Text.Claim.Token, d.pair.Text.Revision)
		if err != nil {
			return []string{"draft: check failed: " + err.Error()}
		}
		d.pair = pair
		c.applyMediaDraft(c.sessionID, pair.Media)
		// Editable text is unchanged by reconciliation. Do not adopt a foreign
		// editable revision (the store checks it under the same writer lock).
		if pair.Text.Claim != nil {
			return []string{"draft: admission unresolved; nothing resent"}
		}
		return []string{"draft: confirmed admission; held snapshot retired"}
	case "release", "restore", "discard":
		if len(fields) != 3 {
			return []string{draftUsage}
		}
		if d.err != nil {
			return []string{"draft: resolve local conflict before held recovery"}
		}
		var pair store.TUIComposerDraft
		var err error
		if fields[1] == "release" {
			pair, err = c.store.ReleaseUnsubmittedTUIComposerDraft(ctx, c.sessionID, fields[2], d.pair.Text.Revision)
		} else {
			pair, err = c.store.ResolveRejectedTUIComposerDraft(ctx, c.sessionID, fields[2], d.pair.Text.Revision, fields[1] == "restore")
		}
		if err != nil {
			return []string{"draft: recovery refused: " + err.Error()}
		}
		d.pair = pair
		d.local = pair.Text.TUITextSnapshot
		c.applyDraftSnapshot(d.local)
		c.applyMediaDraft(c.sessionID, pair.Media)
		return []string{"draft: " + fields[1] + " completed; nothing submitted"}
	}
	return []string{draftUsage}
}
