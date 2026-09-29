package store

import (
	"context"
	"reflect"
	"testing"
)

// This characterizes Gi's per-row mode contract; Pi has a mutable queue-wide
// setting instead. No Gi web/TUI mode switch is exposed, so this is not a
// claim that changing Pi's setting while work is queued has been ported.
func TestSteeringDequeueMixedModesUsesHeadMode(t *testing.T) {
	for _, tc := range []struct {
		name    string
		modes   []string
		batches [][]string
	}{
		{"first one, then all", []string{"one-at-a-time", "all"}, [][]string{{"one"}, {"two"}}},
		{"first all, then one", []string{"all", "one-at-a-time"}, [][]string{{"one", "two"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			s, err := Open("file::memory:?cache=shared")
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			const sessionID = "mixed-steering-modes"
			if _, err := s.CreateSession(ctx, sessionID, sessionID, nil); err != nil {
				t.Fatal(err)
			}
			for i, content := range []string{"one", "two"} {
				if _, err := s.EnqueueSteering(ctx, sessionID, "", "user", content, nil, nil, tc.modes[i]); err != nil {
					t.Fatal(err)
				}
			}
			for i, expected := range tc.batches {
				msgs, err := s.DequeueSteering(ctx, sessionID)
				if err != nil {
					t.Fatal(err)
				}
				var observed []string
				for _, msg := range msgs {
					observed = append(observed, msg.Content)
				}
				if !reflect.DeepEqual(observed, expected) {
					t.Fatalf("batch %d=%q want %q", i+1, observed, expected)
				}
			}
			if remaining, err := s.SteeringQueueLength(ctx, sessionID); err != nil || remaining != 0 {
				t.Fatalf("remaining=%d: %v", remaining, err)
			}
		})
	}
}
