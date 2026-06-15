package middleware

import (
	"bufio"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// responseWriter обёртка для захвата статус кода
type responseWriter struct {
	http.ResponseWriter
	status int
}

// Hijack реализует http.Hijacker для поддержки WebSockets
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, fmt.Errorf("http.Hijacker not implemented by underlying response writer")
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

// Logging логирует каждый HTTP запрос
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		userID, _ := r.Context().Value(ContextUserID).(string)

		next.ServeHTTP(rw, r)

		latency := time.Since(start)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"latency_ms", latency.Milliseconds(),
			"user_id", userID,
			"remote_addr", r.RemoteAddr,
		)
	})
}
