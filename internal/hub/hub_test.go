package hub

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// recorder collects everything delivered to a client.
type recorder struct {
	mu   sync.Mutex
	msgs []any
}

func (r *recorder) deliver(m any) {
	r.mu.Lock()
	r.msgs = append(r.msgs, m)
	r.mu.Unlock()
}

// waitFor polls until pred matches a delivered message or times out.
func (r *recorder) waitFor(t *testing.T, pred func(any) bool) any {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		for _, m := range r.msgs {
			if pred(m) {
				r.mu.Unlock()
				return m
			}
		}
		r.mu.Unlock()
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for message")
	return nil
}

func chatWith(text string) func(any) bool {
	return func(m any) bool {
		c, ok := m.(ChatMsg)
		return ok && c.Message.Text == text
	}
}

func TestJoinChatLeave(t *testing.T) {
	h := New()
	var a, b recorder

	_, leaveA := h.Join(Member{SessionID: "1", Name: "alice", Color: "pink"}, a.deliver)
	history, leaveB := h.Join(Member{SessionID: "2", Name: "bob", Color: "cyan"}, b.deliver)

	if len(history) != 1 || history[0].Text != "alice joined the arcade" {
		t.Fatalf("unexpected history: %+v", history)
	}
	a.waitFor(t, chatWith("bob joined the arcade"))

	p := a.waitFor(t, func(m any) bool {
		p, ok := m.(PresenceMsg)
		return ok && len(p.Members) == 2
	}).(PresenceMsg)
	if p.Members[0].Name != "alice" || p.Members[1].Location != LocationLobby {
		t.Fatalf("unexpected presence: %+v", p.Members)
	}

	if err := h.Say("2", "  hi \x1b[31mthere  "); err != nil {
		t.Fatal(err)
	}
	msg := a.waitFor(t, chatWith("hi there")).(ChatMsg).Message
	if msg.From != "bob" || msg.Color != "cyan" || msg.System {
		t.Fatalf("unexpected message: %+v", msg)
	}

	h.SetLocation("1", "tron")
	b.waitFor(t, func(m any) bool {
		p, ok := m.(PresenceMsg)
		return ok && len(p.Members) == 2 && p.Members[0].Location == "tron"
	})

	leaveA()
	leaveA() // idempotent
	b.waitFor(t, chatWith("alice left"))
	if got := len(h.Members()); got != 1 {
		t.Fatalf("members after leave = %d", got)
	}
	leaveB()
}

func TestSayValidation(t *testing.T) {
	h := New()
	var r recorder
	_, leave := h.Join(Member{SessionID: "1", Name: "alice"}, r.deliver)
	defer leave()

	if err := h.Say("1", " \t "); !errors.Is(err, ErrEmptyMessage) {
		t.Fatalf("expected ErrEmptyMessage, got %v", err)
	}
	if err := h.Say("nope", "hi"); !errors.Is(err, ErrUnknown) {
		t.Fatalf("expected ErrUnknown, got %v", err)
	}

	var limited bool
	for i := 0; i < 10; i++ {
		if errors.Is(h.Say("1", "spam"), ErrRateLimited) {
			limited = true
			break
		}
	}
	if !limited {
		t.Fatal("expected rate limiting to kick in")
	}
}

func TestDuplicateSessionsShareName(t *testing.T) {
	h := New()
	var r recorder
	_, leave1 := h.Join(Member{SessionID: "1", Name: "alice"}, r.deliver)
	_, leave2 := h.Join(Member{SessionID: "2", Name: "alice"}, r.deliver)

	if !h.NameOnline("ALICE", "") || h.NameOnline("alice", "1") != true {
		t.Fatal("NameOnline should see the other session")
	}
	leave1()
	// alice is still online through session 2, so no "left" message.
	time.Sleep(20 * time.Millisecond)
	r.mu.Lock()
	for _, m := range r.msgs {
		if c, ok := m.(ChatMsg); ok && c.Message.Text == "alice left" {
			t.Fatal("unexpected left message while another session is online")
		}
	}
	r.mu.Unlock()
	leave2()
}

func TestRename(t *testing.T) {
	h := New()
	var r recorder
	_, leave := h.Join(Member{SessionID: "1", Name: "alice", Color: "pink"}, r.deliver)
	defer leave()
	h.Rename("1", "alicia", "lime")
	r.waitFor(t, chatWith("alice is now known as alicia"))
	if m := h.Members()[0]; m.Name != "alicia" || m.Color != "lime" {
		t.Fatalf("unexpected member: %+v", m)
	}
}
