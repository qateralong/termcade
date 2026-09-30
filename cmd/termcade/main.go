// Command termcade runs the arcade SSH server.
package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"

	"termcade/internal/app"
	"termcade/internal/games"
	"termcade/internal/hub"
	"termcade/internal/server"
	"termcade/internal/store"
)

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func main() {
	var (
		addr    = flag.String("addr", env("TERMCADE_ADDR", ":2222"), "SSH listen address (env TERMCADE_ADDR)")
		hostKey = flag.String("host-key", env("TERMCADE_HOST_KEY", "data/host_ed25519"), "host key path, created if missing (env TERMCADE_HOST_KEY)")
		dbPath  = flag.String("db", env("TERMCADE_DB", "data/termcade.db"), "SQLite database path (env TERMCADE_DB)")
		debug   = flag.Bool("debug", os.Getenv("TERMCADE_DEBUG") != "", "verbose logging (env TERMCADE_DEBUG)")
	)
	flag.Parse()

	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: true,
		TimeFormat:      time.DateTime,
		Prefix:          "termcade",
	})
	if *debug {
		logger.SetLevel(log.DebugLevel)
	}

	if err := run(logger, *addr, *hostKey, *dbPath); err != nil {
		logger.Fatal("server stopped", "err", err)
	}
}

func run(logger *log.Logger, addr, hostKey, dbPath string) error {
	st, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer st.Close()

	deps := app.Deps{
		Store: st,
		Hub:   hub.New(),
		Games: games.NewRegistry(games.Catalog()...),
		Log:   logger,
	}

	srv, err := server.New(server.Config{
		Addr:        addr,
		HostKeyPath: hostKey,
		IdleTimeout: time.Hour,
	}, deps)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 1)
	go func() {
		players, _ := st.CountPlayers(context.Background())
		logger.Info("arcade is open", "addr", addr, "players", players, "games", len(deps.Games.All()))
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		if !errors.Is(err, ssh.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		return err
	}
	return nil
}
