package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
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

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: mockr start [flags] <configFile>\n")
	fmt.Fprintf(os.Stderr, "\nFlags:\n")
	fmt.Fprintf(os.Stderr, "  -port int\n")
	fmt.Fprintf(os.Stderr, "        Port to run the server on (default 3000)\n")
	fmt.Fprintf(os.Stderr, "  -watch\n")
	fmt.Fprintf(os.Stderr, "        Enable hot reload file watching (default true)\n")
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	if os.Args[1] != "start" {
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}

	// Parse flags
	portFlag := flag.Int("port", 3000, "Port to run the server on")
	watchFlag := flag.Bool("watch", true, "Enable hot reload file watching")
	
	// Parse flags from os.Args[2:] (skip "start" command)
	flag.CommandLine.Parse(os.Args[2:])

	// Get config file from remaining args
	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Error: config file required\n")
		printUsage()
		os.Exit(1)
	}
	configFile := args[0]

	port := *portFlag
	watch := *watchFlag

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

	// Validate and filter routes
	validRoutes := make(map[string]Route)
	skippedCount := 0

	for path, route := range config.Routes {
		// Validate method
		method := strings.ToUpper(route.Method)
		validMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
		isValidMethod := false
		for _, validMethod := range validMethods {
			if method == validMethod {
				isValidMethod = true
				break
			}
		}

		if !isValidMethod {
			log.Printf("Warning: Unsupported method '%s' for route '%s', skipping", route.Method, path)
			skippedCount++
			continue
		}

		// Validate status code (must be valid HTTP status)
		if route.Status != 0 && (route.Status < 100 || route.Status > 599) {
			log.Printf("Warning: Invalid status code %d for route '%s', using default 200", route.Status, path)
			route.Status = 200
		}

		// Validate delay (must be non-negative)
		if route.Delay < 0 {
			log.Printf("Warning: Negative delay %d for route '%s', using 0", route.Delay, path)
			route.Delay = 0
		}

		validRoutes[path] = route
	}

	// Print table header
	fmt.Printf("Loaded %d mock routes", len(validRoutes))
	if skippedCount > 0 {
		fmt.Printf(" (skipped %d invalid routes)", skippedCount)
	}
	fmt.Println()
	fmt.Println("┌────────┬──────────────────────────┬────────┬────────┐")
	fmt.Println("│ METHOD │ PATH                     │ STATUS │ DELAY  │")
	fmt.Println("├────────┼──────────────────────────┼────────┼────────┤")

	// Print routes in table format
	for path, route := range validRoutes {
		method := strings.ToUpper(route.Method)
		status := route.Status
		if status == 0 {
			status = 200
		}

		delayStr := ""
		if route.Delay > 0 {
			delayStr = fmt.Sprintf("%dms", route.Delay)
		} else {
			delayStr = "-"
		}

		// Truncate long paths
		displayPath := path
		if len(displayPath) > 24 {
			displayPath = displayPath[:21] + "..."
		}

		fmt.Printf("│ %-6s │ %-24s │ %-6d │ %-6s │\n", method, displayPath, status, delayStr)
	}
	fmt.Println("└────────┴──────────────────────────┴────────┴────────┘")

	// Convert valid routes to server.Route format
	serverRoutes := make(map[string]server.Route)
	for path, route := range validRoutes {
		serverRoutes[path] = server.Route{
			Method:   route.Method,
			Status:   route.Status,
			Delay:    route.Delay,
			Response: route.Response,
		}
	}

	// Create reload callback
	onReload := func(newConfig map[string]server.Route) {
		fmt.Printf("🔄 config reloaded (%d routes)\n", len(newConfig))
	}

	// Start the server
	mockServer := server.New(serverRoutes, port, onReload)

	// Start file watcher for hot reload if enabled
	if watch {
		go watchConfigFile(configFile, mockServer)
	}

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

	// Validate and filter routes (same logic as main)
	validRoutes := make(map[string]Route)
	for path, route := range config.Routes {
		// Validate method
		method := strings.ToUpper(route.Method)
		validMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
		isValidMethod := false
		for _, validMethod := range validMethods {
			if method == validMethod {
				isValidMethod = true
				break
			}
		}

		if !isValidMethod {
			log.Printf("Warning: Unsupported method '%s' for route '%s', skipping", route.Method, path)
			continue
		}

		// Validate status code
		if route.Status != 0 && (route.Status < 100 || route.Status > 599) {
			log.Printf("Warning: Invalid status code %d for route '%s', using default 200", route.Status, path)
			route.Status = 200
		}

		// Validate delay
		if route.Delay < 0 {
			log.Printf("Warning: Negative delay %d for route '%s', using 0", route.Delay, path)
			route.Delay = 0
		}

		validRoutes[path] = route
	}

	// Convert to server.Route format
	serverRoutes := make(map[string]server.Route)
	for path, route := range validRoutes {
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
