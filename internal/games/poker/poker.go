// Package poker is no-limit Texas hold'em for four. Everyone starts with the
// same stack; the blinds climb as the game goes on, and the game ends when
// one player has all the chips or the hand limit is reached.
package poker

import (
	"math/rand/v2"
	"sort"
	"strconv"
	"time"

	"termcade/internal/games"
	"termcade/internal/games/cards"
	"termcade/internal/games/multi"
	"termcade/internal/games/solo"
	"termcade/internal/ui/theme"
)

const (
	seats      = 4
	startStack = 1000
	smallBlind = 10
	blindEvery = 4 // hands between blind increases
	maxHands   = 16
	turnTime   = 25 * time.Second
	botMin     = 900 * time.Millisecond
	botJitter  = 900 // milliseconds
	showTime   = 5 * time.Second
)

// Game returns Poker.
func Game() games.Game {
	return multi.New(multi.Config{
		Info: games.Info{
			ID:          "poker",
			Name:        "Poker",
			Icon:        "♦",
			Tagline:     "Texas hold'em for four. Chips, bluffs, showdowns.",
			Description: "No-limit Texas hold'em at a four-seat table, 1,000 chips each, with blinds that climb every few hands. Read the others, pick your moment, and take their stacks.",
			Kind:        games.TurnBased,
			Players:     "4",
			Controls:    []string{"c", "check/call", "↑↓", "raise size", "r", "raise", "a", "all-in", "f", "fold"},
			Accent:      []string{theme.Lime, theme.Amber, theme.Coral},
			Art: []string{
				" ╭────╮╭────╮  ◎◎◎  ╭────╮",
				" │ A♠ ││ A♥ │  ◎◎   │ ?? │",
				" ╰────╯╰────╯       ╰────╯",
			},
		},
		Seats: seats,
		NewWorld: func(s []multi.SeatInfo, rng *rand.Rand) multi.World {
			return New(s, rng)
		},
	})
}

type stage int

const (
	preflop stage = iota
	flopStage
	turnStage
	riverStage
	showdown // showing the result of a hand
)

type player struct {
	info    multi.SeatInfo
	bot     bool
	chips   int
	hole    [2]cards.Card
	folded  bool
	allIn   bool
	busted  bool
	bet     int // this betting round
	contrib int // this hand
	acted   bool
	last    string // last action, shown at the seat
	raiseTo int    // raise size picked in the UI
	shown   bool   // cards revealed at showdown
	won     int    // chips won in the last hand
}

// World is one game of poker.
type World struct {
	rng      *rand.Rand
	p        []*player
	deck     []cards.Card
	board    []cards.Card
	dealer   int
	stage    stage
	toAct    int
	curBet   int
	minRaise int
	hand     int
	sb, bb   int

	clock     time.Duration
	turnStart time.Duration
	botAt     time.Duration
	showUntil time.Duration

	over      bool
	bustOrder []int
	log       []string
	result    string
}

// New starts a game.
func New(seatsInfo []multi.SeatInfo, rng *rand.Rand) *World {
	w := &World{rng: rng, dealer: rng.IntN(len(seatsInfo))}
	for _, s := range seatsInfo {
		w.p = append(w.p, &player{info: s, bot: s.Bot, chips: startStack})
	}
	w.startHand()
	return w
}

func (w *World) say(s string) {
	w.log = append(w.log, s)
	if len(w.log) > 2 {
		w.log = w.log[len(w.log)-2:]
	}
}

func (w *World) alive() []int {
	var out []int
	for i, p := range w.p {
		if !p.busted {
			out = append(out, i)
		}
	}
	return out
}

// next returns the next seat after i that satisfies ok.
func (w *World) next(i int, ok func(*player) bool) int {
	for k := 1; k <= len(w.p); k++ {
		j := (i + k) % len(w.p)
		if ok(w.p[j]) {
			return j
		}
	}
	return -1
}

func inHand(p *player) bool    { return !p.busted && !p.folded }
func canAct(p *player) bool    { return inHand(p) && !p.allIn }
func notBusted(p *player) bool { return !p.busted }

func (w *World) startHand() {
	w.hand++
	level := (w.hand - 1) / blindEvery
	w.sb = smallBlind << level
	w.bb = 2 * w.sb
	for _, p := range w.p {
		*p = player{info: p.info, bot: p.bot, chips: p.chips, busted: p.busted}
	}
	w.deck = cards.Deck(2)
	cards.Shuffle(w.rng, w.deck)
	w.board = nil
	w.result = ""
	w.dealer = w.next(w.dealer, notBusted)
	for k := 0; k < 2; k++ {
		for i := range w.p {
			if p := w.p[(w.dealer+1+i)%len(w.p)]; !p.busted {
				p.hole[k] = w.deck[0]
				w.deck = w.deck[1:]
			}
		}
	}

	sbSeat := w.next(w.dealer, notBusted)
	if len(w.alive()) == 2 {
		sbSeat = w.dealer // heads-up: the dealer posts the small blind
	}
	bbSeat := w.next(sbSeat, notBusted)
	w.pay(sbSeat, min(w.sb, w.p[sbSeat].chips))
	w.p[sbSeat].last = "small blind"
	w.pay(bbSeat, min(w.bb, w.p[bbSeat].chips))
	w.p[bbSeat].last = "big blind"
	w.curBet, w.minRaise = w.bb, w.bb
	w.stage = preflop
	w.toAct = w.next(bbSeat, canAct)
	w.say("Hand " + strconv.Itoa(w.hand) + ": blinds " + strconv.Itoa(w.sb) + "/" + strconv.Itoa(w.bb))
	w.touch()
	w.advance(false)
}

func (w *World) pay(i, amount int) {
	p := w.p[i]
	amount = min(amount, p.chips)
	p.chips -= amount
	p.bet += amount
	p.contrib += amount
	if p.chips == 0 {
		p.allIn = true
	}
}

func (w *World) touch() {
	w.turnStart = w.clock
	w.botAt = w.clock + botMin + time.Duration(w.rng.IntN(botJitter))*time.Millisecond
	if w.toAct >= 0 {
		p := w.p[w.toAct]
		p.raiseTo = w.minRaiseTo(w.toAct)
	}
}

func (w *World) pot() int {
	n := 0
	for _, p := range w.p {
		n += p.contrib
	}
	return n
}

// minRaiseTo is the smallest legal raise for seat, capped by its stack.
func (w *World) minRaiseTo(seat int) int {
	p := w.p[seat]
	return min(w.curBet+w.minRaise, p.bet+p.chips)
}

type action int

const (
	fold action = iota
	check
	call
	raise
	allIn
)

// act applies a player's action. It returns false if it isn't their turn
// or the action isn't legal.
func (w *World) act(seat int, a action, to int) bool {
	if w.over || w.stage == showdown || seat != w.toAct {
		return false
	}
	p := w.p[seat]
	toCall := w.curBet - p.bet
	switch a {
	case fold:
		p.folded = true
		p.last = "fold"
	case check:
		if toCall > 0 {
			return false
		}
		p.last = "check"
	case call:
		if toCall <= 0 {
			return w.act(seat, check, 0)
		}
		w.pay(seat, toCall)
		p.last = "call " + strconv.Itoa(p.bet)
		if p.allIn {
			p.last = "all-in " + strconv.Itoa(p.bet)
		}
	case raise, allIn:
		maxTo := p.bet + p.chips
		if a == allIn {
			to = maxTo
		}
		to = min(to, maxTo)
		if to <= w.curBet {
			return w.act(seat, call, 0)
		}
		if to < w.curBet+w.minRaise && to < maxTo {
			return false
		}
		if to-w.curBet >= w.minRaise {
			w.minRaise = to - w.curBet
		}
		w.curBet = to
		w.pay(seat, to-p.bet)
		for i, o := range w.p {
			if i != seat {
				o.acted = false
			}
		}
		p.last = "raise to " + strconv.Itoa(to)
		if p.allIn {
			p.last = "all-in " + strconv.Itoa(to)
		}
	}
	p.acted = true
	w.say(p.info.Name + ": " + p.last)
	w.advance(true)
	return true
}

// advance moves the hand forward after an action (or at the start).
func (w *World) advance(moved bool) {
	var live []int
	for i, p := range w.p {
		if inHand(p) {
			live = append(live, i)
		}
	}
	if len(live) == 1 {
		w.award([][]int{{live[0]}}, false)
		return
	}

	roundDone := true
	actors := 0
	for _, p := range w.p {
		if canAct(p) {
			actors++
			if !p.acted || p.bet < w.curBet {
				roundDone = false
			}
		}
	}
	// Nobody left to bet against: one player facing only all-ins who has
	// matched the bet doesn't need to act.
	if actors <= 1 {
		roundDone = true
		for _, p := range w.p {
			if canAct(p) && p.bet < w.curBet {
				roundDone = false
			}
		}
	}

	if !roundDone {
		if moved {
			w.toAct = w.next(w.toAct, func(p *player) bool { return canAct(p) && (!p.acted || p.bet < w.curBet) })
		}
		if w.toAct < 0 || !canAct(w.p[w.toAct]) {
			w.toAct = w.next(w.dealer, func(p *player) bool { return canAct(p) && (!p.acted || p.bet < w.curBet) })
		}
		w.touch()
		return
	}

	// Next street.
	for _, p := range w.p {
		p.bet, p.acted = 0, false
	}
	w.curBet, w.minRaise = 0, w.bb
	for {
		switch w.stage {
		case preflop:
			w.deal(3)
			w.stage = flopStage
		case flopStage:
			w.deal(1)
			w.stage = turnStage
		case turnStage:
			w.deal(1)
			w.stage = riverStage
		case riverStage:
			w.showdown()
			return
		}
		if actors > 1 {
			break
		}
		// Everyone is all-in: run the board out.
	}
	w.toAct = w.next(w.dealer, canAct)
	w.touch()
}

func (w *World) deal(n int) {
	w.deck = w.deck[1:] // burn
	w.board = append(w.board, w.deck[:n]...)
	w.deck = w.deck[n:]
}

// showdown compares hands and pays out every pot, side pots included.
func (w *World) showdown() {
	scores := map[int]score{}
	for i, p := range w.p {
		if inHand(p) {
			scores[i] = best(append([]cards.Card{p.hole[0], p.hole[1]}, w.board...))
			p.shown = true
		}
	}
	// Rank players best first; ties share a group.
	var order []int
	for i := range scores {
		order = append(order, i)
	}
	sort.Slice(order, func(a, b int) bool { return scores[order[a]] > scores[order[b]] })
	var groups [][]int
	for _, i := range order {
		if n := len(groups); n > 0 && scores[groups[n-1][0]] == scores[i] {
			groups[n-1] = append(groups[n-1], i)
		} else {
			groups = append(groups, []int{i})
		}
	}
	w.award(groups, true)
	if len(groups) > 0 {
		top := groups[0][0]
		w.result += " with " + scores[top].Name()
	}
}

// award splits the pot into main and side pots by contribution level and
// pays each to the best eligible hands. groups ranks the contenders, best
// first, with ties grouped.
func (w *World) award(groups [][]int, shown bool) {
	var levels []int
	seen := map[int]bool{}
	for _, p := range w.p {
		if p.contrib > 0 && !seen[p.contrib] {
			seen[p.contrib] = true
			levels = append(levels, p.contrib)
		}
	}
	sort.Ints(levels)
	prev := 0
	for _, lv := range levels {
		slice := 0
		for _, p := range w.p {
			slice += max(0, min(p.contrib, lv)-prev)
		}
		prev = lv
		// The best group with someone eligible for this level wins it.
		for _, g := range groups {
			var winners []int
			for _, i := range g {
				if w.p[i].contrib >= lv {
					winners = append(winners, i)
				}
			}
			if len(winners) == 0 {
				continue
			}
			share := slice / len(winners)
			rest := slice - share*len(winners)
			for k, i := range winners {
				amt := share
				if k == 0 {
					amt += rest
				}
				w.p[i].chips += amt
				w.p[i].won += amt
			}
			break
		}
	}

	// The pot has been paid out.
	for _, p := range w.p {
		p.contrib, p.bet = 0, 0
	}

	var names []string
	for _, p := range w.p {
		if p.won > 0 {
			names = append(names, p.info.Name+" wins "+strconv.Itoa(p.won))
		}
	}
	w.result = joinComma(names)
	if !shown {
		w.result += " (everyone else folded)"
	}
	w.stage = showdown
	w.toAct = -1
	w.showUntil = w.clock + showTime
}

func joinComma(xs []string) string {
	out := ""
	for i, x := range xs {
		if i > 0 {
			out += ", "
		}
		out += x
	}
	return out
}

// Input implements multi.World.
func (w *World) Input(seat int, key string) {
	p := w.p[seat]
	switch key {
	case "f":
		w.act(seat, fold, 0)
	case "c", "k", " ":
		w.act(seat, call, 0)
	case "r", "enter":
		w.act(seat, raise, p.raiseTo)
	case "a":
		w.act(seat, allIn, 0)
	case "up", "right", "+", "=":
		if seat == w.toAct {
			step := w.bb
			if p.raiseTo >= 10*w.bb {
				step = 5 * w.bb
			}
			p.raiseTo = min(p.raiseTo+step, p.bet+p.chips)
		}
	case "down", "left", "-":
		if seat == w.toAct {
			step := w.bb
			if p.raiseTo > 10*w.bb {
				step = 5 * w.bb
			}
			p.raiseTo = max(p.raiseTo-step, w.minRaiseTo(seat))
		}
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
	if w.stage == showdown {
		if w.clock < w.showUntil {
			return
		}
		for i, p := range w.p {
			if !p.busted && p.chips == 0 {
				p.busted = true
				w.bustOrder = append(w.bustOrder, i)
				w.say(p.info.Name + " is out of chips")
			}
		}
		if len(w.alive()) <= 1 || w.hand >= maxHands {
			w.over = true
			return
		}
		w.startHand()
		return
	}
	if w.toAct < 0 {
		return
	}
	p := w.p[w.toAct]
	timedOut := w.clock-w.turnStart >= turnTime
	switch {
	case p.bot && w.clock >= w.botAt:
		w.botAct(w.toAct)
	case timedOut:
		if w.curBet > p.bet {
			w.act(w.toAct, fold, 0)
		} else {
			w.act(w.toAct, check, 0)
		}
	}
}

// Over implements multi.World.
func (w *World) Over() bool { return w.over }

// Standings implements multi.World: players still in by chips, then the
// busted in reverse order.
func (w *World) Standings() []multi.Standing {
	alive := w.alive()
	sort.SliceStable(alive, func(a, b int) bool { return w.p[alive[a]].chips > w.p[alive[b]].chips })
	var res []multi.Standing
	for _, i := range alive {
		res = append(res, multi.Standing{Seat: i, Detail: solo.Thousands(w.p[i].chips) + " chips"})
	}
	for k := len(w.bustOrder) - 1; k >= 0; k-- {
		res = append(res, multi.Standing{Seat: w.bustOrder[k], Detail: "busted"})
	}
	return res
}

// Stats implements multi.World.
func (w *World) Stats(seat int) []multi.Stat {
	return []multi.Stat{
		{Label: "CHIPS", Value: solo.Thousands(w.p[seat].chips)},
		{Label: "HAND", Value: strconv.Itoa(w.hand) + "/" + strconv.Itoa(maxHands)},
		{Label: "BLINDS", Value: strconv.Itoa(w.sb) + "/" + strconv.Itoa(w.bb)},
	}
}
