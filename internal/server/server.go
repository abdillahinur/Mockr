package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Route represents a mock API route configuration
type Route struct {
	Method   string      `json:"method"`
	Status   int         `json:"status,omitempty"`
	Delay    int         `json:"delay,omitempty"`
	Response interface{} `json:"response"`
}

// Server represents the mock HTTP server
type Server struct {
	config map[string]Route
	port   int
}

// New creates a new mock server instance
func New(config map[string]Route, port int) *Server {
	return &Server{
		config: config,
		port:   port,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	// Register routes dynamically
	for path, route := range s.config {
		switch route.Method {
		case "GET":
			http.HandleFunc(path, s.createHandler(route))
		case "POST":
			http.HandleFunc(path, s.createHandler(route))
		default:
			log.Printf("Warning: Unsupported method '%s' for route '%s', skipping", route.Method, path)
		}
	}

	// Add a default route for unregistered paths
	http.HandleFunc("/", s.defaultHandler)

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("Starting mock server on port %d", s.port)
	log.Printf("Available routes:")
	for path, route := range s.config {
		status := route.Status
		if status == 0 {
			status = 200
		}
		log.Printf("  %s [%s] -> Status: %d", path, route.Method, status)
	}

	return http.ListenAndServe(addr, nil)
}

// createHandler creates an HTTP handler for a specific route
func (s *Server) createHandler(route Route) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Apply delay if configured
		if route.Delay > 0 {
			time.Sleep(time.Duration(route.Delay) * time.Millisecond)
		}

		// Set status code
		status := route.Status
		if status == 0 {
			status = 200
		}
		w.WriteHeader(status)

		// Set content type to JSON
		w.Header().Set("Content-Type", "application/json")

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
	w.Header().Set("Content-Type", "application/json")
	
	response := map[string]interface{}{
		"error": "Route not found",
		"path":  r.URL.Path,
		"method": r.Method,
	}
	
	json.NewEncoder(w).Encode(response)
}
