package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/lavantien/autochess-buddy/internal/store/sqlite"
)

func quietLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// freeAddr reserves an ephemeral port and hands the address back for reuse.
func freeAddr(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := lis.Addr().String()
	if err := lis.Close(); err != nil {
		t.Fatalf("release probe port: %v", err)
	}
	return addr
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
	addr := freeAddr(t)
	dbPath := filepath.Join(t.TempDir(), "app.db")

	errCh := make(chan error, 1)
	go func() { errCh <- run(ctx, addr, dbPath, quietLog()) }()

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
}
