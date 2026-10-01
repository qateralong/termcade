// Package auth handles passwords: hashing them, checking them, and slowing
// down anyone guessing.
package auth

import (
	"errors"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Password length limits. bcrypt only looks at the first 72 bytes.
const (
	MinPassword = 6
	MaxPassword = 72
)

var (
	ErrTooShort  = errors.New("password must be at least 6 characters")
	ErrTooLong   = errors.New("password must be at most 72 bytes")
	ErrMismatch  = errors.New("the passwords don't match")
	ErrWrong     = errors.New("wrong name or password")
	ErrLockedOut = errors.New("too many attempts, wait a minute and try again")
)

// cost is the bcrypt work factor; tests lower it to stay fast.
var cost = bcrypt.DefaultCost

// Validate checks a new password and its confirmation.
func Validate(password, confirm string) error {
	switch {
	case len(password) < MinPassword:
		return ErrTooShort
	case len(password) > MaxPassword:
		return ErrTooLong
	case password != confirm:
		return ErrMismatch
	}
	return nil
}

// Hash returns a bcrypt hash of password.
func Hash(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	return string(h), err
}

// dummyHash is compared against when an account doesn't exist or has no
// password, so a failed login takes as long either way and doesn't reveal
// which names are registered.
var dummyHash = sync.OnceValue(func() []byte {
	h, _ := bcrypt.GenerateFromPassword([]byte("termcade-dummy-password"), cost)
	return h
})

// Check reports whether password matches hash. An empty hash never matches.
func Check(hash, password string) bool {
	if hash == "" {
		bcrypt.CompareHashAndPassword(dummyHash(), []byte(password))
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// Limiter locks an account out for a while after too many wrong passwords.
// It is keyed by account name, so switching SSH connections doesn't help a
// guesser.
type Limiter struct {
	mu       sync.Mutex
	failures map[string]*record
	max      int
	lockout  time.Duration
	now      func() time.Time
}

type record struct {
	count int
	until time.Time
}

// NewLimiter allows max wrong attempts per account before a lockout.
func NewLimiter(max int, lockout time.Duration) *Limiter {
	return &Limiter{failures: map[string]*record{}, max: max, lockout: lockout, now: time.Now}
}

func key(name string) string { return strings.ToLower(name) }

// Allowed reports whether another attempt may be made for name.
func (l *Limiter) Allowed(name string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	r := l.failures[key(name)]
	if r != nil && l.now().Before(r.until) {
		return ErrLockedOut
	}
	return nil
}

// Fail records a wrong attempt.
func (l *Limiter) Fail(name string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	k := key(name)
	r := l.failures[k]
	if r == nil || (!r.until.IsZero() && !l.now().Before(r.until)) {
		r = &record{}
		l.failures[k] = r
	}
	r.count++
	if r.count >= l.max {
		r.until = l.now().Add(l.lockout)
		r.count = 0
	}
}

// Succeed clears the failures for name.
func (l *Limiter) Succeed(name string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, key(name))
}
