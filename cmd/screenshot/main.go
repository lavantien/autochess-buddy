// screenshot boots the seeded app on an ephemeral port, photographs the
// dashboard, and refreshes the marked image block at the top of readme.md.
// Run it (make shot) before publishing any release so the readme always
// shows the shipped ui.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lavantien/autochess-buddy/internal/analytics"
	"github.com/lavantien/autochess-buddy/internal/httpapi"
	"github.com/lavantien/autochess-buddy/internal/seed"
	"github.com/lavantien/autochess-buddy/internal/service"
	"github.com/lavantien/autochess-buddy/internal/store/sqlite"
	"github.com/mxschmitt/playwright-go"
)

func main() {
	out := flag.String("out", "docs/screenshots/dashboard.png", "screenshot output path")
	flag.Parse()
	if err := shoot(*out); err != nil {
		log.Fatal("screenshot: ", err)
	}
}

func shoot(out string) error {
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	dbPath := filepath.Join(os.TempDir(), fmt.Sprintf("acshot-%d.db", time.Now().UnixNano()))
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	base, stop, err := bootSeededApp(lis, dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = stop() }()

	pw, err := playwright.Run()
	if err != nil {
		return err
	}
	defer func() { _ = pw.Stop() }()
	browser, err := pw.Chromium.Launch()
	if err != nil {
		return err
	}
	defer func() { _ = browser.Close() }()
	page, err := browser.NewPage(playwright.BrowserNewPageOptions{
		Viewport: &playwright.Size{Width: 1280, Height: 800},
	})
	if err != nil {
		return err
	}
	if _, err := page.Goto(base + "/dashboard"); err != nil {
		return err
	}
	// .viewn only renders once live analytics answer; fonts.ready settles
	// the local woff2 faces before the shot.
	if err := page.Locator(".viewn").First().WaitFor(); err != nil {
		return fmt.Errorf("wait for analytics: %w", err)
	}
	if _, err := page.Evaluate("document.fonts.ready.then(() => true)"); err != nil {
		return fmt.Errorf("wait for fonts: %w", err)
	}
	if _, err := page.Screenshot(playwright.PageScreenshotOptions{
		Path:     playwright.String(out),
		FullPage: playwright.Bool(true),
	}); err != nil {
		return err
	}
	return refreshReadme("readme.md", filepath.ToSlash(out))
}

// newShotServer matches the production server's header-read cap so a client
// that opens a connection but never sends headers cannot pin a goroutine.
func newShotServer(h http.Handler) *http.Server {
	return &http.Server{Handler: h, ReadHeaderTimeout: 5 * time.Second}
}

// bootSeededApp opens sqlite plus duckdb over dbPath, seeds, and serves the
// app on lis; stop drains the server, closes the engines, and removes the
// scratch db.
func bootSeededApp(lis net.Listener, dbPath string) (base string, stop func() error, err error) {
	st, err := sqlite.Open(dbPath)
	if err != nil {
		return "", nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := seed.Load(st.DB); err != nil {
		_ = st.Close()
		_ = os.Remove(dbPath)
		return "", nil, fmt.Errorf("seed: %w", err)
	}
	eng, err := analytics.New(dbPath, &st.WriteMu)
	if err != nil {
		_ = st.Close()
		_ = os.Remove(dbPath)
		return "", nil, fmt.Errorf("open duckdb: %w", err)
	}
	srv := newShotServer(httpapi.New(nil, st, service.EntryService{St: st}, eng))
	go func() { _ = srv.Serve(lis) }()
	return "http://" + lis.Addr().String(), func() error {
		err := srv.Shutdown(context.Background())
		_ = eng.Close()
		_ = st.Close()
		_ = os.Remove(dbPath)
		return err
	}, nil
}

const (
	shotStart = "<!-- ac-shot:start -->"
	shotEnd   = "<!-- ac-shot:end -->"
)

// refreshReadme rewrites the marked shot block, inserting it right under the
// first heading when absent, so repeated runs stay idempotent.
func refreshReadme(readme, img string) error {
	body, err := os.ReadFile(readme)
	if err != nil {
		return err
	}
	block := shotStart + "\n![dashboard](" + img + ")\n" + shotEnd
	s := string(body)
	if i := strings.Index(s, shotStart); i >= 0 {
		j := strings.Index(s, shotEnd)
		if j < i {
			return fmt.Errorf("%s: end marker before start", readme)
		}
		s = s[:i] + block + s[j+len(shotEnd):]
	} else {
		head, rest, found := strings.Cut(s, "\n")
		if !found || !strings.HasPrefix(strings.TrimSpace(head), "#") {
			return fmt.Errorf("%s: no leading heading to anchor the shot", readme)
		}
		s = head + "\n\n" + block + "\n" + rest
	}
	return os.WriteFile(readme, []byte(s), 0o644)
}
