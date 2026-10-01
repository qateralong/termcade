package invaders

import (
	"testing"
	"time"
)

// An idle player on wave one should last a while, so a real one has time to
// learn the game.
func TestIdlePlayerLastsAWhile(t *testing.T) {
	total := time.Duration(0)
	const runs = 10
	for seed := uint64(0); seed < runs; seed++ {
		e := newTest(seed + 10)
		for e.lives == startLives && e.clock < time.Minute {
			e.Update(25 * time.Millisecond)
		}
		total += e.clock
	}
	avg := total / runs
	t.Logf("an idle player loses the first life after %v on average", avg.Round(100*time.Millisecond))
	if avg < 5*time.Second {
		t.Fatalf("too punishing: first life lost after %v", avg)
	}
}
