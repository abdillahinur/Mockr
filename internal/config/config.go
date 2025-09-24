package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
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

// ValidationResult contains the results of config validation
type ValidationResult struct {
	ValidRoutes  map[string]Route
	SkippedCount int
}

// LoadConfig loads and validates a configuration file
func LoadConfig(configFile string) (*ValidationResult, error) {
	// Check if config file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file '%s' not found", configFile)
	}

	// Resolve symlinks for security
	resolvedConfigFile, err := filepath.EvalSymlinks(configFile)
	if err != nil {
		return nil, fmt.Errorf("error resolving config file path: %w", err)
	}

	// Read config file
	data, err := os.ReadFile(resolvedConfigFile)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	// Parse JSON config
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}

	// Validate and filter routes
	result := validateRoutes(config.Routes)

	return result, nil
}

// validateRoutes validates and filters routes according to security rules
func validateRoutes(routes map[string]Route) *ValidationResult {
	validRoutes := make(map[string]Route)
	skippedCount := 0

	for path, route := range routes {
		// Validate method
		if !isValidMethod(route.Method) {
			log.Printf("Warning: Unsupported method '%s' for route '%s', skipping", route.Method, path)
			skippedCount++
			continue
		}

		// Validate status code (must be valid HTTP status)
		if route.Status != 0 && !isValidStatusCode(route.Status) {
			log.Printf("Warning: Invalid status code %d for route '%s', using default 200", route.Status, path)
			route.Status = 200
		}

		// Validate and clamp delay (0 ≤ delay ≤ 30_000 ms)
		route.Delay = clampDelay(route.Delay, path)

		validRoutes[path] = route
	}

	return &ValidationResult{
		ValidRoutes:  validRoutes,
		SkippedCount: skippedCount,
	}
}

// isValidMethod checks if the HTTP method is allowed
func isValidMethod(method string) bool {
	upperMethod := strings.ToUpper(method)
	validMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

	for _, validMethod := range validMethods {
		if upperMethod == validMethod {
			return true
		}
	}
	return false
}

// isValidStatusCode checks if the status code is in valid HTTP range
func isValidStatusCode(status int) bool {
	return status >= 100 && status <= 599
}

// clampDelay ensures delay is within safe bounds (0 ≤ delay ≤ 30_000 ms)
func clampDelay(delay int, path string) int {
	if delay < 0 {
		log.Printf("Warning: Negative delay %d for route '%s', using 0", delay, path)
		return 0
	}

	if delay > 30000 {
		log.Printf("Warning: Delay %dms exceeds 30s limit for route '%s', capping at 30s", delay, path)
		return 30000
	}

	return delay
}

// PrintRoutesTable prints a formatted table of routes
func (vr *ValidationResult) PrintRoutesTable() {
	fmt.Printf("Loaded %d mock routes", len(vr.ValidRoutes))
	if vr.SkippedCount > 0 {
		fmt.Printf(" (skipped %d invalid routes)", vr.SkippedCount)
	}
	fmt.Println()
	fmt.Println("┌────────┬──────────────────────────┬────────┬────────┐")
	fmt.Println("│ METHOD │ PATH                     │ STATUS │ DELAY  │")
	fmt.Println("├────────┼──────────────────────────┼────────┼────────┤")

	// Print routes in table format
	for path, route := range vr.ValidRoutes {
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
}
