package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/abdillahi-nur/mockr/internal/server"
	"github.com/fsnotify/fsnotify"
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

	// Create reload callback
	onReload := func(newConfig map[string]server.Route) {
		fmt.Printf("🔄 config reloaded (%d routes)\n", len(newConfig))
	}

	// Start the server
	mockServer := server.New(serverRoutes, port, onReload)

	// Start file watcher for hot reload
	go watchConfigFile(configFile, mockServer)

	if err := mockServer.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// watchConfigFile watches the config file for changes and reloads the server
func watchConfigFile(configFile string, mockServer *server.Server) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("Error creating watcher: %v", err)
		return
	}
	defer watcher.Close()

	// Watch the directory containing the config file
	configDir := "."
	configFileName := configFile
	if lastSlash := len(configFile) - 1; lastSlash >= 0 {
		for i := lastSlash; i >= 0; i-- {
			if configFile[i] == '/' || configFile[i] == '\\' {
				configDir = configFile[:i]
				configFileName = configFile[i+1:]
				break
			}
		}
	}

	err = watcher.Add(configDir)
	if err != nil {
		log.Printf("Error adding directory to watcher: %v", err)
		return
	}

	log.Printf("Watching config file: %s", configFileName)

	// Debounce timer to avoid multiple reloads
	var reloadTimer *time.Timer

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			// Check if the config file was modified
			if event.Op&fsnotify.Write == fsnotify.Write {
				// Extract filename from event path
				eventFile := event.Name
				if lastSlash := len(eventFile) - 1; lastSlash >= 0 {
					for i := lastSlash; i >= 0; i-- {
						if eventFile[i] == '/' || eventFile[i] == '\\' {
							eventFile = eventFile[i+1:]
							break
						}
					}
				}

				if eventFile == configFileName {
					// Debounce reload to avoid multiple rapid reloads
					if reloadTimer != nil {
						reloadTimer.Stop()
					}
					reloadTimer = time.AfterFunc(100*time.Millisecond, func() {
						reloadConfig(configFile, mockServer)
					})
				}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v", err)
		}
	}
}

// reloadConfig reloads the configuration and updates the server
func reloadConfig(configFile string, mockServer *server.Server) {
	// Read config file
	data, err := os.ReadFile(configFile)
	if err != nil {
		log.Printf("Error reading config file during reload: %v", err)
		return
	}

	// Parse JSON config
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		log.Printf("Error parsing config file during reload: %v", err)
		return
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

	// Reload server configuration
	mockServer.ReloadConfig(serverRoutes)
}
