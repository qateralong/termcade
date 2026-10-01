package poker

import (
	"termcade/internal/games/cards"
)

const simulations = 160

// equity estimates seat's chance to win the hand against the players still
// in, by dealing out random opponent hands and boards. Bots only use what
// they could see at the table: their own cards and the board.
func (w *World) equity(seat int) float64 {
	me := w.p[seat]
	opponents := 0
	for i, p := range w.p {
		if i != seat && inHand(p) {
			opponents++
		}
	}
	if opponents == 0 {
		return 1
	}
	known := map[cards.Card]bool{me.hole[0]: true, me.hole[1]: true}
	for _, c := range w.board {
		known[c] = true
	}
	var pool []cards.Card
	for _, c := range cards.Deck(2) {
		if !known[c] {
			pool = append(pool, c)
		}
	}
	need := 5 - len(w.board)
	wins := 0.0
	for s := 0; s < simulations; s++ {
		// Partial shuffle: only as many cards as we draw.
		draw := need + 2*opponents
		for i := 0; i < draw; i++ {
			j := i + w.rng.IntN(len(pool)-i)
			pool[i], pool[j] = pool[j], pool[i]
		}
		board := append(append([]cards.Card{}, w.board...), pool[:need]...)
		mine := best(append([]cards.Card{me.hole[0], me.hole[1]}, board...))
		bestOpp := score(-1)
		for o := 0; o < opponents; o++ {
			h := pool[need+2*o : need+2*o+2]
			if sc := best(append([]cards.Card{h[0], h[1]}, board...)); sc > bestOpp {
				bestOpp = sc
			}
		}
		switch {
		case mine > bestOpp:
			wins++
		case mine == bestOpp:
			wins += 0.5
		}
	}
	return wins / simulations
}

// botAct decides for a bot: compare its equity with the price of calling,
// raise with strong hands, and bluff now and then.
func (w *World) botAct(seat int) {
	p := w.p[seat]
	eq := w.equity(seat)
	toCall := w.curBet - p.bet
	pot := w.pot()
	r := w.rng.Float64()

	raiseBy := func(frac float64) int {
		to := w.curBet + max(w.minRaise, int(float64(pot)*frac))
		if to >= p.bet+p.chips*7/10 {
			return p.bet + p.chips // might as well shove
		}
		return to
	}

	if toCall == 0 {
		switch {
		case eq > 0.7 || (eq > 0.5 && r < 0.35):
			w.act(seat, raise, raiseBy(0.6))
		case r < 0.07: // bluff
			w.act(seat, raise, raiseBy(0.5))
		default:
			w.act(seat, check, 0)
		}
		return
	}

	odds := float64(toCall) / float64(pot+toCall)
	switch {
	case eq > 0.78 && r < 0.6:
		w.act(seat, raise, raiseBy(0.8))
	case eq > odds+0.05:
		w.act(seat, call, 0)
	case r < 0.04 && toCall <= p.chips/10:
		w.act(seat, call, 0) // a loose call keeps them guessing
	default:
		w.act(seat, fold, 0)
	}
}
