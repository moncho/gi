package store

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
)

func TestQueueSteerLatestClaimPersistsAndFencesEndedReplacement(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "last-run.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	check := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err = s.CreateSession(ctx, "a", "a", nil)
	check(err)
	for _, id := range []string{"old", "new"} {
		_, err = s.CreateTurnWithStatus(ctx, id, "a", "running", id, nil)
		check(err)
		ok, err := s.ClaimSessionActiveTurn(ctx, "a", id, "worker", id)
		check(err)
		if !ok {
			t.Fatal("claim failed", id)
		}
		check(s.UpdateTurnStatusAndPhase(ctx, id, "completed", "completed"))
		check(s.ReleaseSessionActiveTurn(ctx, "a", id))
	}
	_, err = s.CreateTurnWithStatus(ctx, "q", "a", "queued", "selected", nil)
	check(err)
	check(s.Close())
	s, err = Open(path)
	check(err)
	defer s.Close()
	if ok, err := s.ClaimEndedQueueSteer(ctx, "a", "q", "worker", "q", "old"); err != nil || ok {
		t.Fatal("older ended run accepted", ok, err)
	}
	var wg sync.WaitGroup
	results := make(chan bool, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := s.ClaimEndedQueueSteer(ctx, "a", "q", "worker", "q", "new")
			results <- ok
			errs <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	n := 0
	for ok := range results {
		if ok {
			n++
		}
	}
	for err := range errs {
		check(err)
	}
	if n != 1 {
		t.Fatal("accepted claims", n)
	}
	check(s.ReleaseFailedEndedSteer(ctx, "a", "q", "new"))
	var latest string
	check(s.DB().QueryRow(`select turn_id from session_last_run where session_id='a'`).Scan(&latest))
	if latest != "new" {
		t.Fatal(latest)
	}
}

func TestQueueSteerMarkerUpgradeAndRollbackOwnership(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "upgrade.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	check := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err = s.CreateSession(ctx, "a", "a", nil)
	check(err)
	_, err = s.CreateTurnWithStatus(ctx, "run", "a", "running", "run", nil)
	check(err)
	ok, err := s.ClaimSessionActiveTurn(ctx, "a", "run", "worker", "run")
	check(err)
	if !ok {
		t.Fatal("claim")
	}
	// Model an older DB with an active claim and no latest-run table/triggers.
	for _, sql := range []string{`drop trigger session_last_run_insert`, `drop trigger session_last_run_update`, `drop table session_last_run`} {
		_, err = s.DB().Exec(sql)
		check(err)
	}
	check(s.Close())
	s, err = Open(path)
	check(err)
	defer s.Close()
	var latest string
	check(s.DB().QueryRow(`select turn_id from session_last_run where session_id='a'`).Scan(&latest))
	if latest != "run" {
		t.Fatal(latest)
	}
	// A delayed rollback from an unrelated claim must not rewrite the marker.
	check(s.ReleaseFailedEndedSteer(ctx, "a", "not-owned", "stale"))
	check(s.DB().QueryRow(`select turn_id from session_last_run where session_id='a'`).Scan(&latest))
	if latest != "run" {
		t.Fatal(latest)
	}
	check(s.ReleaseSessionActiveTurn(ctx, "a", "run"))
	check(s.Close())
	s, err = Open(path)
	check(err)
	check(s.DB().QueryRow(`select turn_id from session_last_run where session_id='a'`).Scan(&latest))
	if latest != "run" {
		t.Fatal(latest)
	}
	// Unknown pre-upgrade idle history must stay ineligible.
	_, err = s.DB().Exec(`delete from session_last_run`)
	check(err)
	check(s.Close())
	s, err = Open(path)
	check(err)
	defer s.Close()
	var count int
	check(s.DB().QueryRow(`select count(*) from session_last_run`).Scan(&count))
	if count != 0 {
		t.Fatal("guessed history", count)
	}
}
