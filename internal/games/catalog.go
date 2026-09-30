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

// lobbyNote explains the shared matchmaking rules on every multiplayer game.
const lobbyNote = " Rooms start when full, or after 20 seconds with bots in the empty seats."

// Upcoming returns the multiplayer games that are announced but not built
// yet.
func Upcoming() []Game {
	mp := func(i Info) Game {
		i.Mode = Multiplayer
		i.Description += lobbyNote
		return Placeholder{i}
	}
	return []Game{
		mp(Info{
			ID:          "snake-arena",
			Name:        "Snake Arena",
			Icon:        "●",
			Tagline:     "Classic snake, but everyone's in the same pit.",
			Description: "Collect dots to score and grow while dodging the other snakes.",
			Kind:        Realtime,
			Players:     "2–8",
			Controls:    []string{"←↑→↓", "turn"},
			Accent:      []string{theme.Lime, theme.Mint, theme.Cyan},
			Art: []string{
				"    ·          ◆             ·",
				"  ●━━━━━━━━┓          ┏━━━━━━━●",
				"     ·     ┗━━━━━┓    ┃    ·",
				"   ◆             ┗━━━━┛        ◆",
			},
		}),
		mp(Info{
			ID:          "tanks",
			Name:        "Tanks",
			Icon:        "▣",
			Tagline:     "Brick walls, steel walls and a lot of shells.",
			Description: "Top-down tank battles in the style of the NES classic. Blast through bricks, hide behind steel, and be the last tank rolling.",
			Kind:        Realtime,
			Players:     "2–4",
			Controls:    []string{"←↑→↓", "drive", "space", "fire"},
			Accent:      []string{theme.Amber, theme.Coral, theme.Lime},
			Art: []string{
				" ▓▓▓▓  ░░░░      ▓▓▓▓",
				" ▓▓▓▓  ░░░░  ▲   ▓▓▓▓",
				"       ░░░░  █",
				"   ▀█▀     ·  ·  ·  ▄█▄",
			},
		}),
		mp(Info{
			ID:          "chicken-run",
			Name:        "Chicken Run",
			Icon:        "▲",
			Tagline:     "One button flips gravity. Don't fall off the world.",
			Description: "Four chickens race through a random level. Tap to flip your gravity between floor and ceiling, dodge the obstacles, and don't get left behind the edge of the screen.",
			Kind:        Realtime,
			Players:     "4",
			Controls:    []string{"space", "flip gravity"},
			Accent:      []string{theme.Amber, "#FF9F43", theme.Pink},
			Art: []string{
				"▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀",
				"      ▼        █        ▼",
				"  ▲       ▲    █   ▲",
				"▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄",
			},
		}),
		mp(Info{
			ID:          "racing",
			Name:        "Racing",
			Icon:        "▶",
			Tagline:     "Three laps. Several tracks. First across the line.",
			Description: "Top-down car racing on arrow keys. Take the racing line, don't clip the walls, and be first after three laps.",
			Kind:        Realtime,
			Players:     "4",
			Controls:    []string{"↑", "gas", "↓", "brake", "←→", "steer"},
			Accent:      []string{theme.Coral, theme.Amber, theme.Sky},
			Art: []string{
				"╭────────────────────────╮",
				"│  ▶ ▶      ╭──────╮  ▼  │",
				"│      ▶    ╰──────╯     │",
				"╰────────────────────────╯",
			},
		}),
		mp(Info{
			ID:          "alien",
			Name:        "Alien",
			Icon:        "◉",
			Tagline:     "Orbit the alien. Dodge its attacks. Outlast everyone.",
			Description: "Every player is a flying saucer circling the alien non-stop. Press space to reverse your orbit and dodge its attacks. Last saucer flying wins.",
			Kind:        Realtime,
			Players:     "2–6",
			Controls:    []string{"space", "reverse orbit"},
			Accent:      []string{theme.Mint, theme.Lime, theme.Violet},
			Art: []string{
				"        ◇         ◇",
				"   ◇       ╭───╮      ◇",
				"      ⋯⋯⋯⋯ │◉ ◉│ ⋯⋯⋯",
				"   ◇       ╰─▽─╯      ◇",
			},
		}),
		mp(Info{
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
			},
		}),
		mp(Info{
			ID:          "durak",
			Name:        "Durak",
			Icon:        "♠",
			Tagline:     "The card game where nobody wants to be the fool.",
			Description: "Attack, defend and pile on. Get rid of all your cards first, or be crowned the durak.",
			Kind:        TurnBased,
			Players:     "2",
			Controls:    []string{"←→", "pick card", "enter", "play", "t", "take"},
			Accent:      []string{theme.Pink, theme.Coral, theme.Violet},
			Art: []string{
				" ╭─────╮╭─────╮╭─────╮",
				" │ 7   ││ K   ││ A   │",
				" │  ♠  ││  ♥  ││  ♦  │",
				" ╰─────╯╰─────╯╰─────╯",
			},
		}),
		mp(Info{
			ID:          "poker",
			Name:        "Poker",
			Icon:        "♦",
			Tagline:     "Texas hold'em for four. Chips, bluffs, showdowns.",
			Description: "No-limit Texas hold'em at a four-seat table. Read the others, pick your moment, and take the pot.",
			Kind:        TurnBased,
			Players:     "4",
			Controls:    []string{"c", "check/call", "r", "raise", "f", "fold"},
			Accent:      []string{theme.Lime, theme.Amber, theme.Coral},
			Art: []string{
				" ╭────╮╭────╮  ◎◎◎  ╭────╮",
				" │ A♠ ││ A♥ │  ◎◎   │ ?? │",
				" ╰────╯╰────╯       ╰────╯",
			},
		}),
	}
}
