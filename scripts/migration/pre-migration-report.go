// Command pre-migration-report generates a report of the current state before migration.
//
// This script analyzes:
// - Installed template packs
// - CodingAgent entity count
// - Relationship counts to/from CodingAgents
// - Affected relationship types
//
// Usage:
//
//	EMERGENT_TOKEN=emt_... go run ./scripts/migration/pre-migration-report.go
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

type MigrationReport struct {
	Timestamp              time.Time      `json:"timestamp"`
	InstalledPacks         []PackInfo     `json:"installed_packs"`
	CodingAgentCount       int            `json:"coding_agent_count"`
	CodingAgents           []AgentInfo    `json:"coding_agents"`
	RelationshipCounts     map[string]int `json:"relationship_counts"`
	AffectedRelationships  []string       `json:"affected_relationships"`
	RelationshipBreakdown  map[string]int `json:"relationship_breakdown"`
	EstimatedMigrationTime string         `json:"estimated_migration_time"`
}

type PackInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	ID      string `json:"id"`
}

type AgentInfo struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	Type                string `json:"type"`
	Specialization      string `json:"specialization"`
	DeterminedAgentType string `json:"determined_agent_type"`
	IncomingEdges       int    `json:"incoming_edges"`
	OutgoingEdges       int    `json:"outgoing_edges"`
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("pre-migration report failed: %v", err)
	}
}

func run() error {
	ctx := context.Background()

	// Load config
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Create SDK client directly (like seed.go)
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

	log.Println("Generating pre-migration report...")
	log.Println()

	report := &MigrationReport{
		Timestamp:             time.Now(),
		RelationshipCounts:    make(map[string]int),
		RelationshipBreakdown: make(map[string]int),
	}

	// 1. Get installed template packs
	log.Println("Fetching installed template packs...")
	installedPacks, err := client.TemplatePacks.GetInstalledPacks(ctx)
	if err != nil {
		log.Printf("WARNING: Could not fetch installed packs: %v", err)
	} else {
		for _, pack := range installedPacks {
			report.InstalledPacks = append(report.InstalledPacks, PackInfo{
				Name:    pack.Name,
				Version: pack.Version,
				ID:      pack.ID,
			})
			log.Printf("  - %s v%s (id=%s)", pack.Name, pack.Version, pack.ID)
		}
	}
	log.Println()

	// 2. List all CodingAgent entities
	log.Println("Fetching CodingAgent entities...")
	resp, err := client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type: "CodingAgent",
	})
	if err != nil {
		return fmt.Errorf("listing CodingAgent entities: %w", err)
	}
	codingAgents := resp.Items
	report.CodingAgentCount = len(codingAgents)
	log.Printf("Found %d CodingAgent entities:", len(codingAgents))

	// 3. Analyze each CodingAgent
	totalIncoming := 0
	totalOutgoing := 0
	relationshipLabels := make(map[string]int)

	for i, agent := range codingAgents {
		name := getString(agent.Properties, "name")
		agentType := getString(agent.Properties, "type")
		specialization := getString(agent.Properties, "specialization")
		determinedAgentType := determineAgentType(agent.Properties)

		log.Printf("  [%d] %s (type=%s, specialization=%s)", i+1, name, agentType, specialization)
		log.Printf("      → Will become Agent with agent_type=%s", determinedAgentType)

		// Get relationships
		edges, err := client.Graph.GetObjectEdges(ctx, agent.ID, &graph.GetObjectEdgesOptions{})
		if err != nil {
			log.Printf("      WARNING: Could not fetch edges: %v", err)
			continue
		}

		incomingCount := len(edges.Incoming)
		outgoingCount := len(edges.Outgoing)
		totalIncoming += incomingCount
		totalOutgoing += outgoingCount

		log.Printf("      Relationships: %d incoming, %d outgoing", incomingCount, outgoingCount)

		// Count relationship types
		for _, edge := range edges.Incoming {
			relationshipLabels[edge.Type+" (incoming)"]++
		}
		for _, edge := range edges.Outgoing {
			relationshipLabels[edge.Type+" (outgoing)"]++
		}

		report.CodingAgents = append(report.CodingAgents, AgentInfo{
			ID:                  agent.ID,
			Name:                name,
			Type:                agentType,
			Specialization:      specialization,
			DeterminedAgentType: determinedAgentType,
			IncomingEdges:       incomingCount,
			OutgoingEdges:       outgoingCount,
		})
	}

	report.RelationshipCounts["incoming"] = totalIncoming
	report.RelationshipCounts["outgoing"] = totalOutgoing
	report.RelationshipBreakdown = relationshipLabels

	log.Println()
	log.Printf("Total relationships to migrate: %d", totalIncoming+totalOutgoing)

	// 4. List affected relationship types
	log.Println()
	log.Println("Relationship types that will be affected:")
	for label, count := range relationshipLabels {
		log.Printf("  - %s: %d", label, count)
		report.AffectedRelationships = append(report.AffectedRelationships, label)
	}

	// 5. Estimate migration time
	estimatedSeconds := len(codingAgents)*5 + (totalIncoming+totalOutgoing)*2
	report.EstimatedMigrationTime = fmt.Sprintf("%d seconds", estimatedSeconds)
	log.Println()
	log.Printf("Estimated migration time: %s", report.EstimatedMigrationTime)

	// 6. Save report
	reportFile := "pre-migration-report.json"
	reportJSON, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling report: %w", err)
	}

	if err := os.WriteFile(reportFile, reportJSON, 0644); err != nil {
		return fmt.Errorf("writing report file: %w", err)
	}

	log.Println()
	log.Printf("✓ Report saved to %s", reportFile)
	log.Println()
	log.Println("Summary:")
	log.Printf("  - Template packs installed: %d", len(report.InstalledPacks))
	log.Printf("  - CodingAgent entities: %d", report.CodingAgentCount)
	log.Printf("  - Total relationships: %d", totalIncoming+totalOutgoing)
	log.Printf("  - Estimated migration time: %s", report.EstimatedMigrationTime)
	log.Println()
	log.Println("Next steps:")
	log.Println("  1. Review pre-migration-report.json")
	log.Println("  2. Run: go run ./scripts/migration/dry-run.go")

	return nil
}

func getString(props map[string]interface{}, key string) string {
	if v, ok := props[key].(string); ok {
		return v
	}
	return ""
}

func determineAgentType(props map[string]interface{}) string {
	specialization := getString(props, "specialization")

	switch specialization {
	case "maintenance":
		return "maintenance"
	case "frontend", "backend", "fullstack":
		return "coding"
	case "testing":
		return "testing"
	case "devops":
		return "deployment"
	default:
		return "coding" // Default to coding
	}
}
