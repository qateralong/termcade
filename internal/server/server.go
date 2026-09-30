// Package server wires the SSH server to the per-session arcade app.
package server

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	bm "github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
	"github.com/charmbracelet/wish/ratelimiter"
	"github.com/muesli/termenv"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/time/rate"

	"termcade/internal/app"
	"termcade/internal/ui/theme"
)

// Config holds the SSH server settings.
type Config struct {
	Addr        string        // listen address, e.g. ":2222"
	HostKeyPath string        // generated on first start if missing
	IdleTimeout time.Duration // disconnect sessions with no traffic
}

type ctxKey struct{ name string }

var (
	appKey   = &ctxKey{"app"}
	themeKey = &ctxKey{"theme"}
)

// New returns a configured, not yet listening, SSH server.
func New(cfg Config, deps app.Deps) (*ssh.Server, error) {
	if err := os.MkdirAll(filepath.Dir(cfg.HostKeyPath), 0o700); err != nil {
		return nil, fmt.Errorf("create host key dir: %w", err)
	}
	// Allow a short burst of connections per IP, then one per second.
	limiter := ratelimiter.NewRateLimiter(rate.Every(time.Second), 10, 4096)

	return wish.NewServer(
		wish.WithAddress(cfg.Addr),
		wish.WithHostKeyPath(cfg.HostKeyPath),
		wish.WithIdleTimeout(cfg.IdleTimeout),
		// Anyone may connect. A public key, if offered, becomes the player's
		// identity; without one they play as a guest.
		wish.WithPublicKeyAuth(func(ssh.Context, ssh.PublicKey) bool { return true }),
		wish.WithKeyboardInteractiveAuth(func(ssh.Context, gossh.KeyboardInteractiveChallenge) bool { return true }),
		wish.WithMiddleware(
			// Listed innermost first: logging runs first, farewell runs last.
			farewell(deps.Log),
			bm.MiddlewareWithProgramHandler(programHandler(deps), termenv.ANSI256),
			activeterm.Middleware(),
			ratelimiter.Middleware(limiter),
			logging.StructuredMiddlewareWithLogger(deps.Log, log.InfoLevel),
		),
	)
}

// programHandler builds the Bubble Tea program for a new session.
func programHandler(deps app.Deps) bm.ProgramHandler {
	return func(sess ssh.Session) *tea.Program {
		id := app.Identity{
			SessionID: sess.Context().SessionID(),
			User:      sess.User(),
		}
		if key := sess.PublicKey(); key != nil {
			id.Fingerprint = gossh.FingerprintSHA256(key)
		}

		// The app needs a way to push hub updates into its own program, which
		// doesn't exist yet while the app is being built.
		var program *tea.Program
		ready := make(chan struct{})
		send := func(msg tea.Msg) {
			<-ready
			program.Send(msg)
		}

		th := theme.New(newRenderer(sess))
		sess.Context().SetValue(themeKey, th)
		a := app.New(deps, th, id, send)
		sess.Context().SetValue(appKey, a)

		opts := append(bm.MakeOptions(sess), tea.WithAltScreen())
		program = tea.NewProgram(a, opts...)
		close(ready)
		return program
	}
}

// farewell runs after the program exits: it releases the session's hub
// membership and prints a goodbye on the normal screen.
func farewell(logger *log.Logger) wish.Middleware {
	return func(next ssh.Handler) ssh.Handler {
		return func(sess ssh.Session) {
			if a, ok := sess.Context().Value(appKey).(*app.App); ok {
				a.Close()
				th, _ := sess.Context().Value(themeKey).(*theme.Theme)
				if th == nil {
					th = theme.New(newRenderer(sess))
				}
				logo := th.Gradient("◆ "+theme.Name, theme.LogoGradient, 12, 0, true)
				switch name := a.Name(); {
				case a.ShuttingDown():
					wish.Println(sess, "\n  "+logo+
						th.Dim.Render("  the arcade is restarting, reconnect in a few seconds.")+"\n")
				case name != "":
					wish.Println(sess, "\n  "+logo+
						th.Dim.Render("  thanks for playing, ")+th.PlayerName(name, a.Color())+
						th.Dim.Render("! see you soon.")+"\n")
				}
			}
			next(sess)
		}
	}
}

// ListenAndServe is a small helper for tests: it listens on addr and returns
// the actual address (useful with port 0).
func ListenAndServe(srv *ssh.Server, addr string) (net.Addr, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	go srv.Serve(ln)
	return ln.Addr(), nil
}

// newRenderer builds a lipgloss renderer for a session from its TERM and
// environment.
//
// wish's bubbletea.MakeRenderer is deliberately not used: on Unix it asks the
// client terminal for its background color and blocks until an answer or a
// keypress arrives, swallowing that keypress. Clients that don't answer
// would hang on connect and on disconnect. Instead we assume a dark
// background, which the neon theme is designed for anyway.
func newRenderer(sess ssh.Session) *lipgloss.Renderer {
	pty, _, _ := sess.Pty()
	env := sshEnviron(append(sess.Environ(), "TERM="+pty.Term))
	r := lipgloss.NewRenderer(sess,
		termenv.WithEnvironment(env), termenv.WithUnsafe(), termenv.WithColorCache(true))
	// Most SSH clients don't forward COLORTERM, so a capable terminal often
	// looks like a basic one. 256 colors are supported practically everywhere.
	if pty.Term != "dumb" && r.ColorProfile() > termenv.ANSI256 {
		r.SetColorProfile(termenv.ANSI256)
	}
	r.SetHasDarkBackground(true)
	return r
}

// sshEnviron exposes a session's environment to termenv.
type sshEnviron []string

func (e sshEnviron) Environ() []string { return e }

func (e sshEnviron) Getenv(key string) string {
	for _, kv := range e {
		if v, ok := strings.CutPrefix(kv, key+"="); ok {
			return v
		}
	}
	return ""
}
