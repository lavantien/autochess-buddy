package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

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
