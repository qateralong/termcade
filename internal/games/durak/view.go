package durak

import (
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"termcade/internal/games/cards"
	"termcade/internal/ui/theme"
)

// ViewW and ViewH are the fixed size of the play area.
const (
	ViewW = 78
	ViewH = 20
)

// View implements multi.World.
func (w *World) View(seat int, th *theme.Theme) string {
	me, opp := w.p[seat], w.p[1-seat]

	// Opponent: name, role and card backs.
	role := func(i int) string {
		if i == w.attacker {
			return th.Fg(theme.Coral).Render("attacking")
		}
		return th.Fg(theme.Sky).Render("defending")
	}
	var backs []string
	for i := 0; i < len(opp.hand) && i < 18; i++ {
		backs = append(backs, cards.Back(th))
	}
	oppLine := th.Fg(opp.info.Color).Bold(true).Render(opp.info.Name) + th.Faded.Render("  "+itoa(len(opp.hand))+" cards  ·  ") + role(1-seat)
	oppCards := strings.Join(backs, " ")

	// Deck and trump on the left, the table to the right.
	var deck string
	switch {
	case len(w.deck) > 1:
		deck = lipgloss.JoinHorizontal(lipgloss.Top, cards.BigBack(th), " ", cards.Big(th, w.trumpCard, cards.Normal))
	case len(w.deck) == 1:
		deck = lipgloss.JoinHorizontal(lipgloss.Top, cards.Blank(), " ", cards.Big(th, w.trumpCard, cards.Normal))
	default:
		deck = lipgloss.JoinHorizontal(lipgloss.Top, cards.Blank(), " ", lipgloss.Place(5, 3, lipgloss.Center, lipgloss.Center, th.Fg(theme.Amber).Bold(true).Render(w.trump.Symbol())))
	}
	deckInfo := th.Faded.Render("deck " + itoa(len(w.deck)) + "  trump " + w.trump.Symbol())
	left := lipgloss.JoinVertical(lipgloss.Left, deck, deckInfo)

	var pairs []string
	for _, pr := range w.table {
		top := cards.Big(th, pr.atk, cards.Normal)
		bottom := cards.Blank()
		if pr.beaten {
			bottom = cards.Big(th, pr.def, cards.Normal)
		}
		pairs = append(pairs, lipgloss.JoinVertical(lipgloss.Left, top, bottom))
	}
	table := th.Faded.Render("the table is empty")
	if len(pairs) > 0 {
		table = lipgloss.JoinHorizontal(lipgloss.Top, intersperse(pairs, " ")...)
	}
	divider := th.R.NewStyle().Foreground(th.Border).Render(strings.Repeat("│\n", 6) + "│")
	tableBox := lipgloss.JoinVertical(lipgloss.Left,
		th.Faded.Render("TABLE"),
		lipgloss.Place(52, 6, lipgloss.Left, lipgloss.Center, table))
	middle := lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", divider, "  ", tableBox)

	// Your hand, wrapping to two rows if it gets long.
	var chips []string
	for i, c := range me.hand {
		look := cards.Normal
		if w.actor() == seat && !w.playable(seat, i) {
			look = cards.Dimmed
		}
		if i == me.sel {
			look = cards.Selected
		}
		chips = append(chips, cards.Chip(th, c, look))
	}
	const perRow = 15
	var rows []string
	for i := 0; i < len(chips); i += perRow {
		rows = append(rows, strings.Join(chips[i:min(i+perRow, len(chips))], " "))
	}
	if len(rows) == 0 {
		rows = []string{th.Faded.Render("no cards")}
	}
	cursor := ""
	if len(me.hand) > 0 {
		col := me.sel % perRow
		cursor = strings.Repeat(" ", col*5+1) + th.Fg(theme.Amber).Render("▲")
	}
	myLine := th.Fg(me.info.Color).Bold(true).Render(me.info.Name+" (you)") + th.Faded.Render("  ·  ") + role(seat)

	body := lipgloss.JoinVertical(lipgloss.Left,
		oppLine,
		oppCards,
		"",
		middle,
		"",
		myLine,
		strings.Join(rows, "\n"),
		cursor,
		w.status(seat, th),
		th.Faded.Render(strings.Join(w.log, "  ·  ")),
	)
	return lipgloss.Place(ViewW, ViewH, lipgloss.Left, lipgloss.Top, body)
}

func (w *World) status(seat int, th *theme.Theme) string {
	if w.over {
		switch w.winner {
		case seat:
			return th.Fg(theme.Amber).Bold(true).Render("★ You're out first. Someone else is the durak!")
		case -1:
			return th.Bold.Render("A draw!")
		}
		return th.Error.Bold(true).Render("You're the durak!")
	}
	left := int((turnTime - (w.clock - w.turnStart) + time.Second - 1) / time.Second)
	timer := th.Dim.Render(strconv.Itoa(max(0, left)) + "s")
	if left <= 5 {
		timer = th.Error.Bold(true).Render(strconv.Itoa(max(0, left)) + "s")
	}
	key := th.Key.Render
	if w.actor() != seat {
		return th.Dim.Render("Waiting for "+w.p[1-seat].info.Name+"…  ") + timer
	}
	var msg string
	switch {
	case seat == w.attacker && len(w.table) == 0:
		msg = th.Fg(theme.Lime).Bold(true).Render("▶ Your attack: ") + th.Faded.Render("play a card with ") + key("enter")
	case seat == w.attacker && w.taking:
		msg = th.Fg(theme.Lime).Bold(true).Render("▶ They're taking. ") + th.Faded.Render("Pile on with ") + key("enter") + th.Faded.Render(" or ") + key("d") + th.Faded.Render(" to finish")
	case seat == w.attacker:
		msg = th.Fg(theme.Lime).Bold(true).Render("▶ Add a card ") + th.Faded.Render("with ") + key("enter") + th.Faded.Render(" or ") + key("d") + th.Faded.Render(" for bito")
	default:
		msg = th.Fg(theme.Lime).Bold(true).Render("▶ Defend: ") + th.Faded.Render("beat it with ") + key("enter") + th.Faded.Render(" or ") + key("t") + th.Faded.Render(" to take")
	}
	return msg + th.Faded.Render("  ·  ") + timer
}

func intersperse(xs []string, sep string) []string {
	var out []string
	for i, x := range xs {
		if i > 0 {
			out = append(out, sep)
		}
		out = append(out, x)
	}
	return out
}
