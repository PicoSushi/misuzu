package main

import (
	"context"
	"fmt"

	appengine "cloud.google.com/go/appengine/apiv1"
	appenginepb "cloud.google.com/go/appengine/apiv1/appenginepb"
)

// listTrafficVersions retrieves service versions that have traffic allocation
func listTrafficVersions(ctx context.Context) error {
	// Create App Engine client
	client, err := appengine.NewServicesClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create App Engine client: %w", err)
	}
	defer client.Close()

	projectID := "my-project" // FIXME: Replace with actual project ID or pass as parameter
	serviceName := "default"  // FIXME

	// Get service information including traffic splits
	req := &appenginepb.GetServiceRequest{
		Name: fmt.Sprintf("apps/%s/services/%s", projectID, serviceName),
	}

	service, err := client.GetService(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to get service '%s': %w", serviceName, err)
	}

	fmt.Printf("=== Traffic Allocation for Service '%s' ===\n", serviceName)

	if service.Split == nil {
		fmt.Println("No traffic split information available")
		return nil
	}

	if len(service.Split.Allocations) == 0 {
		fmt.Println("No traffic allocations found")
		return nil
	}

	fmt.Printf("Total Allocations: %d\n\n", len(service.Split.Allocations))

	for versionID, allocation := range service.Split.Allocations {
		percentage := allocation * 100
		fmt.Printf("Version: %s\n", versionID)
		fmt.Printf("Traffic: %.2f%%\n", percentage)
		fmt.Println("---")
	}

	return nil
}
