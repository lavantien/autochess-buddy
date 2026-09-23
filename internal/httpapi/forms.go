package httpapi

import (
	"net/http"
	"strconv"
	"strings"
)

// formValues keeps raw strings so 422 rerenders preserve exactly what was typed.
type formValues map[string]string

func parseForm(r *http.Request) formValues {
	v := formValues{}
	// Query params ride along on the delete routes: Go refuses to parse DELETE
	// bodies, so their context ids travel in the URL.
	for k, vs := range r.URL.Query() {
		if len(vs) > 0 {
			v[k] = vs[0]
		}
	}
	if err := r.ParseForm(); err != nil {
		return v
	}
	for k := range r.PostForm {
		v[k] = r.PostForm.Get(k)
	}
	return v
}

func (f formValues) str(key string) string {
	return f[key]
}

func (f formValues) int(key string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(f[key]))
	return n
}

func (f formValues) int64(key string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(f[key]), 10, 64)
	return n
}

// optInt64 returns nil for absent or blank values.
func (f formValues) optInt64(key string) *int64 {
	s := strings.TrimSpace(f[key])
	if s == "" {
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil
	}
	return &n
}

func pathID(r *http.Request, name string) int64 {
	id, _ := strconv.ParseInt(r.PathValue(name), 10, 64)
	return id
}
