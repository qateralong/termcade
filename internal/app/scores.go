package app

import (
	"context"
	"time"

	"termcade/internal/games"
	"termcade/internal/store"
)

// scoreBook implements games.ScoreBook for one session. Registered players'
// results go to the database; guests only get a best score for the session.
type scoreBook struct {
	m     *App
	guest map[string]int
}

func (b *scoreBook) ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 3*time.Second)
}

func better(a, b int, lowerIsBetter bool) bool {
	if lowerIsBetter {
		return a < b
	}
	return a > b
}

func (b *scoreBook) Best(board string, lowerIsBetter bool) (int, bool) {
	p := b.m.player
	if p == nil {
		v, ok := b.guest[board]
		return v, ok
	}
	ctx, cancel := b.ctx()
	defer cancel()
	v, ok, err := b.m.deps.Store.BestScore(ctx, p.ID, board, lowerIsBetter)
	if err != nil {
		b.m.deps.Log.Error("best score", "err", err)
	}
	return v, ok
}

func (b *scoreBook) Submit(board string, lowerIsBetter bool, score int) (int, bool) {
	prev, had := b.Best(board, lowerIsBetter)
	record := !had || better(score, prev, lowerIsBetter)
	if p := b.m.player; p != nil {
		ctx, cancel := b.ctx()
		defer cancel()
		if err := b.m.deps.Store.AddScore(ctx, p.ID, board, score); err != nil {
			b.m.deps.Log.Error("save score", "err", err)
		}
	} else if record {
		if b.guest == nil {
			b.guest = make(map[string]int)
		}
		b.guest[board] = score
	}
	if record {
		return score, true
	}
	return prev, false
}

// Record implements games.ResultBook.
func (b *scoreBook) Record(r games.Result) {
	p := b.m.player
	if p == nil {
		return
	}
	ctx, cancel := b.ctx()
	defer cancel()
	err := b.m.deps.Store.AddResult(ctx, store.Result{
		PlayerID: p.ID, Game: r.Game, Multiplayer: r.Multiplayer, Place: r.Place,
		Seats: r.Seats, Score: r.Score, Won: r.Won, Duration: r.Duration,
	})
	if err != nil {
		b.m.deps.Log.Error("save result", "err", err)
	}
}

func (b *scoreBook) Top(board string, lowerIsBetter bool, n int) []games.ScoreEntry {
	ctx, cancel := b.ctx()
	defer cancel()
	rows, err := b.m.deps.Store.TopScores(ctx, board, lowerIsBetter, n)
	if err != nil {
		b.m.deps.Log.Error("top scores", "err", err)
		return nil
	}
	out := make([]games.ScoreEntry, len(rows))
	for i, r := range rows {
		out[i] = games.ScoreEntry{Name: r.Name, Color: r.Color, Score: r.Score}
	}
	return out
}
