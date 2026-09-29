package store

import (
	"context"
	"testing"
)

func TestActiveClaimTokenCannotBeReusedAcrossReleasesOrReopen(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/claims.db"
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateSession(ctx, "claim-history", "claim-history", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTurnWithStatus(ctx, "turn", "claim-history", "running", "prompt", nil); err != nil {
		t.Fatal(err)
	}
	original, err := NewActiveTurnClaimToken()
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := s.ClaimSessionActiveTurn(ctx, "claim-history", "turn", "runner", original); err != nil || !ok {
		t.Fatalf("first claim=%t: %v", ok, err)
	}
	if err := s.ReleaseSessionActiveTurn(ctx, "claim-history", original); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.ClaimSessionActiveTurn(ctx, "claim-history", "turn", "foreign", original); err == nil || ok {
		t.Fatalf("reused released token: claimed=%t err=%v", ok, err)
	}
	if _, _, err := s.GetSessionActiveTurn(ctx, "claim-history"); err == nil {
		t.Fatal("failed claim left active row")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if ok, err := s.ClaimSessionActiveTurn(ctx, "claim-history", "turn", "foreign", original); err == nil || ok {
		t.Fatalf("reused token after reopen: claimed=%t err=%v", ok, err)
	}
	next, err := NewActiveTurnClaimToken()
	if err != nil {
		t.Fatal(err)
	}
	if next == original {
		t.Fatal("claim tokens repeated")
	}
	if ok, err := s.ClaimSessionActiveTurn(ctx, "claim-history", "turn", "runner", next); err != nil || !ok {
		t.Fatalf("fresh claim=%t: %v", ok, err)
	}
}

func TestActiveClaimTokenMigrationBackfillsLiveClaim(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/legacy-claims.db"
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateSession(ctx, "legacy", "legacy", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTurnWithStatus(ctx, "legacy-turn", "legacy", "running", "prompt", nil); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.ClaimSessionActiveTurn(ctx, "legacy", "legacy-turn", "runner", "legacy-token"); err != nil || !ok {
		t.Fatalf("legacy claim=%t: %v", ok, err)
	}
	// Recreate a database predating claim-token history while keeping the
	// active claim. The next Open must reserve that token before installing
	// the insert trigger.
	if _, err := s.DB().ExecContext(ctx, `drop trigger session_claim_token_insert; drop table session_claim_tokens;`); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	turnID, token, err := s.GetSessionActiveTurn(ctx, "legacy")
	if err != nil || turnID != "legacy-turn" || token != "legacy-token" {
		t.Fatalf("migrated claim=%q token=%q: %v", turnID, token, err)
	}
	if err := s.ReleaseSessionActiveTurn(ctx, "legacy", token); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.ClaimSessionActiveTurn(ctx, "legacy", "legacy-turn", "other", token); err == nil || ok {
		t.Fatalf("reused migrated token: claimed=%t err=%v", ok, err)
	}
}
