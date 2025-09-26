package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/schollz/progressbar/v3"
)

// Config holds all configuration values for misuzu deployment
type Config struct {
	Project       string
	Service       string
	TargetVersion string
	Interval      int
	TargetRate    int
	StepRate      int
	DryRun        bool
}

// Constants for validation
const (
	DefaultInterval   = 300
	DefaultTargetRate = 100
	DefaultStepRate   = 10
	MinStepRate       = 1
	MaxStepRate       = 100
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error for %s: %s", e.Field, e.Message)
}

// parseFlags parses command line flags and returns a Config
func parseFlags() (*Config, error) {
	config := &Config{}

	flag.StringVar(&config.Project, "project", "", "Project ID (required)")
	flag.StringVar(&config.Service, "service", "", "Service name (required)")
	flag.StringVar(&config.TargetVersion, "target-version", "", "Migration target version (required)")
	flag.IntVar(&config.Interval, "interval", DefaultInterval, "Interval seconds (optional, default: 300)")
	flag.IntVar(&config.TargetRate, "target-rate", DefaultTargetRate, "Target percentage rate (optional, default: 100)")
	flag.IntVar(&config.StepRate, "step-rate", DefaultStepRate, "Rate increment per step - percentage of traffic to shift to target version per step (optional, default: 10, min: 1, max: 100)")
	flag.BoolVar(&config.DryRun, "dry-run", false, "Show what would be done without actually performing the deployment")

	flag.Parse()

	return config, nil
}

// validate checks if the configuration is valid
func (c *Config) validate() error {
	var validationErrors []error

	if c.Project == "" {
		validationErrors = append(validationErrors, ValidationError{Field: "project", Message: "is required"})
	}
	if c.Service == "" {
		validationErrors = append(validationErrors, ValidationError{Field: "service", Message: "is required"})
	}
	if c.TargetVersion == "" {
		validationErrors = append(validationErrors, ValidationError{Field: "target-version", Message: "is required"})
	}
	if c.StepRate < MinStepRate || c.StepRate > MaxStepRate {
		validationErrors = append(validationErrors, ValidationError{
			Field:   "step-rate",
			Message: fmt.Sprintf("must be between %d and %d (inclusive)", MinStepRate, MaxStepRate),
		})
	}

	if len(validationErrors) > 0 {
		return errors.Join(validationErrors...)
	}

	return nil
}

// display prints the configuration in a formatted manner
func (c *Config) display() {
	fmt.Println("=== Misuzu Canary Deployment Configuration ===")
	fmt.Printf("Project ID: %s\n", c.Project)
	fmt.Printf("Service Name: %s\n", c.Service)
	fmt.Printf("Target Version: %s\n", c.TargetVersion)
	fmt.Printf("Interval (seconds): %d\n", c.Interval)
	fmt.Printf("Target Rate (%%): %d\n", c.TargetRate)
	fmt.Printf("Step Rate (%%): %d\n", c.StepRate)
	fmt.Printf("Dry Run: %t\n", c.DryRun)
	if c.DryRun {
		fmt.Println("*** DRY RUN MODE - No actual deployment will be performed ***")
	}
	fmt.Println("=== Configuration Complete ===")
}

// sleepWithProgress sleeps for the specified duration while displaying a progress bar
func sleepWithProgress(duration int, description string) {
	if duration <= 0 {
		return
	}

	fmt.Printf("\n%s (%d seconds)\n", description, duration)

	// Create progress bar with custom options
	bar := progressbar.NewOptions(duration*10,
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetPredictTime(false), // Disable time prediction
		progressbar.OptionSetElapsedTime(true),  // Show elapsed time
		progressbar.OptionFullWidth(),
	)

	// Update progress bar every second
	for i := 0; i < duration*10; i++ {
		time.Sleep(time.Millisecond * 100)
		bar.Add(1)
	}

	fmt.Println() // Add newline after completion
}

// determineSourceVersion automatically determines the source version from traffic info
func determineSourceVersion(trafficInfo *TrafficInfo, targetVersion string) (string, error) {
	if len(trafficInfo.Allocations) == 0 {
		return "", fmt.Errorf("no traffic allocations found, cannot determine source version")
	}

	// Case 1: Only one version with 100% traffic
	if len(trafficInfo.Allocations) == 1 {
		allocation := trafficInfo.Allocations[0]
		if allocation.Version == targetVersion {
			return "", fmt.Errorf("target version '%s' already has 100%% traffic - no migration needed", targetVersion)
		}
		return allocation.Version, nil
	}

	// Case 2: Exactly two versions
	if len(trafficInfo.Allocations) == 2 {
		var sourceVersion string

		for _, allocation := range trafficInfo.Allocations {
			if allocation.Version != targetVersion {
				sourceVersion = allocation.Version
			}
		}

		if sourceVersion == "" {
			return "", fmt.Errorf("target version '%s' not found in current traffic allocations", targetVersion)
		}

		return sourceVersion, nil
	}

	// Case 3: More than two versions - abnormal case
	var versions []string
	for _, allocation := range trafficInfo.Allocations {
		versions = append(versions, fmt.Sprintf("%s (%.2f%%)", allocation.Version, allocation.Percentage))
	}

	return "", fmt.Errorf("abnormal traffic state: found %d versions with traffic allocation: %v", len(trafficInfo.Allocations), versions)
}

// isVersionInTrafficAllocations verifies if the specified version exists in the current traffic allocations
func isVersionInTrafficAllocations(trafficInfo *TrafficInfo, version string) bool {
	for _, allocation := range trafficInfo.Allocations {
		if allocation.Version == version {
			return true
		}
	}
	return false
}

func main() {
	config, err := parseFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	if err := config.validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration validation failed:\n%v\n", err)
		flag.Usage()
		os.Exit(1)
	}

	config.display()

	// List service versions and derive traffic allocations for the specified service
	fmt.Println("\nRetrieving service versions and traffic information...")
	ctx := context.Background()
	serviceClient := &AppEngineClient{}
	serviceVersions, err := serviceClient.ListServiceVersions(ctx, config.Project, config.Service)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list service versions: %v\n", err)
		os.Exit(1)
	}

	trafficInfo := trafficInfoFromServiceVersions(serviceVersions)

	// Display the traffic information
	displayTrafficInfo(trafficInfo)

	sleepWithProgress(config.Interval, "Sleep until next split ...")

	targetVersion := config.TargetVersion
	// Check if the target version exists in the current traffic allocations
	if !isVersionInTrafficAllocations(trafficInfo, targetVersion) {
		fmt.Fprintf(os.Stderr, "Error: Target version '%s' not found in current traffic allocations\n", targetVersion)
		os.Exit(1)
	}

	// Automatically determine source version
	fmt.Println("\nDetermining source version...")
	sourceVersion, err := determineSourceVersion(trafficInfo, targetVersion)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to determine source version: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Source Version: %s, Traffic: %.2f%%\n", sourceVersion, getTrafficPercentage(trafficInfo, sourceVersion))
	fmt.Printf("Target Version: %s, Traffic: %.2f%%\n", targetVersion, getTrafficPercentage(trafficInfo, targetVersion))

	if getTrafficPercentage(trafficInfo, targetVersion) >= float64(config.TargetRate) {
		fmt.Printf("Target version '%s' has already reached the target rate of %d%%. No further action needed.\n", targetVersion, config.TargetRate)
		os.Exit(0)
	}

	nextTargetRate := getTrafficPercentage(trafficInfo, targetVersion) + float64(config.StepRate)

	if nextTargetRate > float64(config.TargetRate) {
		nextTargetRate = float64(config.TargetRate)
	}
	nextSourceRate := 100.0 - nextTargetRate

	// Prepare traffic allocation map for the update
	allocations := map[string]float64{
		sourceVersion: nextSourceRate,
		targetVersion: nextTargetRate,
	}

	err = serviceClient.UpdateTrafficAllocation(ctx, config.Project, config.Service, "", allocations)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to update traffic: %v\n", err)
		os.Exit(1)
	}
}

// getTrafficPercentage returns the traffic percentage for a specific version
func getTrafficPercentage(trafficInfo *TrafficInfo, version string) float64 {
	for _, allocation := range trafficInfo.Allocations {
		if allocation.Version == version {
			return allocation.Percentage
		}
	}
	return -1.0
}
