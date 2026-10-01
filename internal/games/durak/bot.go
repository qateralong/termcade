package durak

import "termcade/internal/games/cards"

// value ranks a card for the bot: trumps are worth far more than anything
// else, so it saves them.
func (w *World) value(c cards.Card) int {
	v := int(c.Rank)
	if c.Suit == w.trump {
		v += 20
	}
	return v
}

// botMove plays one move for seat: the cheapest legal attack or defense,
// and takes or calls bito when that's the sensible thing to do.
func (w *World) botMove(seat int) {
	p := w.p[seat]
	early := len(w.deck) > 8

	if seat == w.attacker {
		best := -1
		for i, c := range p.hand {
			if !w.canAdd(c) {
				continue
			}
			// Don't pile on with trumps or high cards while the deck is
			// still thick, except to open the bout.
			if len(w.table) > 0 && early && (c.Suit == w.trump || c.Rank >= cards.King) {
				continue
			}
			if best < 0 || w.value(c) < w.value(p.hand[best]) {
				best = i
			}
		}
		if best >= 0 && (len(w.table) == 0 || w.rng.IntN(100) < 85) {
			w.play(seat, best)
			return
		}
		if !w.done(seat) && len(w.table) == 0 && len(p.hand) > 0 {
			w.play(seat, 0) // must open with something
		}
		return
	}

	// Defending: beat the first open card as cheaply as possible.
	for _, pr := range w.table {
		if pr.beaten {
			continue
		}
		best := -1
		for i, c := range p.hand {
			if w.beats(c, pr.atk) && (best < 0 || w.value(c) < w.value(p.hand[best])) {
				best = i
			}
		}
		// Spending a big trump on a small card early on isn't worth it.
		if best >= 0 && early && p.hand[best].Suit == w.trump && p.hand[best].Rank >= cards.Queen && pr.atk.Rank <= 9 {
			best = -1
		}
		if best < 0 {
			w.take(seat)
			return
		}
		w.play(seat, best)
		return
	}
}
