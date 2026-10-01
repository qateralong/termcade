package auth

import (
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func init() { cost = bcrypt.MinCost }

func TestHashAndCheck(t *testing.T) {
	h, err := Hash("hunter22")
	if err != nil {
		t.Fatal(err)
	}
	if !Check(h, "hunter22") || Check(h, "hunter23") || Check("", "hunter22") {
		t.Fatal("Check gave the wrong answer")
	}
	if strings.Contains(h, "hunter22") {
		t.Fatal("hash contains the password")
	}
}

func TestValidate(t *testing.T) {
	cases := map[[2]string]error{
		{"secret", "secret"}:   nil,
		{"short", "short"}:     ErrTooShort,
		{"secret1", "secret2"}: ErrMismatch,
		{strings.Repeat("x", 73), strings.Repeat("x", 73)}: ErrTooLong,
	}
	for in, want := range cases {
		if got := Validate(in[0], in[1]); got != want {
			t.Errorf("Validate(%q) = %v, want %v", in[0], got, want)
		}
	}
}

func TestLimiter(t *testing.T) {
	now := time.Unix(1000, 0)
	l := NewLimiter(3, time.Minute)
	l.now = func() time.Time { return now }

	for i := 0; i < 2; i++ {
		l.Fail("Alice")
	}
	if l.Allowed("alice") != nil {
		t.Fatal("locked out too early")
	}
	l.Fail("ALICE")
	if l.Allowed("alice") != ErrLockedOut {
		t.Fatal("should be locked out after 3 failures, whatever the case")
	}
	if l.Allowed("bob") != nil {
		t.Fatal("other accounts must not be affected")
	}
	now = now.Add(time.Minute)
	if l.Allowed("alice") != nil {
		t.Fatal("lockout should expire")
	}
	l.Fail("alice")
	l.Succeed("alice")
	l.Fail("alice")
	l.Fail("alice")
	if l.Allowed("alice") != nil {
		t.Fatal("success should reset the count")
	}
}
