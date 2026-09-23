package httpapi

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseForm_MalformedBodyKeepsQueryValues(t *testing.T) {
	// "%zz" is not valid url-encoding: ParseForm fails and the query map
	// rides alone, which the delete routes depend on.
	req := httptest.NewRequest("POST", "/x?match_id=7", strings.NewReader("bad=%zz"))
	v := parseForm(req)
	if v["match_id"] != "7" || v["bad"] != "" {
		t.Fatalf("v = %v, want query-only {match_id:7}", v)
	}
}

func TestOptInt64(t *testing.T) {
	f := formValues{"absent": "", "blank": " ", "ok": "5", "bad": "abc"}
	for _, key := range []string{"absent", "blank", "bad"} {
		if p := f.optInt64(key); p != nil {
			t.Fatalf("optInt64(%q) = %v, want nil", key, *p)
		}
	}
	if p := f.optInt64("ok"); p == nil || *p != 5 {
		t.Fatalf("optInt64(ok) = %v, want 5", p)
	}
}
