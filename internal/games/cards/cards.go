// Package cards has playing cards and their terminal rendering, shared by
// the card games.
package cards

import (
	"math/rand/v2"
	"strconv"

	"github.com/charmbracelet/lipgloss"

	"termcade/internal/ui/theme"
)

// Suit of a card.
type Suit int

const (
	Spades Suit = iota
	Hearts
	Diamonds
	Clubs
)

// Symbol returns the suit's symbol.
func (s Suit) Symbol() string { return [...]string{"♠", "♥", "♦", "♣"}[s] }

// Red reports whether the suit is red.
func (s Suit) Red() bool { return s == Hearts || s == Diamonds }

// Rank of a card, 2 to 14 (ace).
type Rank int

const (
	Jack  Rank = 11
	Queen Rank = 12
	King  Rank = 13
	Ace   Rank = 14
)

// String returns "2".."10", "J", "Q", "K", "A".
func (r Rank) String() string {
	switch r {
	case Jack:
		return "J"
	case Queen:
		return "Q"
	case King:
		return "K"
	case Ace:
		return "A"
	}
	return strconv.Itoa(int(r))
}

// Card is a playing card.
type Card struct {
	Rank Rank
	Suit Suit
}

// String returns e.g. "10♥".
func (c Card) String() string { return c.Rank.String() + c.Suit.Symbol() }

// Deck returns a sorted deck with ranks from low to Ace in every suit:
// low=2 for 52 cards, low=6 for the 36-card deck.
func Deck(low Rank) []Card {
	var d []Card
	for s := Spades; s <= Clubs; s++ {
		for r := low; r <= Ace; r++ {
			d = append(d, Card{r, s})
		}
	}
	return d
}

// Shuffle shuffles cards in place.
func Shuffle(rng *rand.Rand, d []Card) {
	rng.Shuffle(len(d), func(i, j int) { d[i], d[j] = d[j], d[i] })
}

// Look is how a card is drawn.
type Look int

const (
	Normal   Look = iota
	Selected      // highlighted, e.g. under the cursor
	Dimmed        // can't be played right now
)

const (
	paper    = "#F4EFE6"
	ink      = "#1F1A2E"
	redInk   = "#D7263D"
	backBg   = "#4B3FB0"
	backFg   = "#8C80F0"
	selPaper = "#FFE38A"
)

// Chip draws a card in one line, three cells wide plus padding: " 7♠ ".
func Chip(t *theme.Theme, c Card, look Look) string {
	fg := ink
	if c.Suit.Red() {
		fg = redInk
	}
	bg := paper
	switch look {
	case Selected:
		bg = selPaper
	case Dimmed:
		bg = "#9A93A8"
	}
	label := c.String()
	if len([]rune(label)) < 3 {
		label = " " + label
	}
	return t.R.NewStyle().Background(lipgloss.Color(bg)).Foreground(lipgloss.Color(fg)).Bold(true).Render(label + " ")
}

// Back draws a face-down card chip, the same size as Chip.
func Back(t *theme.Theme) string {
	return t.R.NewStyle().Background(lipgloss.Color(backBg)).Foreground(lipgloss.Color(backFg)).Render("░░░░")
}

// Big draws a card three lines tall and five cells wide.
func Big(t *theme.Theme, c Card, look Look) string {
	fg := ink
	if c.Suit.Red() {
		fg = redInk
	}
	bg := paper
	if look == Selected {
		bg = selPaper
	}
	st := t.R.NewStyle().Background(lipgloss.Color(bg)).Foreground(lipgloss.Color(fg)).Bold(true)
	r := c.Rank.String()
	top := r + c.Suit.Symbol()
	for len([]rune(top)) < 5 {
		top += " "
	}
	bottom := c.Suit.Symbol() + r
	for len([]rune(bottom)) < 5 {
		bottom = " " + bottom
	}
	return st.Render(top) + "\n" + st.Render("  "+c.Suit.Symbol()+"  ") + "\n" + st.Render(bottom)
}

// BigBack draws a face-down card the size of Big.
func BigBack(t *theme.Theme) string {
	st := t.R.NewStyle().Background(lipgloss.Color(backBg)).Foreground(lipgloss.Color(backFg))
	return st.Render("░░░░░") + "\n" + st.Render("░░░░░") + "\n" + st.Render("░░░░░")
}

// Blank is empty space the size of Big.
func Blank() string { return "     \n     \n     " }
