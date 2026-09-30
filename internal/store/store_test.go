package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestPlayerLifecycle(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)

	if _, err := s.PlayerByFingerprint(ctx, "fp1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	p, err := s.CreatePlayer(ctx, "fp1", "qatera", "pink")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID == 0 || p.Name != "qatera" || p.Color != "pink" || p.Logins != 1 {
		t.Fatalf("unexpected player: %+v", p)
	}

	got, err := s.PlayerByFingerprint(ctx, "fp1")
	if err != nil || got.ID != p.ID {
		t.Fatalf("lookup: %+v, %v", got, err)
	}

	if _, err := s.CreatePlayer(ctx, "fp2", "QATERA", "cyan"); !errors.Is(err, ErrNameTaken) {
		t.Fatalf("expected case-insensitive ErrNameTaken, got %v", err)
	}

	bob, err := s.CreatePlayer(ctx, "fp2", "bob", "cyan")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateProfile(ctx, bob.ID, "Qatera", "lime"); !errors.Is(err, ErrNameTaken) {
		t.Fatalf("expected ErrNameTaken on rename, got %v", err)
	}
	if err := s.UpdateProfile(ctx, bob.ID, "robert", "lime"); err != nil {
		t.Fatal(err)
	}

	taken, err := s.NameTaken(ctx, "ROBERT", 0)
	if err != nil || !taken {
		t.Fatalf("NameTaken = %v, %v", taken, err)
	}
	taken, _ = s.NameTaken(ctx, "robert", bob.ID)
	if taken {
		t.Fatal("NameTaken should ignore the player's own name")
	}

	again, err := s.RecordLogin(ctx, bob.ID)
	if err != nil || again.Logins != 2 || again.Name != "robert" {
		t.Fatalf("RecordLogin = %+v, %v", again, err)
	}

	if n, _ := s.CountPlayers(ctx); n != 2 {
		t.Fatalf("CountPlayers = %d", n)
	}
}

func TestReopenKeepsData(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "sub", "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreatePlayer(ctx, "fp", "alice", "sky"); err != nil {
		t.Fatal(err)
	}
	s.Close()

	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p, err := s.PlayerByFingerprint(ctx, "fp")
	if err != nil || p.Name != "alice" {
		t.Fatalf("after reopen: %+v, %v", p, err)
	}
}
