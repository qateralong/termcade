package store

import (
	"context"
	"time"
)

// Result is one finished game for one player.
type Result struct {
	PlayerID    int64
	Game        string
	Multiplayer bool
	Place       int // 1-based finishing place in multiplayer, 0 in solo games
	Seats       int
	Score       int
	Won         bool
	Duration    time.Duration
}

// AddResult records a finished game.
func (s *Store) AddResult(ctx context.Context, r Result) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO results (player_id, game, multiplayer, place, seats, score, won, duration_ms, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.PlayerID, r.Game, r.Multiplayer, r.Place, r.Seats, r.Score, r.Won,
		int64(r.Duration/time.Millisecond), s.now().Unix())
	return err
}

// GameStats sums up a player's results in one game.
type GameStats struct {
	Game       string
	Plays      int
	Wins       int
	Podiums    int // top-three finishes in multiplayer
	Played     time.Duration
	LastPlayed time.Time
}

// GameStats returns a player's results per game, most played first.
func (s *Store) GameStats(ctx context.Context, playerID int64) ([]GameStats, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT game, COUNT(*), SUM(won), SUM(multiplayer AND place BETWEEN 1 AND 3),
		       SUM(duration_ms), MAX(created_at)
		FROM results WHERE player_id = ?
		GROUP BY game ORDER BY COUNT(*) DESC, MAX(created_at) DESC`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GameStats
	for rows.Next() {
		var g GameStats
		var ms, last int64
		if err := rows.Scan(&g.Game, &g.Plays, &g.Wins, &g.Podiums, &ms, &last); err != nil {
			return nil, err
		}
		g.Played = time.Duration(ms) * time.Millisecond
		g.LastPlayed = time.Unix(last, 0)
		out = append(out, g)
	}
	return out, rows.Err()
}

// Bests holds a player's best scores on a leaderboard both ways, since some
// boards rank times (lower is better) and others points.
type Bests struct{ Max, Min int }

// BestScores returns a player's best scores on every board they've played.
func (s *Store) BestScores(ctx context.Context, playerID int64) (map[string]Bests, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT board, MAX(score), MIN(score) FROM scores WHERE player_id = ? GROUP BY board`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]Bests{}
	for rows.Next() {
		var board string
		var b Bests
		if err := rows.Scan(&board, &b.Max, &b.Min); err != nil {
			return nil, err
		}
		out[board] = b
	}
	return out, rows.Err()
}
