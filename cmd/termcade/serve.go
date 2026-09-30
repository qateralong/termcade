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
	"termcade/internal/version"
)

func serveCmd(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	var (
		addr    = fs.String("addr", env("TERMCADE_ADDR", ":2222"), "SSH listen address (env TERMCADE_ADDR)")
		hostKey = fs.String("host-key", env("TERMCADE_HOST_KEY", "data/host_ed25519"), "host key path, created if missing (env TERMCADE_HOST_KEY)")
		dbPath  = fs.String("db", env("TERMCADE_DB", "data/termcade.db"), "SQLite database path (env TERMCADE_DB)")
		debug   = fs.Bool("debug", os.Getenv("TERMCADE_DEBUG") != "", "verbose logging (env TERMCADE_DEBUG)")
	)
	fs.Parse(args)

	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: true,
		TimeFormat:      time.DateTime,
		Prefix:          "termcade",
	})
	if *debug {
		logger.SetLevel(log.DebugLevel)
	}
	return serve(logger, *addr, *hostKey, *dbPath)
}

func serve(logger *log.Logger, addr, hostKey, dbPath string) error {
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
		logger.Info("arcade is open", "version", version.String(), "addr", addr,
			"players", players, "games", len(deps.Games.All()))
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

	// Tell everyone before cutting them off, and give their sessions a
	// moment to show the notice and disconnect on their own.
	online := deps.Hub.Count()
	logger.Info("shutting down", "online", online)
	if online > 0 {
		deps.Hub.Shutdown("The arcade is restarting")
		deadline := time.Now().Add(5 * time.Second)
		for deps.Hub.Count() > 0 && time.Now().Before(deadline) {
			time.Sleep(100 * time.Millisecond)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		logger.Warn("forcing remaining connections closed", "err", err)
		srv.Close()
	}
	return nil
}
