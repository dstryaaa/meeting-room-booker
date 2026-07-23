package middleware

import (
	"log"
	"net/http"
	"time"
)

func LoggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("📥 %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
		log.Printf("✅ %s %s - %v", r.Method, r.URL.Path, time.Since(start))
	}
}
