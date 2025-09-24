package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Route represents a mock API route configuration
type Route struct {
	Method   string      `json:"method"`
	Status   int         `json:"status,omitempty"`
	Delay    int         `json:"delay,omitempty"`
	Response interface{} `json:"response"`
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

// Server represents the mock HTTP server
type Server struct {
	config     map[string]Route
	host       string
	port       int
	mu         sync.RWMutex
	mux        *http.ServeMux
	onReload   func(map[string]Route)
	httpServer *http.Server
	limiter    *rate.Limiter
}

// New creates a new mock server instance
func New(config map[string]Route, host string, port int, onReload func(map[string]Route)) *Server {
	return &Server{
		config:   config,
		host:     host,
		port:     port,
		mux:      http.NewServeMux(),
		onReload: onReload,
	}
}

// SetRateLimit configures rate limiting for the server
func (s *Server) SetRateLimit(requestsPerSecond float64, burst int) {
	s.limiter = rate.NewLimiter(rate.Limit(requestsPerSecond), burst)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.registerRoutes()

	addr := fmt.Sprintf("%s:%d", s.host, s.port)

	// Configure HTTP server with security timeouts
	s.httpServer = &http.Server{
		Addr:              addr,
		Handler:           s.mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("Starting mock server on %s", addr)
	s.logRoutes()

	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the HTTP server
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}

	log.Println("Shutting down HTTP server...")
	return s.httpServer.Shutdown(ctx)
}

// bodyLimitMiddleware wraps request bodies with size limits
func (s *Server) bodyLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Limit request body size to 1MB for security
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		// Read and discard body to enforce size limit even if handler doesn't read it
		if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
			_, err := io.ReadAll(r.Body)
			if err != nil {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				json.NewEncoder(w).Encode(map[string]string{"error": "request body too large"})
				return
			}
		}

		next(w, r)
	}
}

// rateLimitMiddleware applies rate limiting if configured
func (s *Server) rateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Skip rate limiting if not configured
		if s.limiter == nil {
			next(w, r)
			return
		}

		// Check rate limit
		if !s.limiter.Allow() {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{"error": "rate_limited"})
			return
		}

		next(w, r)
	}
}

// loggingMiddleware logs request details
func (s *Server) loggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Process request
		next(rw, r)

		// Calculate duration
		duration := time.Since(start)

		// Log request: method path status duration_ms
		log.Printf("%s %s %d %dms", r.Method, r.URL.Path, rw.statusCode, duration.Milliseconds())
	}
}

// healthHandler handles the /health endpoint
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// registerRoutes registers all routes with the mux
func (s *Server) registerRoutes() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear existing routes by creating new mux
	s.mux = http.NewServeMux()

	// Always register /health endpoint first (no rate limiting, no delay, no status override)
	s.mux.HandleFunc("/health", s.loggingMiddleware(s.bodyLimitMiddleware(s.healthHandler)))

	// Register user-defined routes dynamically
	for path, route := range s.config {
		method := strings.ToUpper(route.Method)
		switch method {
		case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS":
			// Apply all middlewares in order: rate limit → body limit → handler → logging
			// Note: middleware wrapping is applied in reverse order
			handler := s.createHandler(route)
			handler = s.loggingMiddleware(handler)
			handler = s.bodyLimitMiddleware(handler)
			handler = s.rateLimitMiddleware(handler)
			s.mux.HandleFunc(path, handler)
		default:
			log.Printf("Warning: Unsupported method '%s' for route '%s', skipping", route.Method, path)
		}
	}

	// Add a default route for unregistered paths
	defaultHandler := s.defaultHandler
	defaultHandler = s.loggingMiddleware(defaultHandler)
	defaultHandler = s.bodyLimitMiddleware(defaultHandler)
	defaultHandler = s.rateLimitMiddleware(defaultHandler)
	s.mux.HandleFunc("/", defaultHandler)
}

// ReloadConfig updates the server configuration and re-registers routes
func (s *Server) ReloadConfig(newConfig map[string]Route) {
	s.mu.Lock()
	s.config = newConfig
	s.mu.Unlock()

	s.registerRoutes()
	s.logRoutes()

	if s.onReload != nil {
		s.onReload(newConfig)
	}
}

// logRoutes logs the current routes
func (s *Server) logRoutes() {
	log.Printf("Available routes:")

	// Always log /health endpoint
	log.Printf("  /health [GET] -> Status: 200 (health check)")

	// Log user-defined routes
	for path, route := range s.config {
		status := route.Status
		if status == 0 {
			status = 200
		}
		log.Printf("  %s [%s] -> Status: %d", path, route.Method, status)
	}
}

// createHandler creates an HTTP handler for a specific route
func (s *Server) createHandler(route Route) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Apply delay if configured (before setting headers)
		if route.Delay > 0 {
			time.Sleep(time.Duration(route.Delay) * time.Millisecond)
		}

		// Set content type to JSON with charset (before status code)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		// Set status code
		status := route.Status
		if status == 0 {
			status = 200 // default status
		}
		w.WriteHeader(status)

		// Marshal and write response
		if err := json.NewEncoder(w).Encode(route.Response); err != nil {
			log.Printf("Error encoding response for route %s: %v", r.URL.Path, err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}
}

// defaultHandler handles requests to unregistered routes
func (s *Server) defaultHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	response := map[string]interface{}{
		"error":  "Route not found",
		"path":   r.URL.Path,
		"method": r.Method,
	}

	json.NewEncoder(w).Encode(response)
}
