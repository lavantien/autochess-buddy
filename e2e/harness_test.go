//go:build e2e

package e2e

import (
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/lavantien/autochess-buddy/internal/analytics"
	"github.com/lavantien/autochess-buddy/internal/httpapi"
	"github.com/lavantien/autochess-buddy/internal/seed"
	"github.com/lavantien/autochess-buddy/internal/service"
	"github.com/lavantien/autochess-buddy/internal/store/sqlite"
	"github.com/mxschmitt/playwright-go"
)

// newApp boots the full stack (sqlite + duckdb + http) over a fresh seeded db on
// an ephemeral port and returns the base url plus the store.
func newApp(t *testing.T, seeded bool) (string, *sqlite.Store) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "e2e.db")
	st, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if seeded {
		if err := seed.Load(st.DB); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	eng, err := analytics.New(dbPath, &st.WriteMu)
	if err != nil {
		t.Fatalf("analytics (first run needs network for the duckdb sqlite extension): %v", err)
	}
	t.Cleanup(func() { eng.Close() })

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := &http.Server{Handler: httpapi.New(nil, st, service.EntryService{St: st}, eng)}
	go srv.Serve(lis)
	t.Cleanup(func() { srv.Close() })
	return fmt.Sprintf("http://%s", lis.Addr().String()), st
}

// newPage opens one auto-dialog-accepting chromium page.
func newPage(t *testing.T, base string) playwright.Page {
	t.Helper()
	pw, err := playwright.Run()
	if err != nil {
		t.Fatalf("run playwright: %v", err)
	}
	t.Cleanup(func() { pw.Stop() })
	browser, err := pw.Chromium.Launch()
	if err != nil {
		t.Fatalf("launch chromium: %v", err)
	}
	t.Cleanup(func() { browser.Close() })
	page, err := browser.NewPage()
	if err != nil {
		t.Fatalf("new page: %v", err)
	}
	page.On("dialog", func(d playwright.Dialog) { d.Accept() })
	return page
}

// must swallows the locator-error dance the flows repeat constantly.
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// wantText fails unless the locator resolves to exactly want.
func wantText(t *testing.T, loc playwright.Locator, want string) {
	t.Helper()
	got, err := loc.InnerText()
	if err != nil {
		t.Fatalf("locator: %v", err)
	}
	if got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
}

// gone fails unless the locator matches nothing.
func gone(t *testing.T, loc playwright.Locator) {
	t.Helper()
	n, err := loc.Count()
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("locator still matches %d nodes", n)
	}
}

// selectLabel picks one select option by its visible label.
func selectLabel(t *testing.T, loc playwright.Locator, label string) {
	t.Helper()
	if _, err := loc.SelectOption(playwright.SelectOptionValues{Labels: &[]string{label}}); err != nil {
		t.Fatalf("select %q: %v", label, err)
	}
}

// selectValue picks one select option by its value attribute.
func selectValue(t *testing.T, loc playwright.Locator, v string) {
	t.Helper()
	if _, err := loc.SelectOption(playwright.SelectOptionValues{Values: &[]string{v}}); err != nil {
		t.Fatalf("select value %q: %v", v, err)
	}
}

// waitFor waits until the selector matches anything in the live dom.
func waitFor(t *testing.T, page playwright.Page, selector string) {
	t.Helper()
	if err := page.Locator(selector).First().WaitFor(); err != nil {
		t.Fatalf("wait for %s: %v", selector, err)
	}
}

// waitText waits until the selector's text equals want, riding out htmx swaps.
func waitText(t *testing.T, page playwright.Page, selector, want string) {
	t.Helper()
	loc := page.Locator(selector, playwright.PageLocatorOptions{HasText: playwright.String(want)}).First()
	if err := loc.WaitFor(); err != nil {
		t.Fatalf("wait %q on %s: %v", want, selector, err)
	}
}
