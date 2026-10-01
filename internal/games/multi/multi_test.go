package multi

import (
	"math/rand/v2"
	"testing"
	"time"

	"termcade/internal/games"
	"termcade/internal/ui/theme"
)

// fakeWorld records what the room does with it.
type fakeWorld struct {
	seats  []SeatInfo
	inputs []string
	steps  int
	bots   map[int]bool
	over   bool
}

func (w *fakeWorld) Input(seat int, key string) { w.inputs = append(w.inputs, key) }
func (w *fakeWorld) Step(time.Duration)         { w.steps++ }
func (w *fakeWorld) SetBot(seat int)            { w.bots[seat] = true }
func (w *fakeWorld) Over() bool                 { return w.over }
func (w *fakeWorld) Standings() []Standing      { return []Standing{{Seat: 0}, {Seat: 1}} }
func (w *fakeWorld) View(int, *theme.Theme) string {
	return "world"
}
func (w *fakeWorld) Stats(int) []Stat { return nil }

func newTestGame(seats int) (*Game, *[]*fakeWorld) {
	var worlds []*fakeWorld
	g := New(Config{
		Info:  games.Info{ID: "fake", Name: "Fake"},
		Seats: seats,
		NewWorld: func(s []SeatInfo, _ *rand.Rand) World {
			w := &fakeWorld{seats: s, bots: map[int]bool{}}
			worlds = append(worlds, w)
			return w
		},
	})
	g.manual = true
	return g, &worlds
}

func player(name string) games.Player { return games.Player{Name: name, Color: "pink"} }

func TestWaitThenFillWithBots(t *testing.T) {
	g, worlds := newTestGame(4)
	r1, s1 := g.seatFor(player("alice"), func() {})
	r2, s2 := g.seatFor(player("bob"), func() {})
	if r1 != r2 || s1 == s2 {
		t.Fatal("players should share the waiting room")
	}

	now := time.Now()
	r1.tick(now, time.Millisecond)
	if r1.phase != Waiting {
		t.Fatal("room started before the wait was over")
	}
	r1.tick(r1.deadline, time.Millisecond)
	if r1.phase != Countdown || len(*worlds) != 1 {
		t.Fatalf("phase %v after the deadline", r1.phase)
	}
	w := (*worlds)[0]
	humans, bots := 0, 0
	for _, s := range w.seats {
		if s.Bot {
			bots++
		} else {
			humans++
		}
	}
	if humans != 2 || bots != 2 {
		t.Fatalf("humans=%d bots=%d", humans, bots)
	}
	// Both humans picked pink; they must end up with different colors.
	if w.seats[0].Color == w.seats[1].Color {
		t.Fatal("seat colors collide")
	}

	// A late player gets a new room instead of joining the started one.
	r3, _ := g.seatFor(player("carol"), func() {})
	if r3 == r1 || g.Rooms() != 2 {
		t.Fatal("late player joined a room that already started")
	}

	// Countdown, then play.
	r1.tick(r1.startedAt.Add(g.cfg.Countdown), time.Millisecond)
	if r1.phase != Playing {
		t.Fatalf("phase %v after the countdown", r1.phase)
	}
	r1.tick(time.Now(), 50*time.Millisecond)
	if w.steps != 1 {
		t.Fatal("world not stepped while playing")
	}

	// Leaving mid-game hands the seat to a bot.
	r1.leave(s1)
	if !w.bots[s1] {
		t.Fatal("left seat was not handed to a bot")
	}

	w.over = true
	r1.tick(time.Now(), 50*time.Millisecond)
	if r1.phase != Finished || len(r1.results) == 0 {
		t.Fatal("room didn't finish")
	}

	// Once the last human leaves, the room closes.
	r1.leave(s2)
	if r1.tick(time.Now(), time.Millisecond) {
		t.Fatal("empty room kept running")
	}
}

func TestFullRoomStartsImmediately(t *testing.T) {
	g, _ := newTestGame(2)
	r, _ := g.seatFor(player("a"), func() {})
	g.seatFor(player("b"), func() {})
	r.tick(time.Now(), time.Millisecond)
	if r.phase != Countdown {
		t.Fatalf("full room is %v", r.phase)
	}
}

func TestLeavingWhileWaitingFreesTheSeat(t *testing.T) {
	g, _ := newTestGame(2)
	r, s := g.seatFor(player("a"), func() {})
	g.seatFor(player("b"), func() {})
	r.leave(s)
	if r.seats[s] != nil {
		t.Fatal("seat not freed")
	}
	r2, s2 := g.seatFor(player("c"), func() {})
	if r2 != r || s2 != s {
		t.Fatal("freed seat not reused")
	}
}

func TestEveryoneReadyStartsEarly(t *testing.T) {
	g, worlds := newTestGame(4)
	r, a := g.seatFor(player("a"), func() {})
	_, b := g.seatFor(player("b"), func() {})

	r.toggleReady(a)
	r.tick(time.Now(), time.Millisecond)
	if r.phase != Waiting {
		t.Fatal("room started with only one of two players ready")
	}
	// Changing your mind counts.
	r.toggleReady(a)
	r.toggleReady(b)
	r.tick(time.Now(), time.Millisecond)
	if r.phase != Waiting {
		t.Fatal("room started after a player took back their ready")
	}

	r.toggleReady(a)
	r.tick(time.Now(), time.Millisecond)
	if r.phase != Countdown {
		t.Fatalf("room is %v with everyone ready", r.phase)
	}
	bots := 0
	for _, s := range (*worlds)[0].seats {
		if s.Bot {
			bots++
		}
	}
	if bots != 2 {
		t.Fatalf("%d bots, want 2 in the empty seats", bots)
	}
	// Ready can't be toggled once the room has started.
	r.toggleReady(a)
	if !r.seats[a].ready {
		t.Fatal("ready changed after the start")
	}
}

func TestNewcomerMustAlsoBeReady(t *testing.T) {
	g, _ := newTestGame(4)
	r, a := g.seatFor(player("a"), func() {})
	g.seatFor(player("b"), func() {})
	r.toggleReady(a)
	_, c := g.seatFor(player("c"), func() {})
	r.tick(time.Now(), time.Millisecond)
	if r.phase != Waiting {
		t.Fatal("room started without the newcomers being ready")
	}
	// If the only player who isn't ready leaves, everyone left is ready.
	r.leave(c)
	r.leave(1)
	r.tick(time.Now(), time.Millisecond)
	if r.phase != Countdown {
		t.Fatalf("room is %v after the unready players left", r.phase)
	}
}

func TestDefaultWaitIs30s(t *testing.T) {
	g, _ := newTestGame(2)
	if g.cfg.Wait != 30*time.Second {
		t.Fatalf("wait %v", g.cfg.Wait)
	}
}
