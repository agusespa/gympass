package store

import (
	"context"
	"testing"
)

func TestReadyLeavesSyncDataUnchanged(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	cursor, _, err := s.Sync(ctx, 0, map[string][]Row{"workouts": {workout("w1", 100, "Push")}})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := s.Ready(ctx); err != nil {
			t.Fatal(err)
		}
	}
	got, changes, err := s.Sync(ctx, cursor, nil)
	if err != nil || got != cursor || len(changes) != 0 {
		t.Fatalf("probe changed sync state: cursor %d changes %v err %v", got, changes, err)
	}
}

func TestReadyDetectsUnusableDatabase(t *testing.T) {
	for _, fault := range []string{"closed", "read-only", "missing metadata", "cancelled"} {
		t.Run(fault, func(t *testing.T) {
			s := openTest(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var err error
			switch fault {
			case "closed":
				err = s.Close()
			case "read-only":
				_, err = s.db.Exec("PRAGMA query_only = ON")
			case "missing metadata":
				_, err = s.db.Exec("DELETE FROM sync_state")
			case "cancelled":
				cancel()
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Ready(ctx); err == nil {
				t.Fatal("expected readiness to fail")
			}
		})
	}
}
