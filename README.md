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
- **Accounts that follow you.** Register with a name and password, and log in
  from any computer. Computers you trust remember you by their SSH key, so
  you only type the password once per machine. Or just play as a guest.
- **Profiles and stats.** Games played, multiplayer wins, personal bests, time
  in the arcade, and achievements to unlock.
- **Live lobby.** See who's online and which game they're in, and chat while
  you wait.
- **Built for more games.** Each game plugs in through a small interface; the
  platform handles identity, presence, chat and navigation.

## Games

**Solo**, with personal bests and a leaderboard for each game:

| Game | What it is |
| --- | --- |
| Pac-Man | The maze chase, with four ghosts that each hunt differently, power pellets, tunnels, fruit and levels |
| Tetris | Modern rules: SRS rotation with wall kicks, 7-bag, hold, ghost piece, next queue, lock delay |
| Snake | One snake, one apple, a walled arena, and it gets faster as you grow |
| Minesweeper | Easy, medium and hard boards, a safe first click, chording, and mouse support |

**Multiplayer**, with bots in any empty seats. A room starts as soon as it's
full, or after 20 seconds with bots filling the gaps. A game in progress never
takes new players: they get a fresh room. If someone leaves mid-match, a bot
takes over their seat. In turn-based games every move has a timer, and a bot
moves for you if it runs out.

| Game | Players | What it is |
| --- | --- | --- |
| Snake Arena | 6 | Everyone in one pit for two minutes. Dots score, and so does getting others to crash into you |
| Tanks | 4 | Battle City–style battles on three maps with brick, steel, water and bushes. Three lives, last tank wins |
| Chicken Run | 4 | Flip gravity to dodge obstacles and holes across five levels. The screen follows the leader |
| Racing | 4 | Three laps on one of three tracks. Grass is slow |
| Alien | 6 | Saucers orbit an alien; space reverses. Dodge beams, sweeps and orbs. Last one flying wins |
| Battleship | 2 | Russian rules: ships never touch, and a hit earns another shot |
| Durak | 2 | Podkidnoy durak with a 36-card deck: attack, defend, pile on, take |
| Poker | 4 | No-limit Texas hold'em with climbing blinds, side pots and Monte Carlo bots |

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

Termcade updates itself from GitHub releases. Getting the newest version onto
the server is a single command.

### 1. Install once

On a Linux server with systemd (x86_64 or arm64), run the following. The token
is a GitHub [fine-grained token](https://github.com/settings/personal-access-tokens/new)
limited to this repository with read-only **Contents** access. It is only
needed because the repository is private.

```bash
read -rsp "GitHub token: " GITHUB_TOKEN && export GITHUB_TOKEN && echo
curl -fsSL -H "Authorization: Bearer $GITHUB_TOKEN" \
     -H "Accept: application/vnd.github.raw" \
     https://api.github.com/repos/qateralong/termcade/contents/deploy/install.sh \
  | sudo -E bash
```

The script downloads the newest release, verifies its checksum and runs
`termcade install`, which:

- puts the binary in `/usr/local/bin/termcade`,
- writes the settings to `/etc/termcade/termcade.env` and the token to
  `/etc/termcade/github-token` (readable by root only),
- installs and starts a hardened systemd service that runs as an unprivileged
  user and keeps its data in `/var/lib/termcade`.

### 2. Update with one command

```bash
sudo termcade update
```

It checks for a newer release, downloads it, verifies the SHA-256 checksum,
swaps the binary, updates the service definition and restarts the service.
Players who are online see a short "the arcade is restarting" notice rather than
a dropped connection. If the new version doesn't come up healthy, the previous
one is restored and restarted automatically.

| Command | What it does |
| --- | --- |
| `termcade update --check` | Only report whether an update exists |
| `sudo termcade update --to 0.0.2` | Install a specific version, including downgrades |
| `sudo termcade update --force` | Reinstall the current version |
| `termcade version` | Print the installed version |
| `journalctl -u termcade -f` | Follow the server logs |

### Publishing a release

From your machine, on `main`:

```bash
scripts/release.sh 0.0.3
```

This runs gofmt, vet and the tests, commits everything with the version as the
message, tags `v0.0.3` and pushes. GitHub Actions then builds the Linux
binaries and publishes the release, which `termcade update` picks up.

### Serving on port 22

To let players connect with a plain `ssh arcade.example.com`, move the server's
own OpenSSH daemon to another port (for example `Port 2200` in
`/etc/ssh/sshd_config`) and make sure you can still log in on the new port.
Then set `TERMCADE_ADDR=:22` in `/etc/termcade/termcade.env` and run
`sudo systemctl restart termcade`. The service is already allowed to bind low
ports.

### Docker

```bash
docker build --build-arg VERSION=0.0.2 -t termcade .
docker run -d --name termcade --restart unless-stopped \
  -p 2222:2222 -v termcade-data:/data termcade
```

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
- Passwords are stored as bcrypt hashes. Wrong guesses lock an account for a
  minute after five tries, and a failed login takes the same time whether or
  not the name exists.

```
cmd/termcade        entry point: serve, update, install, version
deploy              systemd unit, server layout, install script
internal/update     GitHub release lookup, checksum check, binary swap
internal/version    build version and semver comparison
internal/server     SSH server, middleware, session wiring
internal/app        per-session UI: intro, setup, lobby, game host
internal/hub        presence and lobby chat
internal/games      the Game interface and upcoming games
internal/games/solo shared frame for single-player games: title, pause, scores
internal/games/multi rooms, matchmaking and bots for multiplayer games
internal/games/cards playing cards and their rendering
internal/games/*    one package per game; lineup/ sets the lobby order
internal/store      SQLite persistence
internal/auth       password hashing and login rate limiting
internal/textutil   name validation and text sanitizing
internal/ui/theme   palette, gradients, logo
internal/ui/canvas  half-block pixel graphics for the arcade games
internal/ui/layout  overlays, padding and other text layout helpers
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
