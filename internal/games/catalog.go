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
