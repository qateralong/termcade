# Adding a game

Every cabinet in the lobby is a value that implements `games.Game`:

```go
type Game interface {
	Info() Info               // name, description, art, controls...
	Available() bool          // false shows the game as "coming soon"
	Join(s Session) tea.Model // called each time a player launches it
}
```

## Lifecycle

1. **Server start.** The game value is created once and registered in
   `cmd/termcade/main.go`. It lives until the server stops, so this is where
   shared state goes: rooms, matchmaking queues, the game world, a ticker
   goroutine.
2. **A player presses enter.** The lobby calls `Join` with the player's
   identity, the session's theme and the terminal size. Return a fresh
   Bubble Tea model for that player.
3. **Playing.** The platform forwards every message (keys, window resizes and
   anything you `Send` to the program) to your model. `ctrl+c` always
   disconnects the player.
4. **Leaving.** Return `games.Exit` as a command and the player goes back to
   the lobby. If your model implements `games.Leaver`, its `Leave` method is
   called exactly once when the player goes, whether they returned to the
   lobby or their SSH connection dropped. Use it to remove them from rooms.

## A minimal game

```go
package clicker

import (
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"termcade/internal/games"
	"termcade/internal/ui/theme"
)

type Game struct{}

func (Game) Info() games.Info {
	return games.Info{
		ID:          "clicker",
		Name:        "Clicker",
		Icon:        "●",
		Tagline:     "Press space. That's it.",
		Description: "The simplest game in the arcade.",
		Kind:        games.Realtime,
		Players:     "1",
		Controls:    []string{"space", "click", "q", "leave"},
		Accent:      []string{theme.Lime, theme.Cyan},
	}
}

func (Game) Available() bool { return true }

func (Game) Join(s games.Session) tea.Model { return &model{s: s} }

type model struct {
	s      games.Session
	clicks int
}

func (m *model) Init() tea.Cmd { return nil }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.s.Width, m.s.Height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case " ":
			m.clicks++
		case "q", "esc":
			return m, games.Exit
		}
	}
	return m, nil
}

func (m *model) View() string {
	return m.s.Theme.Title.Render("clicks: ") + m.s.Theme.Bold.Render(strconv.Itoa(m.clicks))
}
```

Then register it in `main.go`:

```go
Games: games.NewRegistry(append([]games.Game{clicker.Game{}}, games.Catalog()...)...),
```

## Multiplayer tips

- **Keep the server authoritative.** Clients only send intents (keys); the
  shared game state decides what happens. Players can't cheat by sending
  impossible moves.
- **Run the simulation in one place.** For realtime games, a single goroutine
  per room ticking at a fixed rate (10–20 Hz works well in a terminal) that
  updates the world and then broadcasts a snapshot is far simpler than
  locking around every player's input.
- **Push updates into sessions** with `Session.Send`. It blocks until the
  player's program takes the message, so give every player a small buffered
  queue drained by its own goroutine and drop updates for players who fall
  behind. `internal/hub` does exactly this and is a good template.
- **Handle disconnects** by implementing `games.Leaver` on your model and
  removing the player from their room in `Leave`.
- **Respect the terminal size.** Your model receives `tea.WindowSizeMsg` right
  after `Join`. Render a friendly "too small" message rather than a broken
  frame.
- **Sanitize anything players type** that other players will see, using
  `textutil.SanitizeLine`.
