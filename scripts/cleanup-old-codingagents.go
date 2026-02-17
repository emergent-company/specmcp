// Command cleanup-old-codingagents deletes inactive CodingAgent entities from the graph.
// These are entities that were migrated to Agent type and marked inactive.
//
// Usage:
//   go run scripts/cleanup-old-codingagents.go
//
// Environment variables:
//   EMERGENT_URL    - Emergent server URL (default: http://localhost:3002)
//   EMERGENT_TOKEN  - Emergent project token (required)

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk"
	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk/graph"
)

func main() {
	log.SetFlags(log.Ltime)

	// Get config from env
	emergentURL := os.Getenv("EMERGENT_URL")
	if emergentURL == "" {
		emergentURL = "http://localhost:3002"
	}
	emergentToken := os.Getenv("EMERGENT_TOKEN")
	if emergentToken == "" {
		log.Fatal("EMERGENT_TOKEN environment variable is required")
	}

	ctx := context.Background()

	// Parse project ID from token or env
	projectID := os.Getenv("EMERGENT_PROJECT_ID")
	if projectID == "" {
		log.Fatal("EMERGENT_PROJECT_ID environment variable is required")
	}

	// Create SDK client
	client, err := sdk.New(sdk.Config{
		ServerURL: emergentURL,
		ProjectID: projectID,
		Auth: sdk.AuthConfig{
			Mode:   "apikey",
			APIKey: emergentToken,
		},
	})
	if err != nil {
		log.Fatalf("Failed to create SDK client: %v", err)
	}

	log.Println("=== CodingAgent Cleanup ===")
	log.Printf("Emergent URL: %s", emergentURL)
	log.Println()

	// Step 1: List all CodingAgent entities
	log.Println("Step 1: Listing CodingAgent entities...")
	resp, err := client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  "CodingAgent",
		Limit: 100,
	})
	if err != nil {
		log.Fatalf("Failed to list CodingAgent entities: %v", err)
	}
	codingAgents := resp.Items
	log.Printf("Found %d CodingAgent entities", len(codingAgents))
	log.Println()

	if len(codingAgents) == 0 {
		log.Println("✓ No CodingAgent entities found. Nothing to cleanup.")
		return
	}

	// Step 2: Filter inactive entities
	var inactiveEntities []*graph.GraphObject
	var activeEntities []*graph.GraphObject

	for _, obj := range codingAgents {
		active, ok := obj.Properties["active"].(bool)
		if !ok || active {
			activeEntities = append(activeEntities, obj)
		} else {
			inactiveEntities = append(inactiveEntities, obj)
		}
	}

	log.Printf("  Active CodingAgent entities: %d", len(activeEntities))
	log.Printf("  Inactive CodingAgent entities: %d", len(inactiveEntities))
	log.Println()

	if len(activeEntities) > 0 {
		log.Println("⚠️  WARNING: Found active CodingAgent entities!")
		log.Println("   These should have been migrated to Agent type.")
		log.Println("   Review before proceeding.")
		log.Println()
		for _, obj := range activeEntities {
			name := getString(obj.Properties, "name")
			log.Printf("   - %s (ID: %s)", name, obj.ID)
		}
		log.Println()
	}

	if len(inactiveEntities) == 0 {
		log.Println("✓ No inactive CodingAgent entities to cleanup.")
		return
	}

	// Step 3: Show entities to be deleted
	log.Println("Step 2: Entities to be deleted:")
	for _, obj := range inactiveEntities {
		name := getString(obj.Properties, "name")
		tags := getStringArray(obj.Properties, "tags")
		hasMigratedTag := false
		for _, tag := range tags {
			if tag == "migrated:v2" {
				hasMigratedTag = true
				break
			}
		}
		log.Printf("  - %s (ID: %s, migrated: %v)", name, obj.ID, hasMigratedTag)
	}
	log.Println()

	// Step 4: Confirm deletion
	log.Printf("⚠️  About to DELETE %d inactive CodingAgent entities.", len(inactiveEntities))
	log.Print("Type 'DELETE' to confirm (case-sensitive): ")

	var confirmation string
	fmt.Scanln(&confirmation)

	if confirmation != "DELETE" {
		log.Println("Deletion cancelled.")
		return
	}
	log.Println()

	// Step 5: Delete entities
	log.Println("Step 3: Deleting entities...")
	deletedCount := 0
	failedCount := 0

	for _, obj := range inactiveEntities {
		name := getString(obj.Properties, "name")
		err := client.Graph.DeleteObject(ctx, obj.ID)
		if err != nil {
			log.Printf("  ✗ Failed to delete %s: %v", name, err)
			failedCount++
		} else {
			log.Printf("  ✓ Deleted %s", name)
			deletedCount++
		}
	}
	log.Println()

	// Step 6: Summary
	log.Println("=== Cleanup Summary ===")
	log.Printf("Total CodingAgent entities found: %d", len(codingAgents))
	log.Printf("Active (skipped): %d", len(activeEntities))
	log.Printf("Inactive (attempted): %d", len(inactiveEntities))
	log.Printf("Successfully deleted: %d", deletedCount)
	log.Printf("Failed: %d", failedCount)
	log.Println()

	if failedCount > 0 {
		log.Println("⚠️  Some deletions failed. Check errors above.")
		os.Exit(1)
	}

	log.Println("✓ Cleanup complete!")

	// Save cleanup report
	report := map[string]any{
		"timestamp":        time.Now().Format(time.RFC3339),
		"total_found":      len(codingAgents),
		"active_skipped":   len(activeEntities),
		"inactive_deleted": deletedCount,
		"failed":           failedCount,
		"deleted_ids":      make([]string, 0),
	}
	for _, obj := range inactiveEntities {
		if err == nil {
			report["deleted_ids"] = append(report["deleted_ids"].([]string), obj.ID)
		}
	}

	reportJSON, _ := json.MarshalIndent(report, "", "  ")
	reportPath := fmt.Sprintf("cleanup-report-%s.json", time.Now().Format("20060102-150405"))
	if err := os.WriteFile(reportPath, reportJSON, 0644); err != nil {
		log.Printf("Failed to write report: %v", err)
	} else {
		log.Printf("Cleanup report saved: %s", reportPath)
	}
}

func getString(props map[string]any, key string) string {
	if v, ok := props[key].(string); ok {
		return v
	}
	return ""
}

func getStringArray(props map[string]any, key string) []string {
	if v, ok := props[key].([]any); ok {
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}
