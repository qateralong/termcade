package bossrush

import (
	"io"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"termcade/internal/games/solo"
	"termcade/internal/ui/theme"
)

func newTest(seed uint64) *Engine {
	return New(theme.New(lipgloss.NewRenderer(io.Discard)), rand.New(rand.NewPCG(seed, seed)))
}

func TestStrikeTiming(t *testing.T) {
	e := newTest(1)
	e.advance() // past the intro
	e.Key("enter")
	if e.phase != strike {
		t.Fatal("FIGHT should open the timing bar")
	}
	e.barPos = 0.5
	e.resolveStrike()
	if e.lastHit != maxHit*3/2 || e.lastNote != "CRITICAL!" {
		t.Fatalf("perfect strike hit %d (%q)", e.lastHit, e.lastNote)
	}
	if e.phase != dodge {
		t.Fatal("the boss should attack after your strike")
	}

	e2 := newTest(2)
	e2.advance()
	e2.Key("enter")
	e2.barPos = 0.02
	e2.resolveStrike()
	if e2.lastHit != 0 || e2.lastNote != "MISS" {
		t.Fatalf("edge strike hit %d", e2.lastHit)
	}
}

func TestHealing(t *testing.T) {
	e := newTest(3)
	e.advance()
	e.hp = 5
	e.Key("right") // HEAL
	e.Key("enter")
	if e.hp != 15 || e.items != startItems-1 || e.phase != dodge {
		t.Fatalf("hp %d items %d phase %v", e.hp, e.items, e.phase)
	}
}

func TestGettingHit(t *testing.T) {
	e := newTest(4)
	e.advance()
	e.startAttack()
	e.att.next = time.Hour // no new bullets
	e.bullets = []*bullet{{x: e.hx, y: e.hy, w: 1, h: 1}}
	e.Update(25 * time.Millisecond)
	if e.hp != maxHP-e.boss().damage {
		t.Fatalf("hp %d", e.hp)
	}
	e.Update(25 * time.Millisecond) // still touching, but invulnerable
	if e.hp != maxHP-e.boss().damage {
		t.Fatal("hit twice during invulnerability")
	}
}

func TestEveryAttackRuns(t *testing.T) {
	for k := boneWall; k <= ring; k++ {
		e := newTest(uint64(k) + 10)
		e.advance()
		e.startAttack()
		e.att.kind = k
		e.blue = k == boneHop
		e.hp = 1 << 20 // can't die here
		spawned := 0
		for e.phase == dodge {
			e.Update(25 * time.Millisecond)
			if n := len(e.bullets) + len(e.beams); n > spawned {
				spawned = n
			}
			if e.hx < 0 || e.hx > BoxW-1 || e.hy < 0 || e.hy > BoxH-1 {
				t.Fatalf("attack %d: heart left the box", k)
			}
		}
		if spawned == 0 {
			t.Errorf("attack %d spawned nothing", k)
		}
		if e.phase != menu {
			t.Fatalf("attack %d didn't hand the turn back", k)
		}
	}
}

func TestBeatingEveryBossWins(t *testing.T) {
	e := newTest(5)
	for e.state == solo.Playing {
		switch e.phase {
		case intro, beaten:
			e.advance()
		case menu:
			e.menuSel = 0
			e.Key("enter")
		case strike:
			e.barPos = 0.5
			e.resolveStrike()
		case dodge:
			e.att.elapsed = e.att.duration // skip the attack
			e.Update(25 * time.Millisecond)
		}
	}
	if e.state != solo.Won || e.stage != len(bosses)-1 {
		t.Fatalf("state %v at boss %d", e.state, e.stage)
	}
}

func TestBlueHeartJumps(t *testing.T) {
	e := newTest(6)
	e.advance()
	e.startAttack()
	e.att.kind, e.blue = boneHop, true
	e.bullets, e.att.next = nil, time.Hour
	e.hy = BoxH - 1
	e.Update(25 * time.Millisecond)
	e.Key("up")
	e.Update(100 * time.Millisecond)
	if e.hy >= BoxH-1 {
		t.Fatal("jump didn't lift the heart")
	}
	for i := 0; i < 80; i++ {
		e.Update(25 * time.Millisecond)
	}
	if e.hy != BoxH-1 || !e.onFloor {
		t.Fatal("heart should land again")
	}
}

func TestViewFits(t *testing.T) {
	e := newTest(7)
	check := func(label string) {
		t.Helper()
		v := e.View()
		if w, h := lipgloss.Width(v), lipgloss.Height(v); w != ViewW || h != ViewH {
			t.Fatalf("%s: view is %dx%d", label, w, h)
		}
	}
	check("intro")
	e.advance()
	check("menu")
	e.Key("enter")
	check("strike")
	e.resolveStrike()
	for i := 0; i < 40; i++ {
		e.Update(25 * time.Millisecond)
	}
	check("dodge")
	for i := range bosses {
		e.meet(i)
		check(bosses[i].name)
	}
}
