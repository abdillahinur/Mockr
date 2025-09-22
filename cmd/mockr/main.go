package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/abdillahi-nur/mockr/internal/server"
)

// Route represents a mock API route configuration
type Route struct {
	Method   string      `json:"method"`
	Status   int         `json:"status,omitempty"`
	Delay    int         `json:"delay,omitempty"`
	Response interface{} `json:"response"`
}

// Config represents the mock server configuration
type Config struct {
	Routes map[string]Route `json:"routes"`
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: mockr start <configFile>\n")
		os.Exit(1)
	}

	if os.Args[1] != "start" {
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		fmt.Fprintf(os.Stderr, "Usage: mockr start <configFile>\n")
		os.Exit(1)
	}

	configFile := os.Args[2]

	// Check if config file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: Config file '%s' not found\n", configFile)
		os.Exit(1)
	}

	// Read config file
	data, err := os.ReadFile(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading config file: %v\n", err)
		os.Exit(1)
	}

	// Parse JSON config
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing config file: %v\n", err)
		os.Exit(1)
	}

	// Print parsed routes
	fmt.Println("Loaded mock routes:")
	for path, route := range config.Routes {
		status := route.Status
		if status == 0 {
			status = 200 // default status
		}
		delay := route.Delay
		if delay == 0 {
			delay = 0
		}
		
		fmt.Printf("  %s [%s] -> Status: %d", path, route.Method, status)
		if delay > 0 {
			fmt.Printf(", Delay: %dms", delay)
		}
		fmt.Println()
		fmt.Printf("    Response: %v\n", route.Response)
	}

	// Convert to server.Route format
	serverRoutes := make(map[string]server.Route)
	for path, route := range config.Routes {
		serverRoutes[path] = server.Route{
			Method:   route.Method,
			Status:   route.Status,
			Delay:    route.Delay,
			Response: route.Response,
		}
	}

	// Get port from environment or use default
	port := 3000
	if portStr := os.Getenv("PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	// Start the server
	mockServer := server.New(serverRoutes, port)
	if err := mockServer.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
