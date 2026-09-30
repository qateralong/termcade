<div align="center">

<pre>
████████╗███████╗██████╗ ███╗   ███╗ ██████╗ █████╗ ██████╗ ███████╗
╚══██╔══╝██╔════╝██╔══██╗████╗ ████║██╔════╝██╔══██╗██╔══██╗██╔════╝
   ██║   █████╗  ██████╔╝██╔████╔██║██║     ███████║██║  ██║█████╗
   ██║   ██╔══╝  ██╔══██╗██║╚██╔╝██║██║     ██╔══██║██║  ██║██╔══╝
   ██║   ███████╗██║  ██║██║ ╚═╝ ██║╚██████╗██║  ██║██████╔╝███████╗
   ╚═╝   ╚══════╝╚═╝  ╚═╝╚═╝     ╚═╝ ╚═════╝╚═╝  ╚═╝╚═════╝ ╚══════╝
</pre>

**A tiny multiplayer arcade you play over SSH.**

No install. No sign-up. Just open a terminal.

</div>

```bash
ssh -p 2222 arcade.example.com
```

---

## What is this?

Termcade is an SSH server that drops everyone who connects into a shared,
colorful terminal lobby. From there you pick a game cabinet and play against
whoever else is online, all rendered with plain text in the terminal you
already have.

- **Zero friction.** `ssh` ships with Windows, macOS and Linux. That's the
  whole client.
- **Your SSH key is your account.** Players are recognized by their public key
  fingerprint, so there are no passwords to remember or leak. Connecting
  without a key works too; you just play as a guest.
- **Live lobby.** See who's online and which game they're in, and chat while
  you wait.
- **Built for more games.** Each game plugs in through a small interface; the
  platform handles identity, presence, chat and navigation.

## Games

| Game | Style | Players | Status |
| --- | --- | --- | --- |
| Tron | Realtime | 2–8 | 🛠 in the workshop |
| Snake Arena | Realtime | 1–30 | 🛠 in the workshop |
| Type Race | Realtime | 2–10 | 🛠 in the workshop |
| Bomberman | Realtime | 2–4 | 🛠 in the workshop |
| Battleship | Turn-based | 2 | 🛠 in the workshop |
| Durak | Turn-based | 2–6 | 🛠 in the workshop |

## Running it locally

You need [Go 1.27+](https://go.dev/dl/).

```bash
go run ./cmd/termcade
```

Then connect from another terminal:

```bash
ssh -p 2222 localhost
```

The first start generates an SSH host key and a SQLite database in `./data`.
Your terminal needs to be at least **80×24**.

### Configuration

Every flag can also be set with an environment variable.

| Flag | Environment | Default | Description |
| --- | --- | --- | --- |
| `-addr` | `TERMCADE_ADDR` | `:2222` | Address to listen on |
| `-host-key` | `TERMCADE_HOST_KEY` | `data/host_ed25519` | SSH host key, created if missing |
| `-db` | `TERMCADE_DB` | `data/termcade.db` | SQLite database file |
| `-debug` | `TERMCADE_DEBUG` | off | Verbose logging |

> **Keep the host key.** If it changes, returning players get a scary
> "host identification has changed" warning from their SSH client.

## Deploying

### Docker

```bash
docker build -t termcade .
docker run -d --name termcade --restart unless-stopped \
  -p 2222:2222 -v termcade-data:/data termcade
```

### systemd

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o termcade ./cmd/termcade
scp termcade you@your-vps:/usr/local/bin/termcade
scp deploy/termcade.service you@your-vps:/etc/systemd/system/
ssh you@your-vps 'systemctl daemon-reload && systemctl enable --now termcade'
```

### Serving on port 22

To let players connect with a plain `ssh arcade.example.com`, move your
server's own OpenSSH daemon to another port (for example `Port 2200` in
`/etc/ssh/sshd_config`), make sure you can log in on the new port, and then run
Termcade with `-addr :22`. The bundled systemd unit already grants the
capability needed to bind low ports.

## How it works

```
 player ──ssh──┐
 player ──ssh──┼──▶ wish SSH server ──▶ one Bubble Tea program per session
 player ──ssh──┘          │                    │
                          │                    ├── intro → player setup → lobby → game
                          ▼                    │
                   SQLite (players)      hub: presence + lobby chat ◀── all sessions
```

- [`wish`](https://github.com/charmbracelet/wish) serves SSH and hands every
  session its own [Bubble Tea](https://github.com/charmbracelet/bubbletea)
  program. There is no shell: connecting only ever reaches the arcade.
- Styles are built per session with [Lip Gloss](https://github.com/charmbracelet/lipgloss),
  so each client gets colors that match its own terminal.
- The **hub** tracks who is online and where, and fans out chat messages. Each
  session has its own delivery queue, so one slow connection can't stall the
  others.
- Everything user-provided (names, chat) is sanitized before it's shown in
  anyone else's terminal: escape sequences, control characters and bidi
  overrides are stripped.

```
cmd/termcade        entry point, flags, graceful shutdown
internal/server     SSH server, middleware, session wiring
internal/app        per-session UI: intro, setup, lobby, game host
internal/hub        presence and lobby chat
internal/games      the Game interface and the game catalog
internal/store      SQLite persistence
internal/textutil   name validation and text sanitizing
internal/ui/theme   palette, gradients, logo
```

## Adding a game

A game is a Go value implementing `games.Game`. It lives for the lifetime of
the server, so shared state (rooms, matchmaking, the world) belongs on it,
while `Join` returns a Bubble Tea model for each player. See
[docs/adding-a-game.md](docs/adding-a-game.md) for a walkthrough.

## Development

```bash
go test ./...      # unit tests, layout tests and an end-to-end SSH test
go vet ./...
```

## License

[MIT](LICENSE)
