// Package hub tracks who is connected and where they are, and runs the lobby
// chat. Every connected session joins the hub and receives updates as
// messages that can be fed straight into its Bubble Tea program.
package hub

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"termcade/internal/textutil"
)

const (
	// LocationLobby is where members are when they are not in a game.
	LocationLobby = "lobby"

	// MaxMessageLen is the maximum length of a chat message, in runes.
	MaxMessageLen = 200

	historySize = 100
	outboxSize  = 64
)

var (
	ErrEmptyMessage = errors.New("message is empty")
	ErrRateLimited  = errors.New("slow down a little")
	ErrUnknown      = errors.New("unknown session")
)

// Member is a connected session as seen by others.
type Member struct {
	SessionID string
	Name      string
	Color     string
	Guest     bool
	Location  string
	JoinedAt  time.Time
}

// ChatMessage is a line in the lobby chat. System messages have no author.
type ChatMessage struct {
	ID     uint64
	Time   time.Time
	From   string
	Color  string
	Text   string
	System bool
}

// PresenceMsg carries a fresh snapshot of everyone online, sorted by name.
type PresenceMsg struct{ Members []Member }

// ChatMsg carries a single new chat message.
type ChatMsg struct{ Message ChatMessage }

type client struct {
	member  Member
	outbox  chan any
	done    chan struct{}
	limiter *rate.Limiter
}

// Hub is safe for concurrent use.
type Hub struct {
	mu      sync.Mutex
	clients map[string]*client
	history []ChatMessage
	nextID  uint64
	now     func() time.Time
}

// New returns an empty hub.
func New() *Hub {
	return &Hub{clients: make(map[string]*client), now: time.Now}
}

// Join registers a session. Updates are passed to deliver from a dedicated
// goroutine, so a slow client never blocks the hub; if a client falls too far
// behind, updates for it are dropped. Join returns the recent chat history
// and a function that must be called when the session ends.
func (h *Hub) Join(m Member, deliver func(any)) (history []ChatMessage, leave func()) {
	c := &client{
		outbox:  make(chan any, outboxSize),
		done:    make(chan struct{}),
		limiter: rate.NewLimiter(rate.Every(time.Second), 4),
	}
	go func() {
		for {
			select {
			case msg := <-c.outbox:
				deliver(msg)
			case <-c.done:
				return
			}
		}
	}()

	h.mu.Lock()
	m.JoinedAt = h.now()
	if m.Location == "" {
		m.Location = LocationLobby
	}
	c.member = m
	h.clients[m.SessionID] = c
	history = append([]ChatMessage(nil), h.history...)
	if !h.nameOnlineLocked(m.Name, m.SessionID) {
		h.systemLocked(m.Name + " joined the arcade")
	}
	h.broadcastPresenceLocked()
	h.mu.Unlock()

	var once sync.Once
	return history, func() { once.Do(func() { h.leave(m.SessionID) }) }
}

func (h *Hub) leave(sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	c, ok := h.clients[sessionID]
	if !ok {
		return
	}
	delete(h.clients, sessionID)
	close(c.done)
	if !h.nameOnlineLocked(c.member.Name, "") {
		h.systemLocked(c.member.Name + " left")
	}
	h.broadcastPresenceLocked()
}

// SetLocation records that a session moved to the lobby or into a game.
func (h *Hub) SetLocation(sessionID, location string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if c, ok := h.clients[sessionID]; ok && c.member.Location != location {
		c.member.Location = location
		h.broadcastPresenceLocked()
	}
}

// Rename updates a session's display name and color.
func (h *Hub) Rename(sessionID, name, color string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	c, ok := h.clients[sessionID]
	if !ok {
		return
	}
	old := c.member.Name
	c.member.Name, c.member.Color = name, color
	if !strings.EqualFold(old, name) {
		h.systemLocked(old + " is now known as " + name)
	}
	h.broadcastPresenceLocked()
}

// Say posts a chat message from the given session.
func (h *Hub) Say(sessionID, text string) error {
	text = textutil.SanitizeLine(text, MaxMessageLen)
	if text == "" {
		return ErrEmptyMessage
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	c, ok := h.clients[sessionID]
	if !ok {
		return ErrUnknown
	}
	if !c.limiter.AllowN(h.now(), 1) {
		return ErrRateLimited
	}
	h.postLocked(ChatMessage{From: c.member.Name, Color: c.member.Color, Text: text})
	return nil
}

// NameOnline reports whether a session other than exceptSession is using
// name (case-insensitively).
func (h *Hub) NameOnline(name, exceptSession string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.nameOnlineLocked(name, exceptSession)
}

// Members returns a snapshot of everyone online, sorted by name.
func (h *Hub) Members() []Member {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.membersLocked()
}

func (h *Hub) nameOnlineLocked(name, exceptSession string) bool {
	for id, c := range h.clients {
		if id != exceptSession && strings.EqualFold(c.member.Name, name) {
			return true
		}
	}
	return false
}

func (h *Hub) membersLocked() []Member {
	out := make([]Member, 0, len(h.clients))
	for _, c := range h.clients {
		out = append(out, c.member)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name)
		if a != b {
			return a < b
		}
		return out[i].SessionID < out[j].SessionID
	})
	return out
}

func (h *Hub) systemLocked(text string) {
	h.postLocked(ChatMessage{Text: text, System: true})
}

func (h *Hub) postLocked(m ChatMessage) {
	h.nextID++
	m.ID = h.nextID
	m.Time = h.now()
	h.history = append(h.history, m)
	if len(h.history) > historySize {
		h.history = append(h.history[:0:0], h.history[len(h.history)-historySize:]...)
	}
	h.sendAllLocked(ChatMsg{Message: m})
}

func (h *Hub) broadcastPresenceLocked() {
	h.sendAllLocked(PresenceMsg{Members: h.membersLocked()})
}

func (h *Hub) sendAllLocked(msg any) {
	for _, c := range h.clients {
		select {
		case c.outbox <- msg:
		default: // client is too slow; drop rather than stall everyone
		}
	}
}
