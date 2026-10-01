package durak

import (
	"io"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"termcade/internal/games/cards"
	"termcade/internal/games/multi"
	"termcade/internal/ui/theme"
)

func seatsOf(bots bool) []multi.SeatInfo {
	return []multi.SeatInfo{{Name: "anna", Color: theme.Pink, Bot: bots}, {Name: "boris", Color: theme.Cyan, Bot: bots}}
}

func newTest(seed uint64, bots bool) *World {
	return New(seatsOf(bots), rand.New(rand.NewPCG(seed, seed)))
}

func card(r cards.Rank, s cards.Suit) cards.Card { return cards.Card{Rank: r, Suit: s} }

func TestDeal(t *testing.T) {
	w := newTest(1, false)
	if len(w.p[0].hand) != 6 || len(w.p[1].hand) != 6 || len(w.deck) != 24 {
		t.Fatalf("hands %d/%d deck %d", len(w.p[0].hand), len(w.p[1].hand), len(w.deck))
	}
	if w.deck[len(w.deck)-1] != w.trumpCard {
		t.Fatal("the trump card must be the last card of the deck")
	}
}

func TestBeats(t *testing.T) {
	w := newTest(2, false)
	w.trump = cards.Hearts
	cases := []struct {
		d, a cards.Card
		want bool
	}{
		{card(10, cards.Spades), card(7, cards.Spades), true},
		{card(7, cards.Spades), card(10, cards.Spades), false},
		{card(6, cards.Hearts), card(cards.Ace, cards.Spades), true},
		{card(cards.Ace, cards.Spades), card(6, cards.Hearts), false},
		{card(cards.Ace, cards.Clubs), card(6, cards.Spades), false},
		{card(8, cards.Hearts), card(7, cards.Hearts), true},
	}
	for _, c := range cases {
		if got := w.beats(c.d, c.a); got != c.want {
			t.Errorf("beats(%v, %v) = %v", c.d, c.a, got)
		}
	}
}

func TestBoutRules(t *testing.T) {
	w := newTest(3, false)
	w.trump = cards.Clubs
	w.attacker = 0
	w.firstBout = false
	w.p[0].hand = []cards.Card{card(7, cards.Spades), card(7, cards.Hearts), card(9, cards.Diamonds)}
	w.p[1].hand = []cards.Card{card(10, cards.Spades), card(6, cards.Clubs), card(8, cards.Hearts)}
	w.deck = nil
	w.startBout()

	if !w.play(0, 0) { // 7♠
		t.Fatal("opening attack refused")
	}
	if w.actor() != 1 {
		t.Fatal("defender should act after an attack")
	}
	if w.done(0) {
		t.Fatal("attacker can't call bito with an open card")
	}
	// 9♦ doesn't match any rank on the table.
	w.Input(1, "enter") // defender plays selected card 0 (10♠) onto 7♠
	if !w.table[0].beaten {
		t.Fatal("10♠ should beat 7♠")
	}
	idx := -1
	for i, c := range w.p[0].hand {
		if c == card(9, cards.Diamonds) {
			idx = i
		}
	}
	if w.play(0, idx) {
		t.Fatal("9♦ matches nothing on the table")
	}
	for i, c := range w.p[0].hand {
		if c == card(7, cards.Hearts) {
			idx = i
		}
	}
	if !w.play(0, idx) {
		t.Fatal("7♥ matches the 7 on the table")
	}
	if !w.take(1) {
		t.Fatal("defender should be able to take")
	}
	if !w.done(0) {
		t.Fatal("attacker should finish after a take")
	}
	if len(w.p[1].hand) != 2+3 {
		t.Fatalf("defender should hold 5 cards after taking, has %d", len(w.p[1].hand))
	}
	if w.attacker != 0 {
		t.Fatal("after a take the attacker attacks again")
	}
}

func TestBotsFinishGames(t *testing.T) {
	draws := 0
	for seed := uint64(0); seed < 30; seed++ {
		w := newTest(seed, true)
		for !w.Over() && w.clock < 30*time.Minute {
			w.Step(100 * time.Millisecond)
			total := len(w.deck) + len(w.p[0].hand) + len(w.p[1].hand) + w.discard
			for _, pr := range w.table {
				total++
				if pr.beaten {
					total++
				}
			}
			if total != 36 {
				t.Fatalf("seed %d: %d cards in play, want 36", seed, total)
			}
		}
		if !w.Over() {
			t.Fatalf("seed %d: game never ended", seed)
		}
		if w.winner < 0 {
			draws++
		}
		if st := w.Standings(); len(st) != 2 {
			t.Fatal("bad standings")
		}
	}
	t.Logf("%d draws in 30 games", draws)
}

func TestTimeoutPlaysForYou(t *testing.T) {
	w := newTest(5, false)
	before := len(w.p[w.attacker].hand)
	w.Step(turnTime + time.Millisecond)
	if len(w.p[w.attacker].hand) != before-1 {
		t.Fatal("timeout should play an opening card")
	}
}

func TestViewSize(t *testing.T) {
	w := newTest(6, true)
	th := theme.New(lipgloss.NewRenderer(io.Discard))
	for i := 0; i < 400 && !w.Over(); i++ {
		w.Step(300 * time.Millisecond)
		v := w.View(0, th)
		if lw, lh := lipgloss.Width(v), lipgloss.Height(v); lw != ViewW || lh != ViewH {
			t.Fatalf("step %d: view is %dx%d", i, lw, lh)
		}
	}
}
