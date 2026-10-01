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

// Player is a registered player. They sign in automatically from any
// computer whose SSH key is linked to the account, or with their name and
// password from anywhere else.
type Player struct {
	ID            int64
	Name          string
	Color         string
	HasPassword   bool
	CreatedAt     time.Time
	LastSeen      time.Time
	Logins        int
	SecondsOnline int
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

	// Accounts: keys move to their own table so an account can have several
	// computers (or none), and players get an optional password.
	`CREATE TABLE player_keys (
		fingerprint TEXT    PRIMARY KEY,
		player_id   INTEGER NOT NULL REFERENCES players(id) ON DELETE CASCADE,
		added_at    INTEGER NOT NULL
	);
	INSERT INTO player_keys (fingerprint, player_id, added_at)
		SELECT fingerprint, id, created_at FROM players;
	CREATE TABLE players_new (
		id             INTEGER PRIMARY KEY,
		name           TEXT    NOT NULL UNIQUE COLLATE NOCASE,
		color          TEXT    NOT NULL,
		password_hash  TEXT,
		created_at     INTEGER NOT NULL,
		last_seen      INTEGER NOT NULL,
		logins         INTEGER NOT NULL DEFAULT 0,
		seconds_online INTEGER NOT NULL DEFAULT 0
	);
	INSERT INTO players_new (id, name, color, created_at, last_seen, logins)
		SELECT id, name, color, created_at, last_seen, logins FROM players;
	DROP TABLE players;
	ALTER TABLE players_new RENAME TO players;
	CREATE INDEX player_keys_player ON player_keys (player_id)`,

	// Every finished game, for profile statistics.
	`CREATE TABLE results (
		id          INTEGER PRIMARY KEY,
		player_id   INTEGER NOT NULL REFERENCES players(id) ON DELETE CASCADE,
		game        TEXT    NOT NULL,
		multiplayer INTEGER NOT NULL,
		place       INTEGER NOT NULL,
		seats       INTEGER NOT NULL,
		score       INTEGER NOT NULL,
		won         INTEGER NOT NULL,
		duration_ms INTEGER NOT NULL,
		created_at  INTEGER NOT NULL
	);
	CREATE INDEX results_player ON results (player_id, game)`,

	// Two games were renamed; keep their scores and results.
	`UPDATE scores SET board = 'blockfall' WHERE board = 'tetris';
	UPDATE scores SET board = 'muncher' WHERE board = 'pacman';
	UPDATE results SET game = 'blockfall' WHERE game = 'tetris';
	UPDATE results SET game = 'muncher' WHERE game = 'pacman'`,
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

const playerColumns = `id, name, color, password_hash IS NOT NULL, created_at, last_seen, logins, seconds_online`

func scanPlayer(row interface{ Scan(...any) error }) (*Player, error) {
	var p Player
	var created, seen int64
	err := row.Scan(&p.ID, &p.Name, &p.Color, &p.HasPassword, &created, &seen, &p.Logins, &p.SecondsOnline)
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

// PlayerByFingerprint looks up the player a key is linked to.
func (s *Store) PlayerByFingerprint(ctx context.Context, fingerprint string) (*Player, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+prefixed("p.", playerColumns)+` FROM players p
		 JOIN player_keys k ON k.player_id = p.id WHERE k.fingerprint = ?`, fingerprint)
	return scanPlayer(row)
}

// PlayerByName looks up a player by name, case-insensitively.
func (s *Store) PlayerByName(ctx context.Context, name string) (*Player, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+playerColumns+` FROM players WHERE name = ?`, name)
	return scanPlayer(row)
}

// PlayerByID looks up a player.
func (s *Store) PlayerByID(ctx context.Context, id int64) (*Player, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+playerColumns+` FROM players WHERE id = ?`, id)
	return scanPlayer(row)
}

// prefixed qualifies every column in a comma-separated list.
func prefixed(prefix, cols string) string {
	parts := strings.Split(cols, ", ")
	for i, c := range parts {
		parts[i] = prefix + c
	}
	return strings.Join(parts, ", ")
}

// NameTaken reports whether a registered player already uses name
// (case-insensitively), ignoring the player with id exceptID.
func (s *Store) NameTaken(ctx context.Context, name string, exceptID int64) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM players WHERE name = ? AND id != ?`, name, exceptID).Scan(&n)
	return n > 0, err
}

// CreatePlayer registers a new player. passwordHash may be empty for a
// key-only account, and fingerprint empty when there's no key to link. It
// returns ErrNameTaken if the name is in use.
func (s *Store) CreatePlayer(ctx context.Context, name, color, passwordHash, fingerprint string) (*Player, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	now := s.now().Unix()
	var hash any
	if passwordHash != "" {
		hash = passwordHash
	}
	row := tx.QueryRowContext(ctx,
		`INSERT INTO players (name, color, password_hash, created_at, last_seen, logins)
		 VALUES (?, ?, ?, ?, ?, 0)
		 RETURNING `+playerColumns,
		name, color, hash, now, now)
	p, err := scanPlayer(row)
	if isUniqueViolation(err, "players.name") {
		return nil, ErrNameTaken
	}
	if err != nil {
		return nil, err
	}
	if fingerprint != "" {
		if err := linkKey(ctx, tx, p.ID, fingerprint, now); err != nil {
			return nil, err
		}
	}
	return p, tx.Commit()
}

type execer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func linkKey(ctx context.Context, db execer, playerID int64, fingerprint string, now int64) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO player_keys (fingerprint, player_id, added_at) VALUES (?, ?, ?)
		 ON CONFLICT (fingerprint) DO UPDATE SET player_id = excluded.player_id, added_at = excluded.added_at`,
		fingerprint, playerID, now)
	return err
}

// LinkKey links a key to a player so that computer signs in automatically.
// A key can only belong to one account; linking moves it.
func (s *Store) LinkKey(ctx context.Context, playerID int64, fingerprint string) error {
	return linkKey(ctx, s.db, playerID, fingerprint, s.now().Unix())
}

// UnlinkKey forgets one of a player's keys.
func (s *Store) UnlinkKey(ctx context.Context, playerID int64, fingerprint string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM player_keys WHERE player_id = ? AND fingerprint = ?`, playerID, fingerprint)
	return err
}

// Key is an SSH key linked to an account.
type Key struct {
	Fingerprint string
	AddedAt     time.Time
}

// Keys lists a player's linked keys, oldest first.
func (s *Store) Keys(ctx context.Context, playerID int64) ([]Key, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT fingerprint, added_at FROM player_keys WHERE player_id = ? ORDER BY added_at, fingerprint`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Key
	for rows.Next() {
		var k Key
		var at int64
		if err := rows.Scan(&k.Fingerprint, &at); err != nil {
			return nil, err
		}
		k.AddedAt = time.Unix(at, 0)
		out = append(out, k)
	}
	return out, rows.Err()
}

// PasswordHash returns a player's password hash, or "" if they have none.
func (s *Store) PasswordHash(ctx context.Context, id int64) (string, error) {
	var h sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT password_hash FROM players WHERE id = ?`, id).Scan(&h)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return h.String, err
}

// SetPasswordHash stores a new password hash for a player.
func (s *Store) SetPasswordHash(ctx context.Context, id int64, hash string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE players SET password_hash = ? WHERE id = ?`, hash, id)
	return err
}

// AddOnlineTime adds to a player's total time in the arcade.
func (s *Store) AddOnlineTime(ctx context.Context, id int64, d time.Duration) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE players SET seconds_online = seconds_online + ? WHERE id = ?`, int(d/time.Second), id)
	return err
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
