package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"
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

	p, err := s.CreatePlayer(ctx, "qatera", "pink", "", "fp1")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID == 0 || p.Name != "qatera" || p.Color != "pink" || p.Logins != 0 || p.HasPassword {
		t.Fatalf("unexpected player: %+v", p)
	}

	got, err := s.PlayerByFingerprint(ctx, "fp1")
	if err != nil || got.ID != p.ID {
		t.Fatalf("lookup by key: %+v, %v", got, err)
	}
	if got, err := s.PlayerByName(ctx, "QATERA"); err != nil || got.ID != p.ID {
		t.Fatalf("lookup by name: %+v, %v", got, err)
	}

	if _, err := s.CreatePlayer(ctx, "QATERA", "cyan", "", "fp2"); !errors.Is(err, ErrNameTaken) {
		t.Fatalf("expected case-insensitive ErrNameTaken, got %v", err)
	}
	// The failed registration must not have linked its key.
	if _, err := s.PlayerByFingerprint(ctx, "fp2"); !errors.Is(err, ErrNotFound) {
		t.Fatal("key from a failed registration was linked")
	}

	bob, err := s.CreatePlayer(ctx, "bob", "cyan", "hash", "")
	if err != nil {
		t.Fatal(err)
	}
	if !bob.HasPassword {
		t.Fatal("bob should have a password")
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
	if taken, _ = s.NameTaken(ctx, "robert", bob.ID); taken {
		t.Fatal("NameTaken should ignore the player's own name")
	}

	again, err := s.RecordLogin(ctx, bob.ID)
	if err != nil || again.Logins != 1 || again.Name != "robert" {
		t.Fatalf("RecordLogin = %+v, %v", again, err)
	}
	if n, _ := s.CountPlayers(ctx); n != 2 {
		t.Fatalf("CountPlayers = %d", n)
	}
}

func TestKeysAndPasswords(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	a, _ := s.CreatePlayer(ctx, "alice", "pink", "", "laptop")
	b, _ := s.CreatePlayer(ctx, "bob", "cyan", "", "")

	if err := s.LinkKey(ctx, a.ID, "desktop"); err != nil {
		t.Fatal(err)
	}
	keys, _ := s.Keys(ctx, a.ID)
	if len(keys) != 2 {
		t.Fatalf("alice has %d keys", len(keys))
	}

	// Linking a key that belongs to someone else moves it.
	if err := s.LinkKey(ctx, b.ID, "laptop"); err != nil {
		t.Fatal(err)
	}
	if p, _ := s.PlayerByFingerprint(ctx, "laptop"); p.ID != b.ID {
		t.Fatal("key didn't move to bob")
	}
	if err := s.UnlinkKey(ctx, a.ID, "desktop"); err != nil {
		t.Fatal(err)
	}
	if keys, _ := s.Keys(ctx, a.ID); len(keys) != 0 {
		t.Fatalf("alice still has %d keys", len(keys))
	}

	if h, err := s.PasswordHash(ctx, a.ID); err != nil || h != "" {
		t.Fatalf("new account hash = %q, %v", h, err)
	}
	s.SetPasswordHash(ctx, a.ID, "$2a$hash")
	if h, _ := s.PasswordHash(ctx, a.ID); h != "$2a$hash" {
		t.Fatalf("hash = %q", h)
	}
	if p, _ := s.PlayerByID(ctx, a.ID); !p.HasPassword {
		t.Fatal("HasPassword should be set")
	}

	s.AddOnlineTime(ctx, a.ID, 90*time.Second)
	if p, _ := s.PlayerByID(ctx, a.ID); p.SecondsOnline != 90 {
		t.Fatalf("SecondsOnline = %d", p.SecondsOnline)
	}
}

func TestResults(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	p, _ := s.CreatePlayer(ctx, "alice", "pink", "", "")
	add := func(game string, multi bool, place int, won bool) {
		t.Helper()
		if err := s.AddResult(ctx, Result{PlayerID: p.ID, Game: game, Multiplayer: multi, Place: place, Seats: 4, Won: won, Duration: time.Minute}); err != nil {
			t.Fatal(err)
		}
	}
	add("tanks", true, 1, true)
	add("tanks", true, 3, false)
	add("tanks", true, 4, false)
	add("blockfall", false, 0, false)

	stats, err := s.GameStats(ctx, p.ID)
	if err != nil || len(stats) != 2 {
		t.Fatalf("stats = %+v, %v", stats, err)
	}
	tanks := stats[0]
	if tanks.Game != "tanks" || tanks.Plays != 3 || tanks.Wins != 1 || tanks.Podiums != 2 || tanks.Played != 3*time.Minute {
		t.Fatalf("tanks stats = %+v", tanks)
	}

	s.AddScore(ctx, p.ID, "blockfall", 500)
	s.AddScore(ctx, p.ID, "blockfall", 900)
	best, _ := s.BestScores(ctx, p.ID)
	if best["blockfall"] != (Bests{Max: 900, Min: 500}) {
		t.Fatalf("best = %+v", best)
	}
}

func TestReopenKeepsData(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "sub", "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreatePlayer(ctx, "alice", "sky", "", "fp"); err != nil {
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

// TestMigratesOldDatabase builds a database the way version 0.0.6 left it
// and checks that players, their keys and scores survive the upgrade,
// including scores of games that were renamed since.
func TestMigratesOldDatabase(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range append(migrations[:2:2],
		`INSERT INTO players (fingerprint, name, color, created_at, last_seen, logins) VALUES ('SHA256:old', 'veteran', 'lime', 100, 200, 7)`,
		`INSERT INTO scores (player_id, board, score, created_at) VALUES (1, 'tetris', 4242, 150)`,
		`PRAGMA user_version = 2`,
	) {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p, err := s.PlayerByFingerprint(ctx, "SHA256:old")
	if err != nil || p.Name != "veteran" || p.Logins != 7 || p.HasPassword {
		t.Fatalf("migrated player = %+v, %v", p, err)
	}
	if best, _, _ := s.BestScore(ctx, p.ID, "blockfall", false); best != 4242 {
		t.Fatalf("migrated score = %d", best)
	}
	if keys, _ := s.Keys(ctx, p.ID); len(keys) != 1 || keys[0].AddedAt.Unix() != 100 {
		t.Fatalf("migrated keys = %+v", keys)
	}
}
