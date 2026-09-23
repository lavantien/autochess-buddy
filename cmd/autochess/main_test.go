package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/autochess-buddy/internal/store/sqlite"
)

func quietLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRunSeed_RoundTrip(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "app.db")
	if err := runSeed(dbPath); err != nil {
		t.Fatalf("seed: %v", err)
	}
	st, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = st.Close() }()
	var matches int
	if err := st.DB.QueryRow("SELECT COUNT(*) FROM matches").Scan(&matches); err != nil {
		t.Fatalf("count matches: %v", err)
	}
	if matches == 0 {
		t.Fatal("seeded db holds no matches")
	}
}

func TestServe_BootsServesAndDrains(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := lis.Addr().String()
	dbPath := filepath.Join(t.TempDir(), "app.db")

	errCh := make(chan error, 1)
	go func() { errCh <- serve(ctx, lis, dbPath, quietLog()) }()

	var resp *http.Response
	deadline := time.Now().Add(15 * time.Second)
	for {
		r, err := http.Get("http://" + addr + "/dashboard")
		if err == nil {
			resp = r
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server never answered /dashboard: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dashboard status: %d", resp.StatusCode)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("serve: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("serve did not drain after ctx cancel")
	}
}

func TestServe_ReturnsListenError(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = lis.Close() }()

	err = run(context.Background(), lis.Addr().String(),
		filepath.Join(t.TempDir(), "app.db"), quietLog())
	if err == nil {
		t.Fatal("want error when the addr is already bound")
	}
	if errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("want a real listen error, got %v", err)
	}
	if !strings.Contains(err.Error(), "listen") {
		t.Fatalf("err = %v, want the listen wrap", err)
	}
}

// fileAsParent builds a db path whose parent is a regular file, so MkdirAll
// fails before any engine opens.
func fileAsParent(t *testing.T) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(f, "app.db")
}

func TestRunSeed_BadDirFails(t *testing.T) {
	if err := runSeed(fileAsParent(t)); err == nil {
		t.Fatal("want mkdir error, got nil")
	}
}

func TestRunSeed_DirectoryAsDbFails(t *testing.T) {
	if err := runSeed(t.TempDir()); err == nil {
		t.Fatal("want open error on a directory db path, got nil")
	}
}

func TestRunSeed_SecondSeedRefused(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "app.db")
	if err := runSeed(dbPath); err != nil {
		t.Fatalf("first seed: %v", err)
	}
	if err := runSeed(dbPath); err == nil {
		t.Fatal("want refusal seeding a non-empty db, got nil")
	}
}

func TestRun_BadDirFails(t *testing.T) {
	if err := run(context.Background(), "127.0.0.1:0", fileAsParent(t), quietLog()); err == nil {
		t.Fatal("want mkdir error, got nil")
	}
}

func TestRun_DirectoryAsDbFails(t *testing.T) {
	err := run(context.Background(), "127.0.0.1:0", t.TempDir(), quietLog())
	if err == nil || !strings.Contains(err.Error(), "open sqlite") {
		t.Fatalf("err = %v, want open sqlite failure", err)
	}
}

// A busy addr must fail at listen before either engine boots, so the error
// names the listen step and never reaches sqlite.
func TestRun_BoundAddrFailsBeforeEngines(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = lis.Close() }()

	err = run(context.Background(), lis.Addr().String(), t.TempDir(), quietLog())
	if err == nil {
		t.Fatal("want error when the addr is already bound")
	}
	if !strings.Contains(err.Error(), "listen") {
		t.Fatalf("err = %v, want the listen wrap", err)
	}
	if strings.Contains(err.Error(), "open sqlite") {
		t.Fatalf("err = %v, engines opened before the listen failure", err)
	}
}

func TestServe_QuoteInDbPathBoots(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- serve(ctx, lis, filepath.Join(t.TempDir(), "qu'ote.db"), quietLog())
	}()

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("serve: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("serve did not drain after ctx cancel")
	}
}
