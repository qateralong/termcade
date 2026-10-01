package poker

import (
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"termcade/internal/games/cards"
	"termcade/internal/games/solo"
	"termcade/internal/ui/layout"
	"termcade/internal/ui/theme"
)

// ViewW and ViewH are the fixed size of the play area.
const (
	ViewW = 78
	ViewH = 20
	seatW = 24
)

// View implements multi.World.
func (w *World) View(seat int, th *theme.Theme) string {
	n := len(w.p)
	at := func(k int) int { return (seat + k) % n }

	// Seats around the table: you at the bottom, then clockwise left, top
	// and right.
	bottom := w.seatBox(th, at(0), seat)
	left := w.seatBox(th, at(1), seat)
	top := ""
	right := ""
	if n > 2 {
		top = w.seatBox(th, at(2), seat)
	}
	if n > 3 {
		right = w.seatBox(th, at(3), seat)
	}

	// The middle: community cards and the pot.
	var board []string
	for i := 0; i < 5; i++ {
		if i < len(w.board) {
			board = append(board, cards.Chip(th, w.board[i], cards.Normal))
		} else {
			board = append(board, th.R.NewStyle().Foreground(lipgloss.Color("#3B3452")).Render("·   "))
		}
	}
	potLine := th.Faded.Render("POT ") + th.Fg(theme.Amber).Bold(true).Render(solo.Thousands(w.pot()))
	center := lipgloss.JoinVertical(lipgloss.Center, strings.Join(board, " "), "", potLine)
	middle := lipgloss.JoinHorizontal(lipgloss.Center,
		lipgloss.Place(seatW, 5, lipgloss.Left, lipgloss.Center, left),
		lipgloss.Place(ViewW-2*seatW, 5, lipgloss.Center, lipgloss.Center, center),
		lipgloss.Place(seatW, 5, lipgloss.Right, lipgloss.Center, right),
	)

	body := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.Place(ViewW, 4, lipgloss.Center, lipgloss.Top, top),
		middle,
		lipgloss.Place(ViewW, 5, lipgloss.Center, lipgloss.Bottom, bottom),
		"",
		w.status(th, seat),
		th.Faded.Render(strings.Join(w.log, "  ·  ")),
	)
	return lipgloss.Place(ViewW, ViewH, lipgloss.Left, lipgloss.Top, body)
}

// seatBox draws one player: name, chips, cards, bet and last action.
func (w *World) seatBox(th *theme.Theme, i, viewer int) string {
	p := w.p[i]
	name := th.Fg(p.info.Color).Bold(true).Render(p.info.Name)
	if i == viewer {
		name += th.Faded.Render(" (you)")
	}
	if i == w.dealer {
		name += " " + th.R.NewStyle().Background(lipgloss.Color("#FFFFFF")).Foreground(lipgloss.Color("#1A1325")).Bold(true).Render("D")
	}
	chips := th.Dim.Render(solo.Thousands(p.chips) + " chips")

	var hole string
	switch {
	case p.busted:
		hole = th.Faded.Render("out of chips")
	case p.folded:
		hole = th.Faded.Render("folded")
	case i == viewer || p.shown:
		hole = cards.Chip(th, p.hole[0], cards.Normal) + " " + cards.Chip(th, p.hole[1], cards.Normal)
	default:
		hole = cards.Back(th) + " " + cards.Back(th)
	}

	var info string
	switch {
	case p.won > 0 && w.stage == showdown:
		info = th.Fg(theme.Amber).Bold(true).Render("+" + solo.Thousands(p.won))
	case p.bet > 0:
		info = th.Fg(theme.Lime).Render("bet " + solo.Thousands(p.bet))
	}
	if p.last != "" && !(p.won > 0 && w.stage == showdown) {
		if info != "" {
			info += th.Faded.Render(" · ")
		}
		info += th.Faded.Render(p.last)
	}

	box := th.Panel
	if i == w.toAct {
		box = th.PanelOn.BorderForeground(lipgloss.Color(theme.Amber))
	}
	return box.Width(seatW - 2).Render(lipgloss.JoinVertical(lipgloss.Left,
		ansiFit(name+"  "+chips, seatW-2), hole, ansiFit(info, seatW-2)))
}

func ansiFit(s string, w int) string {
	if lipgloss.Width(s) > w {
		return layout.PadRight(trimTo(s, w), w)
	}
	return s
}

func trimTo(s string, w int) string {
	r := []rune(s)
	for lipgloss.Width(string(r)) > w && len(r) > 0 {
		r = r[:len(r)-1]
	}
	return string(r)
}

func (w *World) status(th *theme.Theme, seat int) string {
	if w.over {
		return th.Bold.Render("Game over.")
	}
	if w.stage == showdown {
		return th.Fg(theme.Amber).Bold(true).Render("★ "+w.result) + th.Faded.Render("  ·  next hand soon")
	}
	left := int((turnTime - (w.clock - w.turnStart) + time.Second - 1) / time.Second)
	timer := th.Dim.Render(strconv.Itoa(max(0, left)) + "s")
	if left <= 5 {
		timer = th.Error.Bold(true).Render(strconv.Itoa(max(0, left)) + "s")
	}
	if w.toAct != seat {
		if w.toAct < 0 {
			return ""
		}
		return th.Dim.Render("Waiting for "+w.p[w.toAct].info.Name+"…  ") + timer
	}
	p := w.p[seat]
	key := th.Key.Render
	sep := th.Faded.Render(" · ")
	toCall := w.curBet - p.bet
	callTxt := key("c") + th.Faded.Render(" check")
	if toCall > 0 {
		callTxt = key("c") + th.Faded.Render(" call "+strconv.Itoa(min(toCall, p.chips)))
	}
	parts := []string{callTxt}
	if p.chips > toCall {
		parts = append(parts, key("↑↓ r")+th.Faded.Render(" raise to "+strconv.Itoa(p.raiseTo)))
	}
	parts = append(parts, key("a")+th.Faded.Render(" all-in"), key("f")+th.Faded.Render(" fold"), timer)
	return th.Fg(theme.Lime).Bold(true).Render("▶ Your move: ") + strings.Join(parts, sep)
}
