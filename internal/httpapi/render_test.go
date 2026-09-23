package httpapi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type errComp struct{}

func (errComp) Render(context.Context, io.Writer) error { return errors.New("boom") }

func TestIsHX(t *testing.T) {
	cases := []struct {
		name   string
		header string
		want   bool
	}{
		{"value true", "true", true},
		{"any non-empty value counts", "1", true},
		{"header absent", "", false},
		{"empty header value", "", false},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
		if c.name != "header absent" {
			req.Header.Set("HX-Request", c.header)
		}
		if got := isHX(req); got != c.want {
			t.Fatalf("%s: isHX = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestRender_LogsToInjectedLogger(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	renderPage(log, httptest.NewRecorder(), req, http.StatusOK, errComp{})
	renderOOB(log, httptest.NewRecorder(), req, http.StatusOK, errComp{})
	if got := buf.String(); strings.Count(got, "boom") != 2 {
		t.Fatalf("injected logger must receive page and oob render errors, got %q", got)
	}
}
