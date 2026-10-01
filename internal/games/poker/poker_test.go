package poker

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

func c(s string) cards.Card {
	ranks := map[byte]cards.Rank{'2': 2, '3': 3, '4': 4, '5': 5, '6': 6, '7': 7, '8': 8, '9': 9, 'T': 10, 'J': 11, 'Q': 12, 'K': 13, 'A': 14}
	suits := map[byte]cards.Suit{'s': cards.Spades, 'h': cards.Hearts, 'd': cards.Diamonds, 'c': cards.Clubs}
	return cards.Card{Rank: ranks[s[0]], Suit: suits[s[1]]}
}

func hand(ss ...string) []cards.Card {
	var out []cards.Card
	for _, s := range ss {
		out = append(out, c(s))
	}
	return out
}

func TestHandRanking(t *testing.T) {
	ordered := [][]cards.Card{
		hand("2s", "4h", "7d", "9c", "Jh", "3d", "5s"), // high card
		hand("2s", "2h", "7d", "9c", "Jh", "3d", "5s"), // pair
		hand("2s", "2h", "7d", "7c", "Jh", "3d", "5s"), // two pair
		hand("7s", "7h", "7d", "9c", "Jh", "3d", "2s"), // trips
		hand("As", "2h", "3d", "4c", "5h", "9d", "Js"), // wheel straight
		hand("6s", "7h", "8d", "9c", "Th", "2d", "2s"), // straight
		hand("2h", "7h", "9h", "Jh", "Kh", "3d", "5s"), // flush
		hand("7s", "7h", "7d", "9c", "9h", "3d", "2s"), // full house
		hand("7s", "7h", "7d", "7c", "Jh", "3d", "2s"), // quads
		hand("5h", "6h", "7h", "8h", "9h", "Ad", "As"), // straight flush
	}
	for i := 1; i < len(ordered); i++ {
		if best(ordered[i]) <= best(ordered[i-1]) {
			t.Errorf("%s should beat %s", best(ordered[i]).Name(), best(ordered[i-1]).Name())
		}
	}
	if best(ordered[4]).Name() != "Straight" || best(ordered[9]).Name() != "Straight Flush" {
		t.Fatal("wrong category names")
	}
	// Kickers count.
	a := best(hand("As", "Ah", "Kd", "9c", "5h", "3d", "2s"))
	b := best(hand("As", "Ah", "Qd", "9c", "5h", "3d", "2s"))
	if a <= b {
		t.Fatal("king kicker should beat queen kicker")
	}
	// The board plays: identical best hands tie.
	if best(hand("2c", "3d", "Ts", "Js", "Qs", "Ks", "As")) != best(hand("4c", "5d", "Ts", "Js", "Qs", "Ks", "As")) {
		t.Fatal("board royal flush should tie")
	}
}

func seatsOf(n int, bots bool) []multi.SeatInfo {
	out := make([]multi.SeatInfo, n)
	for i := range out {
		out[i] = multi.SeatInfo{Name: "p" + string(rune('a'+i)), Color: theme.PlayerColors[i].Hex, Bot: bots}
	}
	return out
}

func totalChips(w *World) int {
	n := 0
	for _, p := range w.p {
		n += p.chips + p.contrib
	}
	return n
}

func TestSidePots(t *testing.T) {
	w := New(seatsOf(3, false), rand.New(rand.NewPCG(1, 1)))
	// Reset to a hand-crafted all-in situation: A has 100 in, B and C 300.
	for _, p := range w.p {
		p.contrib, p.bet, p.folded, p.won = 0, 0, false, 0
	}
	w.p[0].contrib, w.p[1].contrib, w.p[2].contrib = 100, 300, 300
	w.p[0].chips, w.p[1].chips, w.p[2].chips = 0, 0, 500
	// A has the best hand, B second, C worst.
	w.award([][]int{{0}, {1}, {2}}, true)
	if w.p[0].won != 300 || w.p[1].won != 400 || w.p[2].won != 0 {
		t.Fatalf("won: %d %d %d", w.p[0].won, w.p[1].won, w.p[2].won)
	}
	// A split pot.
	for _, p := range w.p {
		p.contrib, p.won, p.chips = 100, 0, 0
	}
	w.award([][]int{{0, 1}, {2}}, true)
	if w.p[0].won+w.p[1].won != 300 || w.p[2].won != 0 {
		t.Fatalf("split: %d %d %d", w.p[0].won, w.p[1].won, w.p[2].won)
	}
}

func TestUncalledBetIsReturned(t *testing.T) {
	w := New(seatsOf(2, false), rand.New(rand.NewPCG(9, 9)))
	for _, p := range w.p {
		p.contrib, p.won, p.chips = 0, 0, 0
	}
	// A short all-in for 2 against a 40 blind, and then a fold: the all-in
	// player wins 4, and the other 38 go back to the folder.
	w.p[0].contrib, w.p[1].contrib = 2, 40
	w.p[1].folded = true
	w.award([][]int{{0}}, false)
	if w.p[0].chips != 4 || w.p[1].chips != 38 {
		t.Fatalf("chips: %d and %d", w.p[0].chips, w.p[1].chips)
	}
}

func TestBettingRound(t *testing.T) {
	w := New(seatsOf(3, false), rand.New(rand.NewPCG(2, 2)))
	start := totalChips(w)
	first := w.toAct
	if w.act((first+1)%3, call, 0) {
		t.Fatal("acting out of turn should be refused")
	}
	// Everyone calls, then the big blind checks: on to the flop.
	for w.stage == preflop {
		if !w.act(w.toAct, call, 0) {
			t.Fatal("call refused")
		}
	}
	if w.stage != flopStage || len(w.board) != 3 {
		t.Fatalf("stage %v board %d", w.stage, len(w.board))
	}
	// A raise below the minimum is refused.
	if w.act(w.toAct, raise, w.bb/2) {
		t.Fatal("tiny raise accepted")
	}
	if totalChips(w) != start {
		t.Fatal("chips appeared or vanished")
	}
}

func TestEveryoneFoldsToTheRaiser(t *testing.T) {
	w := New(seatsOf(4, false), rand.New(rand.NewPCG(3, 3)))
	raiser := w.toAct
	if !w.act(raiser, raise, 100) {
		t.Fatal("raise refused")
	}
	for w.stage != showdown {
		w.act(w.toAct, fold, 0)
	}
	if w.p[raiser].won != 100+w.sb+w.bb {
		t.Fatalf("raiser won %d", w.p[raiser].won)
	}
}

func TestBotsPlayAWholeGame(t *testing.T) {
	for seed := uint64(0); seed < 4; seed++ {
		w := New(seatsOf(seats, true), rand.New(rand.NewPCG(seed, 5)))
		total := totalChips(w)
		for !w.Over() && w.clock < 2*time.Hour {
			w.Step(200 * time.Millisecond)
			if got := totalChips(w); got != total {
				t.Fatalf("seed %d hand %d: %d chips in play, want %d", seed, w.hand, got, total)
			}
			for _, p := range w.p {
				if p.chips < 0 {
					t.Fatal("negative stack")
				}
			}
		}
		if !w.Over() {
			t.Fatal("game never ended")
		}
		t.Logf("seed %d: %d hands, %d busted, %v", seed, w.hand, len(w.bustOrder), w.clock.Round(time.Second))
		if st := w.Standings(); len(st) != seats {
			t.Fatalf("standings has %d rows", len(st))
		}
	}
}

func TestViewSize(t *testing.T) {
	w := New(seatsOf(seats, true), rand.New(rand.NewPCG(6, 6)))
	th := theme.New(lipgloss.NewRenderer(io.Discard))
	for i := 0; i < 300 && !w.Over(); i++ {
		w.Step(400 * time.Millisecond)
		for s := 0; s < seats; s++ {
			v := w.View(s, th)
			if lw, lh := lipgloss.Width(v), lipgloss.Height(v); lw != ViewW || lh != ViewH {
				t.Fatalf("step %d seat %d: view is %dx%d", i, s, lw, lh)
			}
		}
	}
}
