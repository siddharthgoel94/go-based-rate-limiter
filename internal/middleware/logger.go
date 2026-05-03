package middleware

import (
	"api-gateway/internal/db"
	"log"
	"net/http"
	"time"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		latency := time.Since(start)
		clientID := r.Header.Get("X-API-Key")
		if clientID == "" {
			clientID = r.RemoteAddr
		}

		log.Printf("[%s] %s %s | status=%d | latency=%dms | client=%s",
			time.Now().Format(time.RFC3339),
			r.Method, r.URL.Path,
			wrapped.statusCode,
			latency.Milliseconds(),
			clientID,
		)

		db.LogRequest(clientID, r.URL.Path, r.Method, wrapped.statusCode, int(latency.Milliseconds()))
	})
}
