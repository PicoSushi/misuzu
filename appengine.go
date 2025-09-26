// Package main provides functionality for managing Google App Engine services,
// including traffic allocation and version management operations.
package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	appengine "cloud.google.com/go/appengine/apiv1"
	appenginepb "cloud.google.com/go/appengine/apiv1/appenginepb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

const (
	// PercentageMultiplier is used to convert between decimal (0.0-1.0) and percentage (0-100)
	PercentageMultiplier = 100.0
	// TimeFormat is the standard format for displaying timestamps
	TimeFormat = "2006-01-02 15:04:05 UTC"
)

// AppEngineService represents an interface for App Engine operations.
// This interface abstracts the App Engine client operations for better testability.
type AppEngineService interface {
	// GetTrafficInfo retrieves traffic allocation information for a service
	GetTrafficInfo(ctx context.Context, projectID, serviceName string) (*TrafficInfo, error)
	// UpdateTrafficAllocation updates traffic allocation for specified versions
	UpdateTrafficAllocation(ctx context.Context, projectID, serviceName, shardBy string, allocations map[string]float64) error
	// ListServiceVersions retrieves all versions for a service
	ListServiceVersions(ctx context.Context, projectID, serviceName string) (*ServiceVersions, error)
}

// AppEngineClient implements AppEngineService interface.
// It provides concrete implementations for App Engine operations.
type AppEngineClient struct{}

// Helper functions for common operations

// buildServicePath constructs the service path for API calls
func buildServicePath(projectID, serviceName string) string {
	return fmt.Sprintf("apps/%s/services/%s", projectID, serviceName)
}

// createServicesClient creates and returns a new App Engine Services client
func createServicesClient(ctx context.Context) (*appengine.ServicesClient, error) {
	client, err := appengine.NewServicesClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create App Engine services client: %w", err)
	}
	return client, nil
}

// createVersionsClient creates and returns a new App Engine Versions client
func createVersionsClient(ctx context.Context) (*appengine.VersionsClient, error) {
	client, err := appengine.NewVersionsClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create App Engine versions client: %w", err)
	}
	return client, nil
}

// convertToPercentage converts decimal allocation to percentage
func convertToPercentage(allocation float64) float64 {
	return allocation * PercentageMultiplier
}

// convertToDecimal converts percentage to decimal allocation
func convertToDecimal(percentage float64) float64 {
	return percentage / PercentageMultiplier
}

// validateTrafficSplit checks if the service has valid traffic split information
func validateTrafficSplit(service *appenginepb.Service) error {
	if service.Split == nil {
		return fmt.Errorf("no traffic split information available")
	}
	if len(service.Split.Allocations) == 0 {
		return fmt.Errorf("no traffic allocations found")
	}
	return nil
}

// TrafficAllocation represents a single version's traffic allocation
type TrafficAllocation struct {
	Version    string  `json:"version"`
	Percentage float64 `json:"percentage"`
}

// TrafficInfo holds all traffic information for a service
type TrafficInfo struct {
	ServiceName string              `json:"service_name"`
	Allocations []TrafficAllocation `json:"allocations"`
}

// VersionInfo represents a single service version
type VersionInfo struct {
	VersionID      string    `json:"version_id"`
	CreateTime     time.Time `json:"create_time"`
	ServingStatus  string    `json:"serving_status"`
	HasTraffic     bool      `json:"has_traffic"`
	TrafficPercent float64   `json:"traffic_percent"`
}

// ServiceVersions holds all version information for a service
type ServiceVersions struct {
	ServiceName string        `json:"service_name"`
	Versions    []VersionInfo `json:"versions"`
}

// GetTrafficInfo retrieves service versions that have traffic allocation
func (c *AppEngineClient) GetTrafficInfo(ctx context.Context, projectID, serviceName string) (*TrafficInfo, error) {
	// Initialize result structure
	trafficInfo := &TrafficInfo{
		ServiceName: serviceName,
	}

	// Create App Engine client
	client, err := createServicesClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	// Get service information including traffic splits
	req := &appenginepb.GetServiceRequest{
		Name: buildServicePath(projectID, serviceName),
	}

	service, err := client.GetService(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get service '%s': %w", serviceName, err)
	}

	if err := validateTrafficSplit(service); err != nil {
		return nil, err
	}

	trafficInfo.Allocations = make([]TrafficAllocation, 0, len(service.Split.Allocations))

	for versionID, allocation := range service.Split.Allocations {
		percentage := convertToPercentage(allocation)
		trafficInfo.Allocations = append(trafficInfo.Allocations, TrafficAllocation{
			Version:    versionID,
			Percentage: percentage,
		})
	}

	return trafficInfo, nil
}

// displayTrafficInfo prints the traffic information in the same format as the original output
func displayTrafficInfo(trafficInfo *TrafficInfo) {
	fmt.Printf("=== Traffic Allocation for Service '%s' ===\n", trafficInfo.ServiceName)

	fmt.Printf("Total Allocations: %d\n\n", len(trafficInfo.Allocations))

	for _, allocation := range trafficInfo.Allocations {
		fmt.Printf("Version: %s\n", allocation.Version)
		fmt.Printf("Traffic: %.2f%%\n", allocation.Percentage)
		fmt.Println("---")
	}
}

// UpdateTrafficAllocation updates the traffic allocation for specified versions in an App Engine service
func (c *AppEngineClient) UpdateTrafficAllocation(ctx context.Context, projectID, serviceName, shardBy string, allocations map[string]float64) error {
	// Create App Engine Services client
	client, err := createServicesClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	value, ok := appenginepb.TrafficSplit_ShardBy_value[strings.ToUpper(shardBy)]
	if !ok {
		return fmt.Errorf("unsupported shardBy value: %s", shardBy)
	}
	resolvedShardBy := appenginepb.TrafficSplit_ShardBy(value)

	// Convert percentage values to decimal format required by the API
	apiAllocations := make(map[string]float64)
	for version, percentage := range allocations {
		apiAllocations[version] = convertToDecimal(percentage)
	}

	// Prepare the update request
	req := &appenginepb.UpdateServiceRequest{
		Name: buildServicePath(projectID, serviceName),
		Service: &appenginepb.Service{
			Split: &appenginepb.TrafficSplit{
				Allocations: apiAllocations,
				ShardBy:     resolvedShardBy,
			},
		},
		UpdateMask: &fieldmaskpb.FieldMask{
			Paths: []string{"split"},
		},
	}

	// Execute the update request
	operation, err := client.UpdateService(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to update service traffic: %w", err)
	}

	// Wait for the operation to complete
	_, err = operation.Wait(ctx)
	if err != nil {
		return fmt.Errorf("failed to wait for traffic update operation: %w", err)
	}

	fmt.Printf("Traffic allocation updated successfully for service '%s'\n", serviceName)
	return nil
}

// updateTrafficAllocation is a wrapper for backward compatibility
// buildVersionInfo creates a VersionInfo struct from API response
func buildVersionInfo(version *appenginepb.Version, trafficMap map[string]float64) VersionInfo {
	versionID := version.Id
	trafficPercent, hasTraffic := trafficMap[versionID]

	// Format create time
	createTime := time.Time{}
	if version.CreateTime != nil {
		createTime = version.CreateTime.AsTime()
	}

	return VersionInfo{
		VersionID:      versionID,
		CreateTime:     createTime,
		ServingStatus:  version.ServingStatus.String(),
		HasTraffic:     hasTraffic,
		TrafficPercent: trafficPercent,
	}
}

// getTrafficMap retrieves traffic information and converts it to a map
func getTrafficMap(ctx context.Context, svc AppEngineService, projectID, serviceName string) (map[string]float64, error) {
	trafficInfo, err := svc.GetTrafficInfo(ctx, projectID, serviceName)
	trafficMap := make(map[string]float64)
	if err != nil || trafficInfo == nil {
		return trafficMap, err
	}

	for _, allocation := range trafficInfo.Allocations {
		trafficMap[allocation.Version] = allocation.Percentage
	}
	return trafficMap, nil
}

// ListServiceVersions retrieves all versions for a service (including those without traffic)
func (c *AppEngineClient) ListServiceVersions(ctx context.Context, projectID, serviceName string) (*ServiceVersions, error) {
	// Initialize result structure
	serviceVersions := &ServiceVersions{
		ServiceName: serviceName,
	}

	// Create App Engine Versions client
	client, err := createVersionsClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	// Get traffic information first to know which versions have traffic
	trafficMap, err := getTrafficMap(ctx, c, projectID, serviceName)
	if err != nil {
		// Log the error but continue - not having traffic info is not fatal
		fmt.Printf("Warning: could not retrieve traffic information: %v\n", err)
	}

	// List all versions
	req := &appenginepb.ListVersionsRequest{
		Parent: buildServicePath(projectID, serviceName),
	}

	it := client.ListVersions(ctx, req)
	for {
		version, err := it.Next()
		if err != nil {
			if err.Error() == "no more items in iterator" {
				break
			}
			return nil, fmt.Errorf("failed to list versions: %w", err)
		}

		versionInfo := buildVersionInfo(version, trafficMap)
		serviceVersions.Versions = append(serviceVersions.Versions, versionInfo)
	}

	return serviceVersions, nil
}

// displayServiceVersions prints all service versions information
func displayServiceVersions(serviceVersions *ServiceVersions) {
	fmt.Printf("=== All Versions for Service '%s' ===\n", serviceVersions.ServiceName)
	fmt.Printf("Total Versions: %d\n\n", len(serviceVersions.Versions))

	for _, version := range serviceVersions.Versions {
		fmt.Printf("Version ID: %s\n", version.VersionID)
		if version.CreateTime.IsZero() {
			fmt.Printf("Create Time: Unknown\n")
		} else {
			fmt.Printf("Create Time: %s\n", version.CreateTime.Format(TimeFormat))
		}
		fmt.Printf("Serving Status: %s\n", version.ServingStatus)
		if version.HasTraffic {
			fmt.Printf("Traffic: %.2f%%\n", version.TrafficPercent)
		} else {
			fmt.Printf("Traffic: 0.00%% (No traffic allocation)\n")
		}
		fmt.Println("---")
	}
}
