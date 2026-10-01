package server

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/charmbracelet/x/ansi"
	gossh "golang.org/x/crypto/ssh"

	"termcade/internal/app"
	"termcade/internal/games"
	"termcade/internal/games/lineup"
	"termcade/internal/hub"
	"termcade/internal/store"
)

// screen accumulates everything the server sends, minus escape sequences.
type screen struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *screen) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *screen) waitFor(t *testing.T, text string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		out := ansi.Strip(s.buf.String())
		s.mu.Unlock()
		if strings.Contains(out, text) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	s.mu.Lock()
	out := ansi.Strip(s.buf.String())
	s.mu.Unlock()
	if len(out) > 1500 {
		out = out[len(out)-1500:]
	}
	t.Fatalf("timed out waiting for %q; tail of output:\n%s", text, out)
}

func TestEndToEnd(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	deps := app.Deps{
		Store: st,
		Hub:   hub.New(),
		Games: games.NewRegistry(lineup.All()...),
		Log:   log.New(io.Discard),
	}
	srv, err := New(Config{Addr: "127.0.0.1:0", HostKeyPath: t.TempDir() + "/host_ed25519", IdleTimeout: time.Minute}, deps)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	addr, err := ListenAndServe(srv, "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := gossh.NewSignerFromKey(priv)
	client, err := gossh.Dial("tcp", addr.String(), &gossh.ClientConfig{
		User:            "trinity",
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := sess.RequestPty("xterm-256color", 30, 100, gossh.TerminalModes{}); err != nil {
		t.Fatal(err)
	}
	var out screen
	sess.Stdout = &out
	stdin, _ := sess.StdinPipe()
	if err := sess.Shell(); err != nil {
		t.Fatal(err)
	}

	stdin.Write([]byte("\r")) // skip the intro
	out.waitFor(t, "WELCOME, PLAYER")
	stdin.Write([]byte("1")) // create an account
	out.waitFor(t, "CREATE AN ACCOUNT")
	// The name "trinity" is suggested; add a password and confirm it.
	stdin.Write([]byte("\tmatrix99\tmatrix99\r"))
	out.waitFor(t, "LOBBY CHAT")
	out.waitFor(t, "trinity joined the arcade")

	stdin.Write([]byte("\thello from the test\r"))
	out.waitFor(t, "hello from the test")

	stdin.Write([]byte("\x1b")) // leave chat...
	// ...pause, or ESC followed by q would be read as alt+q...
	time.Sleep(400 * time.Millisecond)
	stdin.Write([]byte("q")) // ...then quit
	out.waitFor(t, "thanks for playing, trinity")

	p, err := st.PlayerByFingerprint(t.Context(), gossh.FingerprintSHA256(signer.PublicKey()))
	if err != nil || p.Name != "trinity" {
		t.Fatalf("player not stored: %+v, %v", p, err)
	}
}
