// Command seed-v2 registers the SpecMCP v2.0.0 template pack with an Emergent project.
// This pack renames CodingAgent -> Agent and adds agent_type field.
//
// The v2 pack is assigned ALONGSIDE v1.0.0 (parallel coexistence).
// After data migration (migrate-agents), the v1 pack can be unassigned.
//
// Usage:
//
//	EMERGENT_TOKEN=emt_... go run ./scripts/seed-v2.go
//	EMERGENT_TOKEN=emt_... go run ./scripts/seed-v2.go --dry-run
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	sdk "github.com/emergent-company/emergent/apps/server-go/pkg/sdk"
	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk/templatepacks"
	"github.com/emergent-company/specmcp/internal/config"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "Parse and validate the pack without registering")
	flag.Parse()

	if err := run(*dryRun); err != nil {
		log.Fatalf("seed-v2 failed: %v", err)
	}
}

func run(dryRun bool) error {
	ctx := context.Background()

	// Read template pack JSON
	packPath := findPackPath()
	packData, err := os.ReadFile(packPath)
	if err != nil {
		return fmt.Errorf("reading template pack: %w", err)
	}
	log.Printf("loaded template pack from %s (%d bytes)", packPath, len(packData))

	// Parse the JSON to extract fields
	var pack struct {
		Name                    string          `json:"name"`
		Version                 string          `json:"version"`
		Description             string          `json:"description"`
		Author                  string          `json:"author"`
		ObjectTypeSchemas       json.RawMessage `json:"object_type_schemas"`
		RelationshipTypeSchemas json.RawMessage `json:"relationship_type_schemas"`
		UIConfigs               json.RawMessage `json:"ui_configs"`
		ExtractionPrompts       json.RawMessage `json:"extraction_prompts"`
	}
	if err := json.Unmarshal(packData, &pack); err != nil {
		return fmt.Errorf("parsing template pack JSON: %w", err)
	}

	// Validate key changes - parse as array (v0.14+ format)
	var objSchemas []struct {
		Name       string                     `json:"name"`
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(pack.ObjectTypeSchemas, &objSchemas); err != nil {
		return fmt.Errorf("parsing object_type_schemas: %w", err)
	}

	hasAgent := false
	hasCodingAgent := false
	var agentProperties map[string]json.RawMessage

	for _, schema := range objSchemas {
		if schema.Name == "Agent" {
			hasAgent = true
			agentProperties = schema.Properties
		}
		if schema.Name == "CodingAgent" {
			hasCodingAgent = true
		}
	}

	if !hasAgent {
		return fmt.Errorf("v2 pack missing 'Agent' entity type")
	}
	if hasCodingAgent {
		return fmt.Errorf("v2 pack still contains 'CodingAgent' - not properly transformed")
	}

	log.Printf("pack %q v%s validated: %d entity types", pack.Name, pack.Version, len(objSchemas))

	// Check Agent has agent_type field
	if _, ok := agentProperties["agent_type"]; !ok {
		return fmt.Errorf("Agent schema missing 'agent_type' property")
	}
	log.Printf("Agent schema has %d properties including agent_type", len(agentProperties))

	if dryRun {
		log.Println("DRY RUN - pack validated successfully, not registering")

		// List entity types
		log.Println("entity types in v2 pack:")
		for _, schema := range objSchemas {
			log.Printf("  %s", schema.Name)
		}

		// List relationship types
		var relSchemas []struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(pack.RelationshipTypeSchemas, &relSchemas); err != nil {
			return fmt.Errorf("parsing relationship_type_schemas: %w", err)
		}
		log.Printf("relationship types: %d", len(relSchemas))

		return nil
	}

	// Load config from environment (only needed for actual registration)
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Initialize SDK client
	client, err := sdk.New(sdk.Config{
		ServerURL: cfg.Emergent.URL,
		ProjectID: cfg.Emergent.ProjectID,
		Auth: sdk.AuthConfig{
			Mode:   "apikey",
			APIKey: cfg.Emergent.Token,
		},
	})
	if err != nil {
		return fmt.Errorf("initializing SDK client: %w", err)
	}

	// List existing packs to show current state
	log.Println("checking existing template packs...")
	installed, err := client.TemplatePacks.GetInstalledPacks(ctx)
	if err != nil {
		log.Printf("warning: could not list installed packs: %v", err)
	} else {
		log.Printf("currently installed: %d packs", len(installed))
		for _, p := range installed {
			log.Printf("  pack: %s v%s (id: %s)", p.Name, p.Version, p.ID)
		}
	}

	// Create the v2 template pack
	log.Printf("creating template pack %q v%s...", pack.Name, pack.Version)
	created, err := client.TemplatePacks.CreatePack(ctx, &templatepacks.CreatePackRequest{
		Name:                    pack.Name,
		Version:                 pack.Version,
		Description:             strPtr(pack.Description),
		Author:                  strPtr(pack.Author),
		ObjectTypeSchemas:       pack.ObjectTypeSchemas,
		RelationshipTypeSchemas: pack.RelationshipTypeSchemas,
		UIConfigs:               pack.UIConfigs,
		ExtractionPrompts:       pack.ExtractionPrompts,
	})
	if err != nil {
		return fmt.Errorf("creating template pack: %w", err)
	}
	log.Printf("template pack created: id=%s", created.ID)

	// Assign the v2 pack to the project (alongside v1)
	log.Println("assigning v2 template pack to project (parallel with v1)...")
	assignment, err := client.TemplatePacks.AssignPack(ctx, &templatepacks.AssignPackRequest{
		TemplatePackID: created.ID,
	})
	if err != nil {
		return fmt.Errorf("assigning template pack: %w", err)
	}
	log.Printf("template pack assigned: assignment_id=%s", assignment.ID)

	// Verify compiled types show both CodingAgent (v1) and Agent (v2)
	log.Println("verifying compiled types...")
	compiled, err := client.TemplatePacks.GetCompiledTypes(ctx)
	if err != nil {
		return fmt.Errorf("getting compiled types: %w", err)
	}
	log.Printf("compiled types: %d object types, %d relationship types",
		len(compiled.ObjectTypes), len(compiled.RelationshipTypes))

	hasCodingAgentCompiled := false
	hasAgentCompiled := false
	for _, ot := range compiled.ObjectTypes {
		if ot.Name == "CodingAgent" {
			hasCodingAgentCompiled = true
			log.Printf("  CodingAgent: pack=%s (v1 - will be migrated)", ot.PackName)
		}
		if ot.Name == "Agent" {
			hasAgentCompiled = true
			log.Printf("  Agent: pack=%s (v2 - new)", ot.PackName)
		}
	}

	if hasCodingAgentCompiled && hasAgentCompiled {
		log.Println("PARALLEL COEXISTENCE: Both CodingAgent (v1) and Agent (v2) are available")
		log.Println("Next steps:")
		log.Println("  1. Run dry-run:         EMERGENT_TOKEN=emt_... ./dist/dry-run")
		log.Println("  2. Run migration:        EMERGENT_TOKEN=emt_... ./dist/migrate-agents")
		log.Println("  3. Verify migration:     EMERGENT_TOKEN=emt_... ./dist/verify-migration")
		log.Println("  4. Unassign v1 pack when migration is verified")
	} else if hasAgentCompiled && !hasCodingAgentCompiled {
		log.Println("NOTE: Only Agent type found (no CodingAgent). Fresh project or v1 already removed.")
	} else {
		log.Printf("WARNING: CodingAgent=%v, Agent=%v - unexpected state", hasCodingAgentCompiled, hasAgentCompiled)
	}

	log.Println("seed-v2 complete")
	return nil
}

// findPackPath locates the v2 template pack JSON file relative to this script.
func findPackPath() string {
	candidates := []string{
		"templates/specmcp-pack-v2.json",
		"server/specmcp/templates/specmcp-pack-v2.json",
	}

	_, thisFile, _, ok := runtime.Caller(0)
	if ok {
		dir := filepath.Dir(thisFile)
		candidates = append(candidates, filepath.Join(dir, "..", "templates", "specmcp-pack-v2.json"))
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			abs, _ := filepath.Abs(p)
			return abs
		}
	}

	return "templates/specmcp-pack-v2.json"
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
