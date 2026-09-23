package httpapi

import (
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

func OriginGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		host, ok := headerHost(r)
		if !ok {
			http.Error(w, "403: mutating request without origin or referer", http.StatusForbidden)
			return
		}
		if host != r.Host {
			http.Error(w, "403: origin host "+host+" does not match server host "+r.Host, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func headerHost(r *http.Request) (string, bool) {
	for _, name := range []string{"Origin", "Referer"} {
		v := r.Header.Get(name)
		if v == "" {
			continue
		}
		u, err := url.Parse(v)
		if err != nil || u.Host == "" {
			continue
		}
		return u.Host, true
	}
	return "", false
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func logRequests(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(sw, r)
		log.Info("http request", "method", r.Method, "path", r.URL.Path, "status", sw.status, "duration", time.Since(start))
	})
}
