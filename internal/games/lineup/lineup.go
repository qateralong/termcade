// Package lineup assembles the games shown in the lobby, in display order.
package lineup

import (
	"termcade/internal/games"
	"termcade/internal/games/alien"
	"termcade/internal/games/battleship"
	"termcade/internal/games/chickenrun"
	"termcade/internal/games/durak"
	"termcade/internal/games/minesweeper"
	"termcade/internal/games/pacman"
	"termcade/internal/games/poker"
	"termcade/internal/games/racing"
	"termcade/internal/games/snake"
	"termcade/internal/games/snakearena"
	"termcade/internal/games/tanks"
	"termcade/internal/games/tetris"
)

// All returns every game: solo games first, then multiplayer.
func All() []games.Game {
	solo := []games.Game{
		pacman.Game(),
		tetris.Game(),
		snake.Game(),
		minesweeper.Game(),
	}
	multiplayer := []games.Game{
		snakearena.Game(),
		tanks.Game(),
		chickenrun.Game(),
		racing.Game(),
		alien.Game(),
		battleship.Game(),
		durak.Game(),
		poker.Game(),
	}
	return append(solo, multiplayer...)
}
