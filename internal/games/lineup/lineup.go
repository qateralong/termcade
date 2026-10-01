// Package lineup assembles the games shown in the lobby, in display order.
package lineup

import (
	"termcade/internal/games"
	"termcade/internal/games/alien"
	"termcade/internal/games/battleship"
	"termcade/internal/games/bomber"
	"termcade/internal/games/bossrush"
	"termcade/internal/games/breakout"
	"termcade/internal/games/chickenrun"
	"termcade/internal/games/durak"
	"termcade/internal/games/g2048"
	"termcade/internal/games/invaders"
	"termcade/internal/games/minesweeper"
	"termcade/internal/games/pacman"
	"termcade/internal/games/poker"
	"termcade/internal/games/racing"
	"termcade/internal/games/snake"
	"termcade/internal/games/snakearena"
	"termcade/internal/games/sokoban"
	"termcade/internal/games/tanks"
	"termcade/internal/games/tetris"
	"termcade/internal/games/tron"
)

// All returns every game: solo games first, then multiplayer.
func All() []games.Game {
	solo := []games.Game{
		pacman.Game(),
		tetris.Game(),
		snake.Game(),
		minesweeper.Game(),
		g2048.Game(),
		sokoban.Game(),
		breakout.Game(),
		invaders.Game(),
		bossrush.Game(),
	}
	multiplayer := []games.Game{
		snakearena.Game(),
		tanks.Game(),
		chickenrun.Game(),
		racing.Game(),
		tron.Game(),
		bomber.Game(),
		alien.Game(),
		battleship.Game(),
		durak.Game(),
		poker.Game(),
	}
	return append(solo, multiplayer...)
}
