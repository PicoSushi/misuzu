package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
)

// Config holds all configuration values for misuzu deployment
type Config struct {
	Project            string
	Service            string
	SourceVersion      string
	DestinationVersion string
	Interval           int
	TargetRate         int
	StepRate           int
	DryRun             bool
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
	flag.StringVar(&config.Project, "p", "", "Project ID (required) [short]")
	flag.StringVar(&config.Service, "service", "", "Service name (required)")
	flag.StringVar(&config.Service, "s", "", "Service name (required) [short]")
	flag.StringVar(&config.SourceVersion, "source-version", "", "Migration source version (required)")
	flag.StringVar(&config.SourceVersion, "f", "", "Migration source version (required) [short]")
	flag.StringVar(&config.DestinationVersion, "destination-version", "", "Migration destination version (required)")
	flag.StringVar(&config.DestinationVersion, "t", "", "Migration destination version (required) [short]")
	flag.IntVar(&config.Interval, "interval", DefaultInterval, "Interval seconds (optional, default: 300)")
	flag.IntVar(&config.Interval, "i", DefaultInterval, "Interval seconds (optional, default: 300) [short]")
	flag.IntVar(&config.TargetRate, "target-rate", DefaultTargetRate, "Target percentage rate (optional, default: 100)")
	flag.IntVar(&config.TargetRate, "r", DefaultTargetRate, "Target percentage rate (optional, default: 100) [short]")
	flag.IntVar(&config.StepRate, "step-rate", DefaultStepRate, "Rate increment per step - percentage of traffic to shift to destination version per step (optional, default: 10, min: 1, max: 100)")
	flag.IntVar(&config.StepRate, "e", DefaultStepRate, "Rate increment per step - percentage of traffic to shift to destination version per step (optional, default: 10, min: 1, max: 100) [short]")
	flag.BoolVar(&config.DryRun, "dry-run", false, "Show what would be done without actually performing the deployment")
	flag.BoolVar(&config.DryRun, "d", false, "Show what would be done without actually performing the deployment [short]")

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
	if c.SourceVersion == "" {
		validationErrors = append(validationErrors, ValidationError{Field: "source-version", Message: "is required"})
	}
	if c.DestinationVersion == "" {
		validationErrors = append(validationErrors, ValidationError{Field: "destination-version", Message: "is required"})
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
	fmt.Printf("Source Version: %s\n", c.SourceVersion)
	fmt.Printf("Destination Version: %s\n", c.DestinationVersion)
	fmt.Printf("Interval (seconds): %d\n", c.Interval)
	fmt.Printf("Target Rate (%%): %d\n", c.TargetRate)
	fmt.Printf("Step Rate (%%): %d\n", c.StepRate)
	fmt.Printf("Dry Run: %t\n", c.DryRun)
	if c.DryRun {
		fmt.Println("*** DRY RUN MODE - No actual deployment will be performed ***")
	}
	fmt.Println("=== Configuration Complete ===")
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

	// List traffic versions for the specified service
	fmt.Println("\nRetrieving traffic information...")
	ctx := context.Background()
	if err := listTrafficVersions(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list traffic versions: %v\n", err)
		os.Exit(1)
	}
}
