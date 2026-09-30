package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// A board is a named leaderboard, such as "tetris" or "minesweeper:hard".
// Boards rank either higher or lower scores as better (points versus times).

// ScoreEntry is one row of a leaderboard: a player's best score on a board.
type ScoreEntry struct {
	Name  string
	Color string
	Score int
	At    time.Time
}

// AddScore records a finished game.
func (s *Store) AddScore(ctx context.Context, playerID int64, board string, score int) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO scores (player_id, board, score, created_at) VALUES (?, ?, ?, ?)`,
		playerID, board, score, s.now().Unix())
	return err
}

func best(lowerIsBetter bool) string {
	if lowerIsBetter {
		return "MIN"
	}
	return "MAX"
}

func order(lowerIsBetter bool) string {
	if lowerIsBetter {
		return "ASC"
	}
	return "DESC"
}

// BestScore returns a player's best score on a board. ok is false if they
// have never finished a game there.
func (s *Store) BestScore(ctx context.Context, playerID int64, board string, lowerIsBetter bool) (score int, ok bool, err error) {
	var v sql.NullInt64
	err = s.db.QueryRowContext(ctx,
		`SELECT `+best(lowerIsBetter)+`(score) FROM scores WHERE player_id = ? AND board = ?`,
		playerID, board).Scan(&v)
	if err != nil || !v.Valid {
		return 0, false, err
	}
	return int(v.Int64), true, nil
}

// TopScores returns the best score of each of the top n players on a board.
// Ties go to whoever got there first.
func (s *Store) TopScores(ctx context.Context, board string, lowerIsBetter bool, n int) ([]ScoreEntry, error) {
	rows, err := s.db.QueryContext(ctx, `
		WITH ranked AS (
			SELECT player_id, score, created_at,
			       ROW_NUMBER() OVER (PARTITION BY player_id
			                          ORDER BY score `+order(lowerIsBetter)+`, created_at) AS rn
			FROM scores WHERE board = ?
		)
		SELECT p.name, p.color, r.score, r.created_at
		FROM ranked r JOIN players p ON p.id = r.player_id
		WHERE r.rn = 1
		ORDER BY r.score `+order(lowerIsBetter)+`, r.created_at
		LIMIT ?`, board, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScoreEntry
	for rows.Next() {
		var e ScoreEntry
		var at int64
		if err := rows.Scan(&e.Name, &e.Color, &e.Score, &at); err != nil {
			return nil, err
		}
		e.At = time.Unix(at, 0)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return out, nil
}
