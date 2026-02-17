// Command dry-run simulates the CodingAgent → Agent migration without making changes.
//
// This script:
// - Reads all CodingAgent entities
// - Shows what new Agent entities would be created
// - Shows what relationships would be migrated
// - Estimates migration time
// - Does NOT make any changes to the graph
//
// Usage:
//
//	EMERGENT_TOKEN=emt_... go run ./scripts/migration/dry-run.go
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

type DryRunResult struct {
	Timestamp           time.Time      `json:"timestamp"`
	CodingAgentCount    int            `json:"coding_agent_count"`
	AgentsToCreate      []NewAgent     `json:"agents_to_create"`
	RelationshipsToMove map[string]int `json:"relationships_to_move"`
	EstimatedTime       string         `json:"estimated_time"`
	Warnings            []string       `json:"warnings"`
}

type NewAgent struct {
	OldID             string     `json:"old_id"`
	OldName           string     `json:"old_name"`
	NewProperties     AgentProps `json:"new_properties"`
	IncomingEdgeCount int        `json:"incoming_edge_count"`
	OutgoingEdgeCount int        `json:"outgoing_edge_count"`
}

type AgentProps struct {
	Name                  string   `json:"name"`
	DisplayName           string   `json:"display_name,omitempty"`
	AgentType             string   `json:"agent_type"`
	HumanOrAI             string   `json:"type"` // "human" or "ai"
	Active                bool     `json:"active"`
	Skills                []string `json:"skills,omitempty"`
	Specialization        string   `json:"specialization,omitempty"`
	Instructions          string   `json:"instructions,omitempty"`
	VelocityPointsPerHour float64  `json:"velocity_points_per_hour,omitempty"`
	Tags                  []string `json:"tags,omitempty"`
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("dry-run failed: %v", err)
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

	log.Println("🔍 DRY-RUN MODE: No changes will be made")
	log.Println()
	log.Println("Analyzing current state...")
	log.Println()

	result := &DryRunResult{
		Timestamp:           time.Now(),
		RelationshipsToMove: make(map[string]int),
		Warnings:            []string{},
	}

	// 1. List all CodingAgent entities
	log.Println("Fetching CodingAgent entities...")
	resp, err := client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type: "CodingAgent",
	})
	if err != nil {
		return fmt.Errorf("listing CodingAgent entities: %w", err)
	}
	codingAgents := resp.Items
	result.CodingAgentCount = len(codingAgents)
	log.Printf("Found %d CodingAgent entities", len(codingAgents))
	log.Println()

	// 2. Analyze each CodingAgent and determine migration
	totalIncoming := 0
	totalOutgoing := 0

	for i, agent := range codingAgents {
		name := getString(agent.Properties, "name")
		displayName := getString(agent.Properties, "display_name")
		humanOrAI := getString(agent.Properties, "type")
		active := getBool(agent.Properties, "active", true)
		specialization := getString(agent.Properties, "specialization")
		instructions := getString(agent.Properties, "instructions")
		velocity := getFloat(agent.Properties, "velocity_points_per_hour")
		skills := getStringArray(agent.Properties, "skills")
		tags := getStringArray(agent.Properties, "tags")

		// Determine new agent_type based on specialization
		agentType := determineAgentType(specialization, name)

		log.Printf("[%d] %s", i+1, name)
		log.Printf("    Old type: CodingAgent (specialization=%s)", specialization)
		log.Printf("    New type: Agent (agent_type=%s)", agentType)

		// Get relationships
		edges, err := client.Graph.GetObjectEdges(ctx, agent.ID, &graph.GetObjectEdgesOptions{})
		if err != nil {
			log.Printf("    ⚠️  WARNING: Could not fetch edges: %v", err)
			result.Warnings = append(result.Warnings, fmt.Sprintf("Agent %s: could not fetch edges", name))
			continue
		}

		incomingCount := len(edges.Incoming)
		outgoingCount := len(edges.Outgoing)
		totalIncoming += incomingCount
		totalOutgoing += outgoingCount

		log.Printf("    Relationships: %d incoming, %d outgoing", incomingCount, outgoingCount)

		// Count relationship types
		relTypes := make(map[string]bool)
		for _, edge := range edges.Incoming {
			relTypes[edge.Type] = true
			result.RelationshipsToMove[edge.Type+" (incoming)"]++
		}
		for _, edge := range edges.Outgoing {
			relTypes[edge.Type] = true
			result.RelationshipsToMove[edge.Type+" (outgoing)"]++
		}

		if len(relTypes) > 0 {
			log.Printf("    Relationship types: %v", keys(relTypes))
		}

		// Build new agent properties
		newAgent := NewAgent{
			OldID:   agent.ID,
			OldName: name,
			NewProperties: AgentProps{
				Name:                  name,
				DisplayName:           displayName,
				AgentType:             agentType,
				HumanOrAI:             humanOrAI,
				Active:                active,
				Skills:                skills,
				Specialization:        specialization,
				Instructions:          instructions,
				VelocityPointsPerHour: velocity,
				Tags:                  tags,
			},
			IncomingEdgeCount: incomingCount,
			OutgoingEdgeCount: outgoingCount,
		}
		result.AgentsToCreate = append(result.AgentsToCreate, newAgent)
		log.Println()
	}

	// 3. Summary
	log.Println("═══════════════════════════════════════════════════════════")
	log.Println("DRY-RUN SUMMARY")
	log.Println("═══════════════════════════════════════════════════════════")
	log.Println()
	log.Printf("Entities to migrate: %d CodingAgent → Agent", result.CodingAgentCount)
	log.Printf("Relationships to migrate: %d total", totalIncoming+totalOutgoing)
	log.Println()

	if len(result.RelationshipsToMove) > 0 {
		log.Println("Relationships by type:")
		for relType, count := range result.RelationshipsToMove {
			log.Printf("  - %s: %d", relType, count)
		}
		log.Println()
	}

	// Estimate time (conservative: 5s per entity, 2s per relationship)
	estimatedSeconds := len(codingAgents)*5 + (totalIncoming+totalOutgoing)*2
	result.EstimatedTime = fmt.Sprintf("%d seconds (~%.1f minutes)", estimatedSeconds, float64(estimatedSeconds)/60)
	log.Printf("Estimated migration time: %s", result.EstimatedTime)
	log.Println()

	if len(result.Warnings) > 0 {
		log.Println("⚠️  Warnings:")
		for _, w := range result.Warnings {
			log.Printf("  - %s", w)
		}
		log.Println()
	}

	// 4. Save result
	outputFile := "dry-run-result.json"
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling result: %w", err)
	}

	if err := os.WriteFile(outputFile, resultJSON, 0644); err != nil {
		return fmt.Errorf("writing result file: %w", err)
	}

	log.Printf("✓ Detailed results saved to %s", outputFile)
	log.Println()
	log.Println("═══════════════════════════════════════════════════════════")
	log.Println()
	log.Println("Next steps:")
	log.Println("  1. Review dry-run-result.json")
	log.Println("  2. If everything looks good, run the actual migration:")
	log.Println("     go run ./scripts/migration/migrate-agents.go")
	log.Println()

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

func keys(m map[string]bool) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}
