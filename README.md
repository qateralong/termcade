<div align="center">

<pre>
████████╗███████╗██████╗ ███╗   ███╗ ██████╗ █████╗ ██████╗ ███████╗
╚══██╔══╝██╔════╝██╔══██╗████╗ ████║██╔════╝██╔══██╗██╔══██╗██╔════╝
   ██║   █████╗  ██████╔╝██╔████╔██║██║     ███████║██║  ██║█████╗
   ██║   ██╔══╝  ██╔══██╗██║╚██╔╝██║██║     ██╔══██║██║  ██║██╔══╝
   ██║   ███████╗██║  ██║██║ ╚═╝ ██║╚██████╗██║  ██║██████╔╝███████╗
   ╚═╝   ╚══════╝╚═╝  ╚═╝╚═╝     ╚═╝ ╚═════╝╚═╝  ╚═╝╚═════╝ ╚══════╝
</pre>

**A multiplayer arcade you play over SSH.** No client to install: just a terminal.

</div>

```bash
ssh -p 2222 arcade.example.com
```

A shared lobby with live chat, accounts that follow you between computers,
profiles with stats and achievements, and 19 games:

- **Solo:** Maze Muncher, Blockfall, Snake, Minesweeper, 2048, Sokoban, Breakout,
  Space Invaders, Boss Rush.
- **Multiplayer:** Snake Arena, Tanks, Chicken Run, Racing, Tron, Bomberman,
  Alien, Battleship, Durak, Poker.

Multiplayer rooms start when they're full, when everyone is ready, or after
30 seconds, with bots in the empty seats.

## Run it

```bash
go run ./cmd/termcade      # needs Go 1.27+
ssh -p 2222 localhost      # in another terminal, at least 80×24
```

Flags: `-addr` (`:2222`), `-host-key`, `-db`, `-debug`, each also settable via
`TERMCADE_*` environment variables.

## Deploy

On a Linux server with systemd:

```bash
curl -fsSL https://raw.githubusercontent.com/qateralong/termcade/main/deploy/install.sh | sudo bash
```

This installs the newest release as a systemd service. To update later:

```bash
sudo termcade update
```

Updates verify checksums, restart without dropping players abruptly, and
roll back on their own if the new version fails to start.

## Built with

Go, [wish](https://github.com/charmbracelet/wish),
[Bubble Tea](https://github.com/charmbracelet/bubbletea),
[Lip Gloss](https://github.com/charmbracelet/lipgloss) and SQLite. To add a
game, see [docs/adding-a-game.md](docs/adding-a-game.md).

## License

[MIT](LICENSE)
