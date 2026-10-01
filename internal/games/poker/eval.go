package poker

import (
	"sort"

	"termcade/internal/games/cards"
)

// Hand categories, weakest first.
const (
	highCard = iota
	onePair
	twoPair
	trips
	straight
	flush
	fullHouse
	quads
	straightFlush
)

var categoryNames = []string{
	"High Card", "One Pair", "Two Pair", "Three of a Kind", "Straight",
	"Flush", "Full House", "Four of a Kind", "Straight Flush",
}

// score encodes a five-card hand so that a higher score is a better hand:
// the category, then up to five tiebreak ranks, four bits each.
type score int64

func (s score) category() int { return int(s >> 20) }

// Name describes the hand category, e.g. "Two Pair".
func (s score) Name() string { return categoryNames[s.category()] }

func makeScore(cat int, ranks ...int) score {
	v := int64(cat)
	for i := 0; i < 5; i++ {
		v <<= 4
		if i < len(ranks) {
			v |= int64(ranks[i])
		}
	}
	return score(v)
}

// eval5 scores exactly five cards.
func eval5(h [5]cards.Card) score {
	var counts [15]int
	suitCount := map[cards.Suit]int{}
	for _, c := range h {
		counts[c.Rank]++
		suitCount[c.Suit]++
	}
	isFlush := len(suitCount) == 1

	// Straight: five distinct ranks in a row; A-2-3-4-5 counts, with the
	// five as the top card.
	var distinct []int
	for r := 14; r >= 2; r-- {
		if counts[r] > 0 {
			distinct = append(distinct, r)
		}
	}
	top := 0
	if len(distinct) == 5 {
		if distinct[0]-distinct[4] == 4 {
			top = distinct[0]
		} else if distinct[0] == 14 && distinct[1] == 5 {
			top = 5
		}
	}

	// Ranks grouped by how many of each, bigger groups and ranks first.
	type group struct{ n, r int }
	var groups []group
	for r := 14; r >= 2; r-- {
		if counts[r] > 0 {
			groups = append(groups, group{counts[r], r})
		}
	}
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].n > groups[j].n })
	ranks := make([]int, len(groups))
	for i, g := range groups {
		ranks[i] = g.r
	}

	switch {
	case top > 0 && isFlush:
		return makeScore(straightFlush, top)
	case groups[0].n == 4:
		return makeScore(quads, ranks...)
	case groups[0].n == 3 && groups[1].n == 2:
		return makeScore(fullHouse, ranks...)
	case isFlush:
		return makeScore(flush, distinct...)
	case top > 0:
		return makeScore(straight, top)
	case groups[0].n == 3:
		return makeScore(trips, ranks...)
	case groups[0].n == 2 && groups[1].n == 2:
		return makeScore(twoPair, ranks...)
	case groups[0].n == 2:
		return makeScore(onePair, ranks...)
	}
	return makeScore(highCard, distinct...)
}

// best scores the best five-card hand out of five to seven cards.
func best(cs []cards.Card) score {
	top := score(-1)
	var h [5]cards.Card
	var pick func(start, k int)
	pick = func(start, k int) {
		if k == 5 {
			if s := eval5(h); s > top {
				top = s
			}
			return
		}
		for i := start; i <= len(cs)-(5-k); i++ {
			h[k] = cs[i]
			pick(i+1, k+1)
		}
	}
	pick(0, 0)
	return top
}
