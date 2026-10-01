package store

import (
	"context"
	"testing"
	"time"
)

func TestScores(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	clock := time.Unix(1_000_000, 0)
	s.now = func() time.Time { clock = clock.Add(time.Second); return clock }

	alice, _ := s.CreatePlayer(ctx, "alice", "pink", "", "fa")
	bob, _ := s.CreatePlayer(ctx, "bob", "cyan", "", "fb")
	carol, _ := s.CreatePlayer(ctx, "carol", "lime", "", "fc")

	if _, ok, err := s.BestScore(ctx, alice.ID, "tetris", false); ok || err != nil {
		t.Fatalf("empty best: ok=%v err=%v", ok, err)
	}

	for _, r := range []struct {
		id    int64
		score int
	}{{alice.ID, 100}, {alice.ID, 900}, {bob.ID, 500}, {carol.ID, 900}, {bob.ID, 50}} {
		if err := s.AddScore(ctx, r.id, "tetris", r.score); err != nil {
			t.Fatal(err)
		}
	}
	s.AddScore(ctx, bob.ID, "snake", 99999) // other boards don't leak in

	if best, ok, _ := s.BestScore(ctx, bob.ID, "tetris", false); !ok || best != 500 {
		t.Fatalf("bob best = %d, %v", best, ok)
	}
	top, err := s.TopScores(ctx, "tetris", false, 10)
	if err != nil {
		t.Fatal(err)
	}
	// One row per player; alice reached 900 before carol did.
	want := []string{"alice:900", "carol:900", "bob:500"}
	if len(top) != len(want) {
		t.Fatalf("top = %+v", top)
	}
	for i, e := range top {
		if got := e.Name + ":" + itoa(e.Score); got != want[i] {
			t.Fatalf("top[%d] = %s, want %s", i, got, want[i])
		}
	}

	// Times: lower is better.
	s.AddScore(ctx, alice.ID, "mines:easy", 42)
	s.AddScore(ctx, alice.ID, "mines:easy", 30)
	s.AddScore(ctx, bob.ID, "mines:easy", 35)
	if best, _, _ := s.BestScore(ctx, alice.ID, "mines:easy", true); best != 30 {
		t.Fatalf("alice best time = %d", best)
	}
	top, _ = s.TopScores(ctx, "mines:easy", true, 1)
	if len(top) != 1 || top[0].Name != "alice" || top[0].Score != 30 {
		t.Fatalf("fastest = %+v", top)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}
