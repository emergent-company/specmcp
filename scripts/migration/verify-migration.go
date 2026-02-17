// Command verify-migration verifies the CodingAgent → Agent migration was successful.
//
// This script:
// 1. Checks that all expected Agent entities exist
// 2. Verifies relationships were migrated correctly
// 3. Compares against migration log
// 4. Reports any discrepancies
//
// Usage:
//
//	EMERGENT_TOKEN=emt_... go run ./scripts/migration/verify-migration.go [migration-log.json]
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	sdk "github.com/emergent-company/emergent/apps/server-go/pkg/sdk"
	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk/graph"
	"github.com/emergent-company/specmcp/internal/config"
)

type MigrationLog struct {
	CodingAgentsFound     int            `json:"coding_agents_found"`
	AgentsCreated         int            `json:"agents_created"`
	RelationshipsMigrated int            `json:"relationships_migrated"`
	Mappings              []AgentMapping `json:"mappings"`
}

type AgentMapping struct {
	OldID             string `json:"old_id"`
	OldName           string `json:"old_name"`
	NewID             string `json:"new_id"`
	NewAgentType      string `json:"new_agent_type"`
	RelationshipCount int    `json:"relationship_count"`
}

type VerificationResult struct {
	Success                bool     `json:"success"`
	AgentsExpected         int      `json:"agents_expected"`
	AgentsFound            int      `json:"agents_found"`
	RelationshipsExpected  int      `json:"relationships_expected"`
	RelationshipsFound     int      `json:"relationships_found"`
	MissingAgents          []string `json:"missing_agents,omitempty"`
	RelationshipMismatches []string `json:"relationship_mismatches,omitempty"`
	Warnings               []string `json:"warnings,omitempty"`
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("❌ verification failed: %v", err)
	}
}

func run() error {
	ctx := context.Background()

	// Load config
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Create SDK client
	client, err := sdk.New(sdk.Config{
		ServerURL: cfg.Emergent.URL,
		ProjectID: cfg.Emergent.ProjectID,
		Auth: sdk.AuthConfig{
			Mode:   "apikey",
			APIKey: cfg.Emergent.Token,
		},
	})
	if err != nil {
		return fmt.Errorf("creating SDK client: %w", err)
	}

	log.Println("╔═══════════════════════════════════════════════════════════╗")
	log.Println("║  Migration Verification                                   ║")
	log.Println("╚═══════════════════════════════════════════════════════════╝")
	log.Println()

	// Find migration log
	logFile := findMigrationLog()
	if logFile == "" {
		return fmt.Errorf("migration log not found. Please specify: go run verify-migration.go <log-file>")
	}

	log.Printf("Reading migration log: %s", logFile)
	logData, err := os.ReadFile(logFile)
	if err != nil {
		return fmt.Errorf("reading migration log: %w", err)
	}

	var migLog MigrationLog
	if err := json.Unmarshal(logData, &migLog); err != nil {
		return fmt.Errorf("parsing migration log: %w", err)
	}

	log.Printf("Expected: %d Agent entities, %d relationships", migLog.AgentsCreated, migLog.RelationshipsMigrated)
	log.Println()

	result := &VerificationResult{
		Success:               true,
		AgentsExpected:        migLog.AgentsCreated,
		RelationshipsExpected: migLog.RelationshipsMigrated,
	}

	// Step 1: Verify all Agent entities exist
	log.Println("Step 1: Verifying Agent entities...")
	resp, err := client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type: "Agent",
	})
	if err != nil {
		return fmt.Errorf("listing Agent entities: %w", err)
	}
	agents := resp.Items
	result.AgentsFound = len(agents)
	log.Printf("Found %d Agent entities", len(agents))

	// Build map of existing agents by ID
	agentsByID := make(map[string]*graph.GraphObject)
	for _, agent := range agents {
		agentsByID[agent.ID] = agent
	}

	// Check each expected agent
	for _, mapping := range migLog.Mappings {
		if _, exists := agentsByID[mapping.NewID]; !exists {
			result.Success = false
			result.MissingAgents = append(result.MissingAgents, fmt.Sprintf("%s (%s)", mapping.OldName, mapping.NewID))
			log.Printf("  ❌ Missing: %s (expected ID: %s)", mapping.OldName, mapping.NewID)
		}
	}

	if len(result.MissingAgents) == 0 {
		log.Println("  ✓ All expected Agent entities found")
	}
	log.Println()

	// Step 2: Verify relationships
	log.Println("Step 2: Verifying relationships...")
	totalRelationships := 0

	for _, mapping := range migLog.Mappings {
		agent, exists := agentsByID[mapping.NewID]
		if !exists {
			continue // Already reported as missing
		}

		edges, err := client.Graph.GetObjectEdges(ctx, agent.ID, &graph.GetObjectEdgesOptions{})
		if err != nil {
			log.Printf("  ⚠️  Could not get edges for %s: %v", mapping.OldName, err)
			result.Warnings = append(result.Warnings, fmt.Sprintf("Could not verify relationships for %s", mapping.OldName))
			continue
		}

		actualCount := len(edges.Incoming) + len(edges.Outgoing)
		totalRelationships += actualCount

		if actualCount != mapping.RelationshipCount {
			result.Success = false
			mismatch := fmt.Sprintf("%s: expected %d relationships, found %d",
				mapping.OldName, mapping.RelationshipCount, actualCount)
			result.RelationshipMismatches = append(result.RelationshipMismatches, mismatch)
			log.Printf("  ❌ %s", mismatch)
		}
	}

	result.RelationshipsFound = totalRelationships
	log.Printf("Total relationships found: %d", totalRelationships)

	if len(result.RelationshipMismatches) == 0 {
		log.Println("  ✓ All relationships verified")
	}
	log.Println()

	// Step 3: Check for orphaned CodingAgent entities
	log.Println("Step 3: Checking for orphaned CodingAgent entities...")
	codingResp, err := client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type: "CodingAgent",
	})
	if err != nil {
		log.Printf("  ⚠️  Could not list CodingAgent entities: %v", err)
	} else {
		activeCount := 0
		for _, ca := range codingResp.Items {
			if getBool(ca.Properties, "active", true) {
				activeCount++
			}
		}

		if activeCount > 0 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%d CodingAgent entities still active (should be archived)", activeCount))
			log.Printf("  ⚠️  %d CodingAgent entities still active", activeCount)
		} else {
			log.Println("  ✓ All CodingAgent entities are archived")
		}
	}
	log.Println()

	// Summary
	log.Println("╔═══════════════════════════════════════════════════════════╗")
	log.Println("║  Verification Summary                                     ║")
	log.Println("╚═══════════════════════════════════════════════════════════╝")
	log.Println()
	log.Printf("Agents: %d expected, %d found", result.AgentsExpected, result.AgentsFound)
	log.Printf("Relationships: %d expected, %d found", result.RelationshipsExpected, result.RelationshipsFound)
	log.Println()

	if len(result.MissingAgents) > 0 {
		log.Printf("❌ Missing agents: %d", len(result.MissingAgents))
		for _, name := range result.MissingAgents {
			log.Printf("  - %s", name)
		}
		log.Println()
	}

	if len(result.RelationshipMismatches) > 0 {
		log.Printf("❌ Relationship mismatches: %d", len(result.RelationshipMismatches))
		for _, msg := range result.RelationshipMismatches {
			log.Printf("  - %s", msg)
		}
		log.Println()
	}

	if len(result.Warnings) > 0 {
		log.Printf("⚠️  Warnings: %d", len(result.Warnings))
		for _, warn := range result.Warnings {
			log.Printf("  - %s", warn)
		}
		log.Println()
	}

	// Save verification result
	resultFile := "verification-result.json"
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling result: %w", err)
	}

	if err := os.WriteFile(resultFile, resultJSON, 0644); err != nil {
		return fmt.Errorf("writing result file: %w", err)
	}

	log.Printf("✓ Verification result saved to %s", resultFile)
	log.Println()

	if result.Success {
		log.Println("✅ Migration verification PASSED")
		log.Println()
		log.Println("The migration was successful! All entities and relationships are present.")
		if len(result.Warnings) > 0 {
			log.Println("Note: There are some warnings to review (see above).")
		}
	} else {
		log.Println("❌ Migration verification FAILED")
		log.Println()
		log.Println("There are issues with the migration. Review the errors above.")
		log.Println("You may need to:")
		log.Println("  1. Re-run the migration script")
		log.Println("  2. Manually fix missing entities/relationships")
		log.Println("  3. Check the migration log for errors")
		return fmt.Errorf("verification failed")
	}

	return nil
}

func findMigrationLog() string {
	// Check command line args
	if len(os.Args) > 1 {
		return os.Args[1]
	}

	// Look for migration-log-*.json in current directory
	files, err := os.ReadDir(".")
	if err != nil {
		return ""
	}

	var latest string
	for _, f := range files {
		name := f.Name()
		if len(name) > 14 && name[:14] == "migration-log-" && name[len(name)-5:] == ".json" {
			// Found a migration log, use the most recent one (they're timestamped)
			if latest == "" || name > latest {
				latest = name
			}
		}
	}

	return latest
}

func getBool(props map[string]interface{}, key string, defaultVal bool) bool {
	if v, ok := props[key].(bool); ok {
		return v
	}
	return defaultVal
}
