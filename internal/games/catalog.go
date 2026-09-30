package games

import (
	tea "github.com/charmbracelet/bubbletea"

	"termcade/internal/ui/theme"
)

// Placeholder is a game that is announced in the lobby but not playable yet.
type Placeholder struct{ info Info }

// Info implements Game.
func (p Placeholder) Info() Info { return p.info }

// Available implements Game.
func (Placeholder) Available() bool { return false }

// Join implements Game. It is never called for unavailable games.
func (Placeholder) Join(Session) tea.Model { return nil }

// Catalog returns the default set of games shown in the lobby.
func Catalog() []Game {
	return []Game{
		Placeholder{Info{
			ID:          "tron",
			Name:        "Tron",
			Icon:        "▶",
			Tagline:     "Light cycles. Walls everywhere. Last one riding wins.",
			Description: "Every rider leaves a solid wall of light behind them. Cut others off, squeeze through gaps and outlast everyone in fast 30-second rounds.",
			Kind:        Realtime,
			Players:     "2–8",
			Controls:    []string{"←↑→↓", "steer", "space", "boost"},
			Accent:      []string{theme.Cyan, theme.Sky, theme.Violet},
			Art: []string{
				"━━━━━━━━━━━━━━━━━━┓",
				"                  ┃    ┏━━━━━━━━━━▶",
				"   ◀━━━━━━━━━━━┓  ┗━━━━┛",
				"               ┃",
				"   ━━━━━━━━━━━━┻━━━━━━━━━━━━━━━━╳",
			},
		}},
		Placeholder{Info{
			ID:          "snake",
			Name:        "Snake Arena",
			Icon:        "●",
			Tagline:     "One huge arena. Eat, grow, don't hit anyone.",
			Description: "A shared world full of snakes. Grow by eating, make others crash into you, and turn their remains into your next meal. Climb the live leaderboard.",
			Kind:        Realtime,
			Players:     "1–30",
			Controls:    []string{"←↑→↓", "turn", "space", "dash"},
			Accent:      []string{theme.Lime, theme.Mint, theme.Cyan},
			Art: []string{
				"    ·          ◆             ·",
				"  ●━━━━━━━━┓          ┏━━━━━━━●",
				"     ·     ┗━━━━━┓    ┃    ·",
				"   ◆             ┗━━━━┛        ◆",
				"         ·              ·",
			},
		}},
		Placeholder{Info{
			ID:          "typerace",
			Name:        "Type Race",
			Icon:        "▮",
			Tagline:     "Same text for everyone. Fastest fingers win.",
			Description: "Everyone gets the same passage. Type it as fast and as cleanly as you can while watching the others' progress bars creep forward.",
			Kind:        Realtime,
			Players:     "2–10",
			Controls:    []string{"type", "race", "esc", "leave"},
			Accent:      []string{theme.Amber, theme.Coral, theme.Pink},
			Art: []string{
				" alice  ██████████████░░░░  94 wpm",
				" you    ███████████░░░░░░░  82 wpm",
				" bob    ████████░░░░░░░░░░  61 wpm",
				"",
				" the quick brown fox jumps▏",
			},
		}},
		Placeholder{Info{
			ID:          "bomber",
			Name:        "Bomberman",
			Icon:        "◆",
			Tagline:     "Drop bombs, blast walls, trap your friends.",
			Description: "A classic maze brawl. Blow up crates to find power-ups, chain explosions together and corner your opponents until only one is left standing.",
			Kind:        Realtime,
			Players:     "2–4",
			Controls:    []string{"←↑→↓", "move", "space", "bomb"},
			Accent:      []string{theme.Coral, theme.Amber, theme.Pink},
			Art: []string{
				" ███████████████████████████",
				" █ ◉    ▒▒   ▒▒         ◉  █",
				" █ █ █ █ █ █▒█ █ █ █ █ █ █ █",
				" █   ▒▒    ━━━╋━━━   ▒▒    █",
				" ███████████████████████████",
			},
		}},
		Placeholder{Info{
			ID:          "battleship",
			Name:        "Battleship",
			Icon:        "■",
			Tagline:     "Hide your fleet. Find theirs.",
			Description: "The timeless duel. Place your ships, then take turns calling shots across the grid until one fleet is sunk.",
			Kind:        TurnBased,
			Players:     "2",
			Controls:    []string{"←↑→↓", "aim", "enter", "fire"},
			Accent:      []string{theme.Sky, theme.Cyan, theme.Mint},
			Art: []string{
				"    A B C D E F G H",
				"  1 · · ■ ■ ■ · · ·",
				"  2 · ╳ · · · · ○ ·",
				"  3 · · · ○ · · · ·",
				"  4 ■ · · · · ╳ ╳ ·",
			},
		}},
		Placeholder{Info{
			ID:          "durak",
			Name:        "Durak",
			Icon:        "♠",
			Tagline:     "The card game where nobody wants to be the fool.",
			Description: "Attack, defend and pile on. Get rid of all your cards before everyone else, or be crowned the durak. Bots fill empty seats.",
			Kind:        TurnBased,
			Players:     "2–6",
			Controls:    []string{"←→", "pick card", "enter", "play", "t", "take"},
			Accent:      []string{theme.Pink, theme.Coral, theme.Violet},
			Art: []string{
				" ╭─────╮╭─────╮╭─────╮",
				" │ 7   ││ K   ││ A   │",
				" │  ♠  ││  ♥  ││  ♦  │",
				" │   7 ││   K ││   A │",
				" ╰─────╯╰─────╯╰─────╯",
			},
		}},
	}
}
