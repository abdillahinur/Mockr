package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/abdillahi-nur/mockr/internal/config"
	"github.com/abdillahi-nur/mockr/internal/server"
	"github.com/fsnotify/fsnotify"
)

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: mockr start [flags] <configFile>\n")
	fmt.Fprintf(os.Stderr, "\nFlags:\n")
	fmt.Fprintf(os.Stderr, "  -host string\n")
	fmt.Fprintf(os.Stderr, "        Host to bind to (default \"127.0.0.1\")\n")
	fmt.Fprintf(os.Stderr, "  -port int\n")
	fmt.Fprintf(os.Stderr, "        Port to run the server on (default 3000)\n")
	fmt.Fprintf(os.Stderr, "  -watch\n")
	fmt.Fprintf(os.Stderr, "        Enable hot reload file watching (default true)\n")
	fmt.Fprintf(os.Stderr, "  -rate-limit float\n")
	fmt.Fprintf(os.Stderr, "        Rate limit in requests per second (default 0 = disabled)\n")
	fmt.Fprintf(os.Stderr, "  -burst int\n")
	fmt.Fprintf(os.Stderr, "        Burst size for rate limiting (default 0; only used if rate-limit > 0)\n")
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
	hostFlag := flag.String("host", "127.0.0.1", "Host to bind to")
	portFlag := flag.Int("port", 3000, "Port to run the server on")
	watchFlag := flag.Bool("watch", true, "Enable hot reload file watching")
	rateLimitFlag := flag.Float64("rate-limit", 0, "Rate limit in requests per second (default 0 = disabled)")
	burstFlag := flag.Int("burst", 0, "Burst size for rate limiting (default 0; only used if rate-limit > 0)")

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

	host := *hostFlag
	port := *portFlag
	watch := *watchFlag
	rateLimit := *rateLimitFlag
	burst := *burstFlag

	// Load and validate configuration
	configResult, err := config.LoadConfig(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Print routes table
	configResult.PrintRoutesTable()

	// Resolve symlinks for security (needed for file watching)
	resolvedConfigFile, err := filepath.EvalSymlinks(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving config file path: %v\n", err)
		os.Exit(1)
	}

	// Convert valid routes to server.Route format
	serverRoutes := make(map[string]server.Route)
	for path, route := range configResult.ValidRoutes {
		serverRoutes[path] = server.Route{
			Method:   route.Method,
			Status:   route.Status,
			Delay:    route.Delay,
			Response: route.Response,
		}
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create reload callback
	onReload := func(newConfig map[string]server.Route) {
		fmt.Printf("🔄 config reloaded (%d routes)\n", len(newConfig))
	}

	// Start the server with rate limiting configuration
	mockServer := server.New(serverRoutes, host, port, onReload)
	if rateLimit > 0 {
		mockServer.SetRateLimit(rateLimit, burst)
		log.Printf("Rate limiting enabled: %.2f req/s, burst: %d", rateLimit, burst)
	}

	// Channel to track file watcher lifecycle
	watcherDone := make(chan struct{})

	// Start file watcher for hot reload if enabled
	if watch {
		go func() {
			defer close(watcherDone)
			watchConfigFile(ctx, resolvedConfigFile, mockServer)
		}()
	} else {
		// If no watcher, close the channel immediately
		close(watcherDone)
	}

	// Start server in goroutine
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- mockServer.Start()
	}()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for either server error or shutdown signal
	select {
	case err := <-serverDone:
		if err != nil {
			log.Fatalf("Server error: %v", err)
		}
	case sig := <-sigChan:
		log.Printf("Received signal %v, initiating graceful shutdown...", sig)

		// Cancel context to stop file watcher
		cancel()

		// Wait for file watcher to stop (with timeout)
		select {
		case <-watcherDone:
			log.Println("File watcher stopped")
		case <-time.After(2 * time.Second):
			log.Println("File watcher stop timeout")
		}

		// Create shutdown context with 10 second timeout
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		// Shutdown server gracefully
		if err := mockServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("Server shutdown error: %v", err)
			os.Exit(1)
		}

		log.Println("Server shutdown complete")
	}
}

// watchConfigFile watches the config file for changes and reloads the server
func watchConfigFile(ctx context.Context, configFile string, mockServer *server.Server) {
	// Store the original resolved path for symlink safety
	originalResolvedPath, err := filepath.EvalSymlinks(configFile)
	if err != nil {
		log.Printf("Error resolving config file path for watching: %v", err)
		return
	}

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
		case <-ctx.Done():
			// Context cancelled, stop watching
			log.Println("Stopping file watcher due to shutdown signal")
			return
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
					// Security check: verify the resolved path hasn't changed (symlink safety)
					currentResolvedPath, err := filepath.EvalSymlinks(configFile)
					if err != nil {
						log.Printf("Error resolving config file path during reload: %v", err)
						continue
					}
					if currentResolvedPath != originalResolvedPath {
						log.Printf("Warning: Config file path resolution changed, ignoring reload for security")
						continue
					}

					// Debounce reload to avoid multiple rapid reloads
					if reloadTimer != nil {
						reloadTimer.Stop()
					}
					reloadTimer = time.AfterFunc(200*time.Millisecond, func() {
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
	// Load and validate configuration using the config package
	configResult, err := config.LoadConfig(configFile)
	if err != nil {
		log.Printf("Error loading config file during reload: %v", err)
		return
	}

	// Convert to server.Route format
	serverRoutes := make(map[string]server.Route)
	for path, route := range configResult.ValidRoutes {
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
