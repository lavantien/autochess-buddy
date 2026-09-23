package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/lavantien/autochess-buddy/internal/analytics"
	"github.com/lavantien/autochess-buddy/internal/httpapi"
	"github.com/lavantien/autochess-buddy/internal/seed"
	"github.com/lavantien/autochess-buddy/internal/service"
	"github.com/lavantien/autochess-buddy/internal/store/sqlite"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "listen address")
	dbPath := flag.String("db", "data/app.db", "sqlite database file")
	doSeed := flag.Bool("seed", false, "load the demo fixture into an empty database, then exit")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(log)

	if *doSeed {
		if err := runSeed(*dbPath); err != nil {
			log.Error("seed", "err", err)
			os.Exit(1)
		}
		return
	}

	if err := run(*addr, *dbPath, log); err != nil {
		log.Error("serve", "err", err)
		os.Exit(1)
	}
}

// runSeed loads the fixture, refusing a database that already holds matches.
func runSeed(dbPath string) error {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return err
	}
	st, err := sqlite.Open(dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()
	if err := seed.Load(st.DB); err != nil {
		return err
	}
	fmt.Println("seeded", dbPath)
	return nil
}

// run wires both engines and drains http first, then duckdb, then sqlite
// (readme:211).
func run(addr, dbPath string, log *slog.Logger) error {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return err
	}
	st, err := sqlite.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}
	defer func() { _ = st.Close() }()

	eng, err := analytics.New(dbPath, &st.WriteMu)
	if err != nil {
		return fmt.Errorf("open duckdb: %w", err)
	}
	defer func() { _ = eng.Close() }()
	var dash analytics.Service = eng

	srv := &http.Server{
		Addr:              addr,
		Handler:           httpapi.New(log, st, service.EntryService{St: st}, dash),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("drain http: %w", err)
	}
	return nil
}
