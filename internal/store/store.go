// Package store persists players in a SQLite database.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, no cgo required
)

var (
	ErrNotFound  = errors.New("not found")
	ErrNameTaken = errors.New("that name is already taken")
)

// Player is a registered player, identified by the fingerprint of their SSH
// public key.
type Player struct {
	ID          int64
	Fingerprint string
	Name        string
	Color       string
	CreatedAt   time.Time
	LastSeen    time.Time
	Logins      int
}

// Store is a handle to the database. It is safe for concurrent use.
type Store struct {
	db  *sql.DB
	now func() time.Time
}

// migrations are applied in order; the index+1 of the last applied migration
// is kept in PRAGMA user_version.
var migrations = []string{
	`CREATE TABLE players (
		id          INTEGER PRIMARY KEY,
		fingerprint TEXT    NOT NULL UNIQUE,
		name        TEXT    NOT NULL UNIQUE COLLATE NOCASE,
		color       TEXT    NOT NULL,
		created_at  INTEGER NOT NULL,
		last_seen   INTEGER NOT NULL,
		logins      INTEGER NOT NULL DEFAULT 0
	)`,
	`CREATE TABLE scores (
		id         INTEGER PRIMARY KEY,
		player_id  INTEGER NOT NULL REFERENCES players(id) ON DELETE CASCADE,
		board      TEXT    NOT NULL,
		score      INTEGER NOT NULL,
		created_at INTEGER NOT NULL
	);
	CREATE INDEX scores_board ON scores (board, score)`,
}

// Open opens (creating if needed) the database at path and brings its schema
// up to date. Use ":memory:" for a throwaway database.
func Open(path string) (*Store, error) {
	dsn := ":memory:"
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create data dir: %w", err)
		}
		dsn = "file:" + filepath.ToSlash(path) +
			"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite allows a single writer; one connection keeps things simple and
	// is plenty for this workload. It is also required for ":memory:".
	db.SetMaxOpenConns(1)

	s := &Store{db: db, now: time.Now}
	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate(ctx context.Context) error {
	var version int
	if err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	for i := version; i < len(migrations); i++ {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, migrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

const playerColumns = `id, fingerprint, name, color, created_at, last_seen, logins`

func scanPlayer(row interface{ Scan(...any) error }) (*Player, error) {
	var p Player
	var created, seen int64
	err := row.Scan(&p.ID, &p.Fingerprint, &p.Name, &p.Color, &created, &seen, &p.Logins)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	p.CreatedAt = time.Unix(created, 0)
	p.LastSeen = time.Unix(seen, 0)
	return &p, nil
}

// PlayerByFingerprint looks up the player owning the given key fingerprint.
func (s *Store) PlayerByFingerprint(ctx context.Context, fingerprint string) (*Player, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+playerColumns+` FROM players WHERE fingerprint = ?`, fingerprint)
	return scanPlayer(row)
}

// NameTaken reports whether a registered player already uses name
// (case-insensitively), ignoring the player with id exceptID.
func (s *Store) NameTaken(ctx context.Context, name string, exceptID int64) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM players WHERE name = ? AND id != ?`, name, exceptID).Scan(&n)
	return n > 0, err
}

// CreatePlayer registers a new player. It returns ErrNameTaken if the name is
// in use.
func (s *Store) CreatePlayer(ctx context.Context, fingerprint, name, color string) (*Player, error) {
	now := s.now().Unix()
	row := s.db.QueryRowContext(ctx,
		`INSERT INTO players (fingerprint, name, color, created_at, last_seen, logins)
		 VALUES (?, ?, ?, ?, ?, 1)
		 RETURNING `+playerColumns,
		fingerprint, name, color, now, now)
	p, err := scanPlayer(row)
	if isUniqueViolation(err, "players.name") {
		return nil, ErrNameTaken
	}
	return p, err
}

// UpdateProfile changes a player's name and color.
func (s *Store) UpdateProfile(ctx context.Context, id int64, name, color string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE players SET name = ?, color = ? WHERE id = ?`, name, color, id)
	if isUniqueViolation(err, "players.name") {
		return ErrNameTaken
	}
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// RecordLogin bumps the login counter and last-seen time, returning the
// updated player.
func (s *Store) RecordLogin(ctx context.Context, id int64) (*Player, error) {
	row := s.db.QueryRowContext(ctx,
		`UPDATE players SET logins = logins + 1, last_seen = ? WHERE id = ?
		 RETURNING `+playerColumns,
		s.now().Unix(), id)
	return scanPlayer(row)
}

// CountPlayers returns the number of registered players.
func (s *Store) CountPlayers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM players`).Scan(&n)
	return n, err
}

func isUniqueViolation(err error, column string) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed: "+column)
}
