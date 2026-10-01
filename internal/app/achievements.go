package app

import (
	"time"

	"termcade/internal/store"
)

// summary condenses a player's record for checking achievements.
type summary struct {
	plays     int
	distinct  int
	available int
	wins      int
	bests     map[string]store.Bests
	online    time.Duration
}

func (m *App) summary() summary {
	s := summary{bests: m.profile.bests}
	for _, g := range m.profile.stats {
		s.plays += g.Plays
		s.wins += g.Wins
		s.distinct++
	}
	for _, g := range m.deps.Games.All() {
		if g.Available() {
			s.available++
		}
	}
	if m.player != nil {
		s.online = time.Duration(m.player.SecondsOnline) * time.Second
		if !m.onlineSince.IsZero() {
			s.online += time.Since(m.onlineSince)
		}
	}
	return s
}

func (s summary) best(board string) int {
	if b, ok := s.bests[board]; ok {
		return b.Max
	}
	return 0
}

type achievement struct {
	name, desc string
	done       func(summary) bool
}

var achievements = []achievement{
	{"First Steps", "play any game", func(s summary) bool { return s.plays >= 1 }},
	{"Regular", "play 25 games", func(s summary) bool { return s.plays >= 25 }},
	{"Arcade Rat", "play 100 games", func(s summary) bool { return s.plays >= 100 }},
	{"Explorer", "try 6 different games", func(s summary) bool { return s.distinct >= 6 }},
	{"Completionist", "play every game", func(s summary) bool { return s.available > 0 && s.distinct >= s.available }},
	{"First Victory", "win a multiplayer match", func(s summary) bool { return s.wins >= 1 }},
	{"Champion", "win 10 multiplayer matches", func(s summary) bool { return s.wins >= 10 }},
	{"Block Master", "10,000 points in Tetris", func(s summary) bool { return s.best("tetris") >= 10000 }},
	{"Ghost Hunter", "5,000 points in Pac-Man", func(s summary) bool { return s.best("pacman") >= 5000 }},
	{"Snake Charmer", "500 points in Snake", func(s summary) bool { return s.best("snake-classic") >= 500 }},
	{"Bomb Squad", "clear Minesweeper on Hard", func(s summary) bool {
		_, ok := s.bests["minesweeper:hard"]
		return ok
	}},
	{"Night Shift", "spend 2 hours in the arcade", func(s summary) bool { return s.online >= 2*time.Hour }},
}
