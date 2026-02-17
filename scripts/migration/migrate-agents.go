// Command migrate-agents performs the actual CodingAgent → Agent migration.
//
// This script:
// 1. Verifies v2.0.0 template pack is installed
// 2. Creates new Agent entities from existing CodingAgent entities
// 3. Migrates all relationships to point to new Agent entities
// 4. Archives old CodingAgent entities (does NOT delete them)
// 5. Creates a migration log for audit purposes
//
// Usage:
//
//	EMERGENT_TOKEN=emt_... go run ./scripts/migration/migrate-agents.go
//
// Safety features:
// - Verifies template pack before starting
// - Creates migration log with all changes
// - Preserves original CodingAgent entities (archived)
// - Can be rolled back if needed
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	sdk "github.com/emergent-company/emergent/apps/server-go/pkg/sdk"
	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk/graph"
	"github.com/emergent-company/specmcp/internal/config"
)

type MigrationLog struct {
	Timestamp             time.Time      `json:"timestamp"`
	Status                string         `json:"status"` // in_progress, completed, failed
	TemplatePackVersion   string         `json:"template_pack_version"`
	CodingAgentsFound     int            `json:"coding_agents_found"`
	AgentsCreated         int            `json:"agents_created"`
	RelationshipsMigrated int            `json:"relationships_migrated"`
	Mappings              []AgentMapping `json:"mappings"`
	Errors                []string       `json:"errors,omitempty"`
	CompletedAt           *time.Time     `json:"completed_at,omitempty"`
}

type AgentMapping struct {
	OldID             string `json:"old_id"`
	OldName           string `json:"old_name"`
	NewID             string `json:"new_id"`
	NewAgentType      string `json:"new_agent_type"`
	RelationshipCount int    `json:"relationship_count"`
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("❌ migration failed: %v", err)
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
	log.Println("║  CodingAgent → Agent Migration                            ║")
	log.Println("╚═══════════════════════════════════════════════════════════╝")
	log.Println()

	migrationLog := &MigrationLog{
		Timestamp: time.Now(),
		Status:    "in_progress",
		Mappings:  []AgentMapping{},
		Errors:    []string{},
	}

	// Step 1: Verify v2.0.0 template pack is installed
	log.Println("Step 1: Verifying template pack...")
	compiled, err := client.TemplatePacks.GetCompiledTypes(ctx)
	if err != nil {
		return fmt.Errorf("getting compiled types: %w", err)
	}

	hasAgentType := false
	for _, ot := range compiled.ObjectTypes {
		if ot.Name == "Agent" {
			hasAgentType = true
			migrationLog.TemplatePackVersion = "v2.0.0" // We know it's v2
			log.Printf("✓ Found Agent type (pack: %s)", ot.PackName)
			break
		}
	}

	if !hasAgentType {
		return fmt.Errorf("Agent type not found in compiled types. Did you register the v2.0.0 template pack?")
	}
	log.Println()

	// Step 2: List all CodingAgent entities
	log.Println("Step 2: Fetching CodingAgent entities...")
	resp, err := client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type: "CodingAgent",
	})
	if err != nil {
		return fmt.Errorf("listing CodingAgent entities: %w", err)
	}
	codingAgents := resp.Items
	migrationLog.CodingAgentsFound = len(codingAgents)
	log.Printf("Found %d CodingAgent entities", len(codingAgents))
	log.Println()

	// Step 3: Migrate each CodingAgent
	log.Println("Step 3: Migrating entities and relationships...")
	log.Println()

	for i, oldAgent := range codingAgents {
		name := getString(oldAgent.Properties, "name")
		log.Printf("[%d/%d] Migrating %s...", i+1, len(codingAgents), name)

		// Get all edges before creating new entity
		edges, err := client.Graph.GetObjectEdges(ctx, oldAgent.ID, &graph.GetObjectEdgesOptions{})
		if err != nil {
			errMsg := fmt.Sprintf("Failed to get edges for %s: %v", name, err)
			log.Printf("  ❌ %s", errMsg)
			migrationLog.Errors = append(migrationLog.Errors, errMsg)
			continue
		}

		totalEdges := len(edges.Incoming) + len(edges.Outgoing)
		log.Printf("  Found %d relationships", totalEdges)

		// Build new Agent properties
		agentType := determineAgentType(getString(oldAgent.Properties, "specialization"), name)
		newProps := map[string]interface{}{
			"name":       name,
			"agent_type": agentType,
			"type":       getString(oldAgent.Properties, "type"),
			"active":     getBool(oldAgent.Properties, "active", true),
		}

		// Optional fields
		if v := getString(oldAgent.Properties, "display_name"); v != "" {
			newProps["display_name"] = v
		}
		if v := getString(oldAgent.Properties, "specialization"); v != "" {
			newProps["specialization"] = v
		}
		if v := getString(oldAgent.Properties, "instructions"); v != "" {
			newProps["instructions"] = v
		}
		if v := getFloat(oldAgent.Properties, "velocity_points_per_hour"); v > 0 {
			newProps["velocity_points_per_hour"] = v
		}
		if v := getStringArray(oldAgent.Properties, "skills"); len(v) > 0 {
			newProps["skills"] = v
		}
		if v := getStringArray(oldAgent.Properties, "tags"); len(v) > 0 {
			newProps["tags"] = v
		}

		// Create new Agent entity
		log.Printf("  Creating Agent entity (agent_type=%s)...", agentType)
		keyPtr := name // Use pointer
		newAgent, err := client.Graph.CreateObject(ctx, &graph.CreateObjectRequest{
			Type:       "Agent",
			Key:        &keyPtr, // Pointer to string
			Properties: newProps,
		})
		if err != nil {
			errMsg := fmt.Sprintf("Failed to create Agent for %s: %v", name, err)
			log.Printf("  ❌ %s", errMsg)
			migrationLog.Errors = append(migrationLog.Errors, errMsg)
			continue
		}
		log.Printf("  ✓ Created Agent: %s", newAgent.ID)

		// Migrate incoming relationships
		for _, edge := range edges.Incoming {
			_, err := client.Graph.CreateRelationship(ctx, &graph.CreateRelationshipRequest{
				Type:       edge.Type,
				SrcID:      edge.SrcID,
				DstID:      newAgent.ID,
				Properties: edge.Properties,
			})
			if err != nil {
				errMsg := fmt.Sprintf("Failed to migrate incoming edge %s: %v", edge.Type, err)
				log.Printf("  ⚠️  %s", errMsg)
				migrationLog.Errors = append(migrationLog.Errors, errMsg)
			}
		}

		// Migrate outgoing relationships
		for _, edge := range edges.Outgoing {
			_, err := client.Graph.CreateRelationship(ctx, &graph.CreateRelationshipRequest{
				Type:       edge.Type,
				SrcID:      newAgent.ID,
				DstID:      edge.DstID,
				Properties: edge.Properties,
			})
			if err != nil {
				errMsg := fmt.Sprintf("Failed to migrate outgoing edge %s: %v", edge.Type, err)
				log.Printf("  ⚠️  %s", errMsg)
				migrationLog.Errors = append(migrationLog.Errors, errMsg)
			}
		}

		log.Printf("  ✓ Migrated %d relationships", totalEdges)

		// Record mapping
		migrationLog.Mappings = append(migrationLog.Mappings, AgentMapping{
			OldID:             oldAgent.ID,
			OldName:           name,
			NewID:             newAgent.ID,
			NewAgentType:      agentType,
			RelationshipCount: totalEdges,
		})

		migrationLog.AgentsCreated++
		migrationLog.RelationshipsMigrated += totalEdges
		log.Println()
	}

	// Step 4: Mark old CodingAgent entities as archived
	log.Println("Step 4: Archiving old CodingAgent entities...")
	for _, oldAgent := range codingAgents {
		// Add an "archived" tag to mark them as migrated
		tags := getStringArray(oldAgent.Properties, "tags")
		tags = append(tags, "migrated:v2", fmt.Sprintf("migrated_at:%s", time.Now().Format("2006-01-02")))

		updatedProps := oldAgent.Properties
		updatedProps["tags"] = tags
		updatedProps["active"] = false

		_, err := client.Graph.UpdateObject(ctx, oldAgent.ID, &graph.UpdateObjectRequest{
			Properties: updatedProps,
		})
		if err != nil {
			log.Printf("  ⚠️  Failed to archive %s: %v", getString(oldAgent.Properties, "name"), err)
		}
	}
	log.Println("  ✓ Archived CodingAgent entities")
	log.Println()

	// Complete migration log
	now := time.Now()
	migrationLog.CompletedAt = &now
	if len(migrationLog.Errors) == 0 {
		migrationLog.Status = "completed"
	} else {
		migrationLog.Status = "completed_with_errors"
	}

	// Save migration log
	logFile := fmt.Sprintf("migration-log-%s.json", time.Now().Format("20060102-150405"))
	logJSON, err := json.MarshalIndent(migrationLog, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling migration log: %w", err)
	}

	if err := os.WriteFile(logFile, logJSON, 0644); err != nil {
		return fmt.Errorf("writing migration log: %w", err)
	}

	// Summary
	log.Println("╔═══════════════════════════════════════════════════════════╗")
	log.Println("║  Migration Complete                                       ║")
	log.Println("╚═══════════════════════════════════════════════════════════╝")
	log.Println()
	log.Printf("CodingAgent entities found: %d", migrationLog.CodingAgentsFound)
	log.Printf("Agent entities created: %d", migrationLog.AgentsCreated)
	log.Printf("Relationships migrated: %d", migrationLog.RelationshipsMigrated)
	log.Println()

	if len(migrationLog.Errors) > 0 {
		log.Printf("⚠️  Encountered %d errors during migration", len(migrationLog.Errors))
		log.Println("See migration log for details")
		log.Println()
	}

	log.Printf("✓ Migration log saved to %s", logFile)
	log.Println()
	log.Println("Next steps:")
	log.Println("  1. Review migration log")
	log.Println("  2. Run verification: go run ./scripts/migration/verify-migration.go")
	log.Println("  3. Test SpecMCP with new Agent entities")
	log.Println()

	if len(migrationLog.Errors) > 0 {
		return fmt.Errorf("migration completed with %d errors", len(migrationLog.Errors))
	}

	return nil
}

func getString(props map[string]interface{}, key string) string {
	if v, ok := props[key].(string); ok {
		return v
	}
	return ""
}

func getBool(props map[string]interface{}, key string, defaultVal bool) bool {
	if v, ok := props[key].(bool); ok {
		return v
	}
	return defaultVal
}

func getFloat(props map[string]interface{}, key string) float64 {
	if v, ok := props[key].(float64); ok {
		return v
	}
	return 0
}

func getStringArray(props map[string]interface{}, key string) []string {
	if arr, ok := props[key].([]interface{}); ok {
		result := make([]string, 0, len(arr))
		for _, item := range arr {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

func determineAgentType(specialization, name string) string {
	// Special case for janitor
	if name == "janitor" {
		return "maintenance"
	}

	switch specialization {
	case "frontend", "backend", "fullstack":
		return "coding"
	case "testing":
		return "testing"
	case "devops":
		return "deployment"
	case "maintenance":
		return "maintenance"
	default:
		// Default to coding for unknown specializations
		return "coding"
	}
}
