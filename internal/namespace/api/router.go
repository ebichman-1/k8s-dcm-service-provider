package api

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

// SetupRouter sets up the HTTP router for the namespace service
func SetupRouter(handler *Handler, logger *zap.Logger) *mux.Router {
	router := mux.NewRouter()

	// Add middleware
	router.Use(loggingMiddleware(logger))
	router.Use(corsMiddleware)

	// API v1 routes (AEP-compliant: GET for list)
	v1 := router.PathPrefix("/api/v1").Subrouter()
	v1.HandleFunc("/namespaces", handler.ListNamespaces).Methods("GET")
	v1.HandleFunc("/health", handler.HealthCheck).Methods("GET")

	// Handle 404 and 405 errors
	router.NotFoundHandler = http.HandlerFunc(handler.NotFoundHandler)
	router.MethodNotAllowedHandler = http.HandlerFunc(handler.MethodNotAllowedHandler)

	return router
}

// loggingMiddleware logs HTTP requests using zap
func loggingMiddleware(logger *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Create a wrapped ResponseWriter to capture status code
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Process request
			next.ServeHTTP(wrapped, r)

			// Log request
			duration := time.Since(start)
			logger.Info("HTTP request",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status_code", wrapped.statusCode),
				zap.Duration("duration", duration),
				zap.String("remote_addr", r.RemoteAddr),
				zap.String("user_agent", r.UserAgent()),
				zap.String("query_params", r.URL.RawQuery),
			)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// corsMiddleware adds CORS headers
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
