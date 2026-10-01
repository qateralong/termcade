// Package durak is two-player podkidnoy durak: attack, defend, pile on, and
// whoever is left holding cards is the fool.
package durak

import (
	"math/rand/v2"
	"sort"
	"time"

	"termcade/internal/games"
	"termcade/internal/games/cards"
	"termcade/internal/games/multi"
	"termcade/internal/ui/theme"
)

const (
	handSize  = 6
	turnTime  = 30 * time.Second
	botMin    = 700 * time.Millisecond
	botJitter = 700 // milliseconds
)

// Game returns Durak.
func Game() games.Game {
	return multi.New(multi.Config{
		Info: games.Info{
			ID:          "durak",
			Name:        "Durak",
			Icon:        "♠",
			Tagline:     "The card game where nobody wants to be the fool.",
			Description: "Podkidnoy durak for two. Attack, beat every card or take them all, and pile on cards of the same rank. Get rid of your cards before the deck runs out, or be crowned the durak.",
			Kind:        games.TurnBased,
			Players:     "2",
			Controls:    []string{"←→", "pick card", "enter", "play", "t", "take", "d", "done (bito)"},
			Accent:      []string{theme.Pink, theme.Coral, theme.Violet},
			Art: []string{
				" ╭─────╮╭─────╮╭─────╮",
				" │ 7   ││ K   ││ A   │",
				" │  ♠  ││  ♥  ││  ♦  │",
				" ╰─────╯╰─────╯╰─────╯",
			},
		},
		Seats: 2,
		NewWorld: func(s []multi.SeatInfo, rng *rand.Rand) multi.World {
			return New(s, rng)
		},
	})
}

type player struct {
	info multi.SeatInfo
	bot  bool
	hand []cards.Card
	sel  int // cursor in hand
}

type pair struct {
	atk    cards.Card
	def    cards.Card
	beaten bool
}

// World is one game of durak.
type World struct {
	rng       *rand.Rand
	p         [2]*player
	deck      []cards.Card // dealt from the front; the trump card is last
	trump     cards.Suit
	trumpCard cards.Card
	attacker  int
	table     []pair
	taking    bool // the defender gave up and will take the table
	limit     int  // max attack cards this bout
	firstBout bool
	discard   int

	clock     time.Duration
	turnStart time.Duration
	botAt     time.Duration

	over   bool
	winner int // -1 for a draw
	log    []string
}

// New deals a game.
func New(seats []multi.SeatInfo, rng *rand.Rand) *World {
	w := &World{rng: rng, firstBout: true, winner: -1}
	w.deck = cards.Deck(6)
	cards.Shuffle(rng, w.deck)
	w.trumpCard = w.deck[len(w.deck)-1]
	w.trump = w.trumpCard.Suit
	for i := range w.p {
		w.p[i] = &player{info: seats[i], bot: seats[i].Bot}
	}
	for i := 0; i < handSize; i++ {
		for _, p := range w.p {
			p.hand = append(p.hand, w.draw())
		}
	}
	for _, p := range w.p {
		w.sortHand(p)
	}
	// Lowest trump leads.
	w.attacker = rng.IntN(2)
	best := cards.Ace + 1
	for i, p := range w.p {
		for _, c := range p.hand {
			if c.Suit == w.trump && c.Rank < best {
				best, w.attacker = c.Rank, i
			}
		}
	}
	w.startBout()
	w.say(w.p[w.attacker].info.Name + " attacks first (lowest trump)")
	return w
}

func (w *World) draw() cards.Card {
	c := w.deck[0]
	w.deck = w.deck[1:]
	return c
}

func (w *World) defender() int { return 1 - w.attacker }

func (w *World) say(s string) {
	w.log = append(w.log, s)
	if len(w.log) > 3 {
		w.log = w.log[len(w.log)-3:]
	}
}

// sortHand orders a hand by suit (trumps last) and rank.
func (w *World) sortHand(p *player) {
	key := func(c cards.Card) int {
		s := int(c.Suit)
		if c.Suit == w.trump {
			s = 10
		}
		return s*100 + int(c.Rank)
	}
	sort.Slice(p.hand, func(i, j int) bool { return key(p.hand[i]) < key(p.hand[j]) })
	p.sel = min(p.sel, max(0, len(p.hand)-1))
}

func (w *World) startBout() {
	w.table = nil
	w.taking = false
	w.limit = min(handSize, len(w.p[w.defender()].hand))
	if w.firstBout {
		w.limit = min(w.limit, 5)
	}
	w.touch()
}

// touch restarts the action timer.
func (w *World) touch() {
	w.turnStart = w.clock
	w.botAt = w.clock + botMin + time.Duration(w.rng.IntN(botJitter))*time.Millisecond
}

// beats reports whether d beats a.
func (w *World) beats(d, a cards.Card) bool {
	if d.Suit == a.Suit {
		return d.Rank > a.Rank
	}
	return d.Suit == w.trump && a.Suit != w.trump
}

func (w *World) unbeaten() int {
	n := 0
	for _, p := range w.table {
		if !p.beaten {
			n++
		}
	}
	return n
}

// actor is whose move it is.
func (w *World) actor() int {
	if !w.taking && w.unbeaten() > 0 {
		return w.defender()
	}
	return w.attacker
}

// canAdd reports whether the attacker may put c on the table.
func (w *World) canAdd(c cards.Card) bool {
	if len(w.table) >= w.limit {
		return false
	}
	// The defender must have enough cards to answer everything open.
	if w.unbeaten()+1 > len(w.p[w.defender()].hand) {
		return false
	}
	if len(w.table) == 0 {
		return true
	}
	for _, p := range w.table {
		if p.atk.Rank == c.Rank || (p.beaten && p.def.Rank == c.Rank) {
			return true
		}
	}
	return false
}

// target is the first open attack card that c beats, or -1.
func (w *World) target(c cards.Card) int {
	for i, p := range w.table {
		if !p.beaten && w.beats(c, p.atk) {
			return i
		}
	}
	return -1
}

// playable reports whether the hand card at i can be played by seat now.
func (w *World) playable(seat, i int) bool {
	if w.over || w.actor() != seat {
		return false
	}
	c := w.p[seat].hand[i]
	if seat == w.attacker {
		return w.canAdd(c)
	}
	return w.target(c) >= 0
}

func (w *World) removeCard(p *player, i int) cards.Card {
	c := p.hand[i]
	p.hand = append(p.hand[:i], p.hand[i+1:]...)
	p.sel = min(p.sel, max(0, len(p.hand)-1))
	return c
}

// play puts the hand card at i on the table for seat.
func (w *World) play(seat, i int) bool {
	if i < 0 || i >= len(w.p[seat].hand) || !w.playable(seat, i) {
		return false
	}
	p := w.p[seat]
	c := p.hand[i]
	if seat == w.attacker {
		w.removeCard(p, i)
		w.table = append(w.table, pair{atk: c})
		w.say(p.info.Name + " plays " + c.String())
	} else {
		t := w.target(c)
		w.removeCard(p, i)
		w.table[t].def, w.table[t].beaten = c, true
		w.say(p.info.Name + " beats " + w.table[t].atk.String() + " with " + c.String())
	}
	w.touch()
	return true
}

// take: the defender gives up this bout.
func (w *World) take(seat int) bool {
	if w.over || seat != w.defender() || w.taking || w.unbeaten() == 0 {
		return false
	}
	w.taking = true
	w.say(w.p[seat].info.Name + " takes")
	w.touch()
	return true
}

// done: the attacker has nothing more to add.
func (w *World) done(seat int) bool {
	if w.over || seat != w.attacker || len(w.table) == 0 || (!w.taking && w.unbeaten() > 0) {
		return false
	}
	def := w.p[w.defender()]
	if w.taking {
		for _, pr := range w.table {
			def.hand = append(def.hand, pr.atk)
			if pr.beaten {
				def.hand = append(def.hand, pr.def)
			}
		}
		w.sortHand(def)
		w.refill()
		// The defender loses their turn to attack.
	} else {
		w.discard += len(w.table) * 2
		w.say("Bito: " + def.info.Name + " fought them off")
		w.refill()
		w.attacker = w.defender()
	}
	w.table = nil
	w.firstBout = false
	w.checkEnd()
	if !w.over {
		w.startBout()
	}
	return true
}

func (w *World) refill() {
	for _, i := range []int{w.attacker, w.defender()} {
		p := w.p[i]
		for len(p.hand) < handSize && len(w.deck) > 0 {
			p.hand = append(p.hand, w.draw())
		}
		w.sortHand(p)
	}
}

func (w *World) checkEnd() {
	if len(w.deck) > 0 {
		return
	}
	a, b := len(w.p[0].hand) == 0, len(w.p[1].hand) == 0
	switch {
	case a && b:
		w.over, w.winner = true, -1
		w.say("Both out at once: a draw!")
	case a:
		w.over, w.winner = true, 0
	case b:
		w.over, w.winner = true, 1
	}
	if w.over && w.winner >= 0 {
		w.say(w.p[1-w.winner].info.Name + " is the durak!")
	}
}

// Input implements multi.World.
func (w *World) Input(seat int, key string) {
	p := w.p[seat]
	switch key {
	case "left", "h":
		if len(p.hand) > 0 {
			p.sel = (p.sel + len(p.hand) - 1) % len(p.hand)
		}
	case "right", "l":
		if len(p.hand) > 0 {
			p.sel = (p.sel + 1) % len(p.hand)
		}
	case "enter", " ":
		w.play(seat, p.sel)
	case "t":
		w.take(seat)
	case "d", "b":
		w.done(seat)
	}
}

// SetBot implements multi.World.
func (w *World) SetBot(seat int) { w.p[seat].bot = true }

// Step implements multi.World.
func (w *World) Step(dt time.Duration) {
	w.clock += dt
	if w.over {
		return
	}
	a := w.actor()
	p := w.p[a]
	if (p.bot && w.clock >= w.botAt) || w.clock-w.turnStart >= turnTime {
		if !p.bot && w.clock-w.turnStart >= turnTime {
			w.say(p.info.Name + " ran out of time")
		}
		w.botMove(a)
	}
}

// Over implements multi.World.
func (w *World) Over() bool { return w.over }

// Standings implements multi.World.
func (w *World) Standings() []multi.Standing {
	if w.winner < 0 {
		return []multi.Standing{{Seat: 0, Detail: "draw"}, {Seat: 1, Detail: "draw"}}
	}
	return []multi.Standing{
		{Seat: w.winner, Detail: "out first"},
		{Seat: 1 - w.winner, Detail: "durak"},
	}
}

// Stats implements multi.World.
func (w *World) Stats(seat int) []multi.Stat {
	return []multi.Stat{
		{Label: "TRUMP", Value: w.trump.Symbol()},
		{Label: "DECK", Value: itoa(len(w.deck))},
	}
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}
