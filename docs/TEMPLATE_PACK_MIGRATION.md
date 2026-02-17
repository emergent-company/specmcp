# Template Pack Migration Strategy

**Created**: 2026-02-17  
**For Version**: v2.0.0 (CodingAgent → Agent refactoring)

## Overview

This document describes the strategy for migrating from SpecMCP template pack v1.0.0 to v2.0.0, which renames `CodingAgent` to `Agent` and adds new properties.

---

## Understanding Template Packs in Emergent

### Key Concepts

1. **Template Pack**: A versioned collection of object type schemas and relationship type schemas
2. **Assignment**: A project can have multiple template packs assigned simultaneously
3. **Compiled Types**: Emergent merges all assigned packs into a unified type system
4. **Version Coexistence**: Multiple versions of the same pack can be assigned to a project

### Available APIs

```go
// Create a new version of a pack
client.TemplatePacks.CreatePack(ctx, &CreatePackRequest{
    Name:    "SpecMCP",
    Version: "2.0.0",  // New version
    ObjectTypeSchemas: ...
})

// Assign pack to project
client.TemplatePacks.AssignPack(ctx, &AssignPackRequest{
    TemplatePackID: packID,
})

// List what's installed
client.TemplatePacks.GetInstalledPacks(ctx)

// See compiled types (merged view)
client.TemplatePacks.GetCompiledTypes(ctx)

// Remove pack assignment
client.TemplatePacks.DeleteAssignment(ctx, assignmentID)
```

---

## Migration Strategy: Parallel Assignment Approach

### Phase 1: Create v2.0.0 Alongside v1.0.0

**Goal**: Both `CodingAgent` (v1) and `Agent` (v2) coexist temporarily

**Steps**:

1. Create `templates/specmcp-pack-v2.json` with:
   - Version: `2.0.0`
   - Remove `CodingAgent` object type
   - Add `Agent` object type with new properties
   - Update relationship descriptions (but keep same relationship names)

2. Register v2.0.0 template pack:
   ```bash
   EMERGENT_TOKEN=emt_... go run ./scripts/seed-v2.go
   ```

3. Both packs now assigned to project:
   - `SpecMCP v1.0.0` - Has `CodingAgent`
   - `SpecMCP v2.0.0` - Has `Agent`

4. Emergent compiled types will include **both** types

### Phase 2: Data Migration

**Goal**: Copy all `CodingAgent` entities to `Agent` entities

**Migration Script**: `scripts/migrate-agents.go`

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/emergent-company/specmcp/internal/config"
    "github.com/emergent-company/specmcp/internal/emergent"
)

func main() {
    ctx := context.Background()
    
    // Load config
    cfg, err := config.Load("")
    if err != nil {
        log.Fatalf("loading config: %v", err)
    }
    
    // Create client factory
    factory := emergent.NewClientFactory(cfg.Emergent.URL, cfg.Emergent.Token, nil)
    client, err := factory.ClientFor(ctx)
    if err != nil {
        log.Fatalf("creating client: %v", err)
    }
    
    // Run migration
    if err := migrateAgents(ctx, client); err != nil {
        log.Fatalf("migration failed: %v", err)
    }
    
    log.Println("migration complete")
}

func migrateAgents(ctx context.Context, client *emergent.Client) error {
    // 1. List all CodingAgent entities
    log.Println("fetching all CodingAgent entities...")
    codingAgents, err := client.ListObjects(ctx, &graph.ListObjectsOptions{
        Type: "CodingAgent",  // Old type
    })
    if err != nil {
        return fmt.Errorf("listing CodingAgent entities: %w", err)
    }
    log.Printf("found %d CodingAgent entities", len(codingAgents))
    
    // 2. For each CodingAgent, create corresponding Agent
    for i, oldAgent := range codingAgents {
        log.Printf("[%d/%d] migrating %s...", i+1, len(codingAgents), 
            oldAgent.Key)
        
        // Determine agent_type from specialization
        agentType := determineAgentType(oldAgent.Properties)
        
        // Create new Agent entity with same properties
        newAgent := &emergent.Agent{
            Name:                  getString(oldAgent.Properties, "name"),
            DisplayName:           getString(oldAgent.Properties, "display_name"),
            Type:                  getString(oldAgent.Properties, "type"),
            AgentType:             agentType,  // NEW field
            Active:                getBool(oldAgent.Properties, "active"),
            Skills:                getStringSlice(oldAgent.Properties, "skills"),
            Specialization:        getString(oldAgent.Properties, "specialization"),
            Instructions:          getString(oldAgent.Properties, "instructions"),
            VelocityPointsPerHour: getFloat64(oldAgent.Properties, "velocity_points_per_hour"),
            Tags:                  getStringSlice(oldAgent.Properties, "tags"),
        }
        
        // Create Agent entity
        agentObj, err := client.CreateAgent(ctx, newAgent)
        if err != nil {
            log.Printf("  WARNING: failed to create Agent: %v", err)
            continue
        }
        log.Printf("  created Agent: %s (id=%s)", agentObj.ID, agentObj.ID)
        
        // 3. Migrate all relationships FROM this CodingAgent
        outgoing, err := client.GetObjectEdges(ctx, oldAgent.ID, &graph.GetObjectEdgesOptions{
            Direction: "outgoing",
        })
        if err != nil {
            log.Printf("  WARNING: failed to get outgoing edges: %v", err)
        } else {
            for _, edge := range outgoing.Outgoing {
                // Skip if target is also a CodingAgent (will be migrated separately)
                targetObj, _ := client.GetObject(ctx, edge.ToID)
                if targetObj != nil && targetObj.Type == "CodingAgent" {
                    continue
                }
                
                // Create same relationship from new Agent
                err := client.CreateRelationship(ctx, agentObj.ID, edge.Label, edge.ToID, nil)
                if err != nil {
                    log.Printf("  WARNING: failed to migrate relationship %s: %v", edge.Label, err)
                }
            }
        }
        
        // 4. Migrate all relationships TO this CodingAgent
        incoming, err := client.GetObjectEdges(ctx, oldAgent.ID, &graph.GetObjectEdgesOptions{
            Direction: "incoming",
        })
        if err != nil {
            log.Printf("  WARNING: failed to get incoming edges: %v", err)
        } else {
            for _, edge := range incoming.Incoming {
                // Skip if source is also a CodingAgent (will be migrated separately)
                sourceObj, _ := client.GetObject(ctx, edge.FromID)
                if sourceObj != nil && sourceObj.Type == "CodingAgent" {
                    continue
                }
                
                // Create same relationship to new Agent
                err := client.CreateRelationship(ctx, edge.FromID, edge.Label, agentObj.ID, nil)
                if err != nil {
                    log.Printf("  WARNING: failed to migrate relationship %s: %v", edge.Label, err)
                }
            }
        }
        
        log.Printf("  migrated %d outgoing + %d incoming relationships", 
            len(outgoing.Outgoing), len(incoming.Incoming))
    }
    
    return nil
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
        return "coding"  // Default to coding
    }
}

// Helper functions to safely extract values
func getString(props map[string]interface{}, key string) string {
    if v, ok := props[key].(string); ok {
        return v
    }
    return ""
}

func getBool(props map[string]interface{}, key string) bool {
    if v, ok := props[key].(bool); ok {
        return v
    }
    return false
}

func getFloat64(props map[string]interface{}, key string) float64 {
    if v, ok := props[key].(float64); ok {
        return v
    }
    return 0
}

func getStringSlice(props map[string]interface{}, key string) []string {
    if v, ok := props[key].([]interface{}); ok {
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
```

**Run Migration**:
```bash
EMERGENT_TOKEN=emt_... go run ./scripts/migrate-agents.go
```

**Output**:
```
fetching all CodingAgent entities...
found 1 CodingAgent entities
[1/1] migrating janitor...
  created Agent: b2bcc109-7de7-48b2-af9b-906ad475e61f (id=...)
  migrated 2 outgoing + 5 incoming relationships
migration complete
```

### Phase 3: Update Code to Use Agent

**Goal**: Change all SpecMCP code to use `Agent` instead of `CodingAgent`

**Files to Update**:
- `internal/emergent/types.go` - Update constant, struct, relationships
- `internal/emergent/entities.go` - Rename functions
- `internal/tools/janitor/janitor.go` - Use Agent
- All other tools that reference CodingAgent

**Code Changes**: Already detailed in `AGENT_REFACTORING_PLAN.md`

### Phase 4: Verification

**Goal**: Ensure migration was successful

**Verification Script**: `scripts/verify-migration.go`

```go
func verifyMigration(ctx context.Context, client *emergent.Client) error {
    // 1. Count CodingAgent entities
    codingAgents, _ := client.ListObjects(ctx, &graph.ListObjectsOptions{
        Type: "CodingAgent",
    })
    
    // 2. Count Agent entities
    agents, _ := client.ListObjects(ctx, &graph.ListObjectsOptions{
        Type: "Agent",
    })
    
    log.Printf("CodingAgent entities: %d", len(codingAgents))
    log.Printf("Agent entities: %d", len(agents))
    
    // 3. Check each Agent has proper agent_type
    for _, agent := range agents {
        agentType := agent.Properties["agent_type"]
        if agentType == nil || agentType == "" {
            return fmt.Errorf("Agent %s missing agent_type", agent.Key)
        }
    }
    
    // 4. Check relationships migrated
    for _, codingAgent := range codingAgents {
        // Find corresponding Agent by name
        name := codingAgent.Properties["name"]
        for _, agent := range agents {
            if agent.Properties["name"] == name {
                // Compare relationship counts
                oldEdges, _ := client.GetObjectEdges(ctx, codingAgent.ID, nil)
                newEdges, _ := client.GetObjectEdges(ctx, agent.ID, nil)
                
                if len(oldEdges.Outgoing) != len(newEdges.Outgoing) {
                    log.Printf("WARNING: %s has different outgoing edge counts: old=%d new=%d",
                        name, len(oldEdges.Outgoing), len(newEdges.Outgoing))
                }
                if len(oldEdges.Incoming) != len(newEdges.Incoming) {
                    log.Printf("WARNING: %s has different incoming edge counts: old=%d new=%d",
                        name, len(oldEdges.Incoming), len(newEdges.Incoming))
                }
            }
        }
    }
    
    log.Println("verification complete")
    return nil
}
```

### Phase 5: Cleanup Old Data (Optional)

**Goal**: Remove old `CodingAgent` entities after confirming everything works

**⚠️ Warning**: Only do this after thorough testing!

```go
func cleanupOldAgents(ctx context.Context, client *emergent.Client) error {
    // List all CodingAgent entities
    codingAgents, err := client.ListObjects(ctx, &graph.ListObjectsOptions{
        Type: "CodingAgent",
    })
    if err != nil {
        return err
    }
    
    log.Printf("deleting %d CodingAgent entities...", len(codingAgents))
    
    for i, agent := range codingAgents {
        // Delete all relationships first
        edges, _ := client.GetObjectEdges(ctx, agent.ID, nil)
        for _, edge := range edges.Outgoing {
            client.DeleteRelationship(ctx, edge.ID)
        }
        for _, edge := range edges.Incoming {
            client.DeleteRelationship(ctx, edge.ID)
        }
        
        // Delete entity
        err := client.DeleteObject(ctx, agent.ID)
        if err != nil {
            log.Printf("WARNING: failed to delete %s: %v", agent.Key, err)
            continue
        }
        
        log.Printf("[%d/%d] deleted %s", i+1, len(codingAgents), agent.Key)
    }
    
    return nil
}
```

### Phase 6: Unassign v1.0.0 Pack (Optional)

**Goal**: Remove old template pack assignment

```bash
# List installed packs
curl http://localhost:3002/api/v1/template-packs/installed \
  -H "Authorization: Bearer $EMERGENT_TOKEN"

# Find v1.0.0 assignment ID, then:
curl -X DELETE http://localhost:3002/api/v1/template-packs/assignments/{assignment-id} \
  -H "Authorization: Bearer $EMERGENT_TOKEN"
```

---

## Dry Run Strategy

### Before Migration: Understand Current State

**Script**: `scripts/pre-migration-report.go`

```go
func generatePreMigrationReport(ctx context.Context, client *emergent.Client) error {
    report := &MigrationReport{
        Timestamp: time.Now(),
    }
    
    // 1. List installed packs
    packs, err := client.GetInstalledPacks(ctx)
    if err != nil {
        return err
    }
    report.InstalledPacks = packs
    
    // 2. Count CodingAgent entities
    codingAgents, _ := client.ListObjects(ctx, &graph.ListObjectsOptions{
        Type: "CodingAgent",
    })
    report.CodingAgentCount = len(codingAgents)
    
    // 3. Count relationships to/from CodingAgents
    totalIncoming := 0
    totalOutgoing := 0
    for _, agent := range codingAgents {
        edges, _ := client.GetObjectEdges(ctx, agent.ID, nil)
        totalIncoming += len(edges.Incoming)
        totalOutgoing += len(edges.Outgoing)
    }
    report.RelationshipCounts = map[string]int{
        "incoming": totalIncoming,
        "outgoing": totalOutgoing,
    }
    
    // 4. List all relationship types that reference CodingAgent
    compiled, _ := client.GetCompiledTypes(ctx)
    affectedRelationships := []string{}
    for _, rt := range compiled.RelationshipTypes {
        if rt.FromType == "CodingAgent" || rt.ToType == "CodingAgent" {
            affectedRelationships = append(affectedRelationships, rt.Name)
        }
    }
    report.AffectedRelationships = affectedRelationships
    
    // 5. Save report
    reportJSON, _ := json.MarshalIndent(report, "", "  ")
    os.WriteFile("pre-migration-report.json", reportJSON, 0644)
    
    log.Println("Pre-migration report saved to pre-migration-report.json")
    return nil
}
```

**Run Before Migration**:
```bash
EMERGENT_TOKEN=emt_... go run ./scripts/pre-migration-report.go
```

**Example Output** (`pre-migration-report.json`):
```json
{
  "timestamp": "2026-02-17T14:00:00Z",
  "installed_packs": [
    {
      "name": "SpecMCP",
      "version": "1.0.0",
      "assignment_id": "abc-123"
    }
  ],
  "coding_agent_count": 1,
  "relationship_counts": {
    "incoming": 5,
    "outgoing": 2
  },
  "affected_relationships": [
    "assigned_to",
    "owned_by",
    "proposed_by"
  ]
}
```

### Dry Run: Simulate Migration

**Script**: `scripts/dry-run-migration.go`

```go
func dryRunMigration(ctx context.Context, client *emergent.Client) error {
    log.Println("=== DRY RUN MODE ===")
    log.Println("No actual changes will be made")
    
    // Simulate Phase 1: Register v2.0.0
    log.Println("\n[Phase 1] Would register SpecMCP v2.0.0")
    log.Println("  - Adds Agent object type")
    log.Println("  - Keeps CodingAgent in v1.0.0")
    log.Println("  - Both types available after assignment")
    
    // Simulate Phase 2: Data migration
    codingAgents, _ := client.ListObjects(ctx, &graph.ListObjectsOptions{
        Type: "CodingAgent",
    })
    
    log.Printf("\n[Phase 2] Would migrate %d CodingAgent entities:", len(codingAgents))
    for _, agent := range codingAgents {
        name := agent.Properties["name"]
        spec := agent.Properties["specialization"]
        agentType := determineAgentType(agent.Properties)
        
        log.Printf("  - %s (specialization=%s) → Agent (agent_type=%s)", 
            name, spec, agentType)
        
        // Count relationships
        edges, _ := client.GetObjectEdges(ctx, agent.ID, nil)
        log.Printf("    Would migrate %d incoming + %d outgoing relationships",
            len(edges.Incoming), len(edges.Outgoing))
    }
    
    // Simulate Phase 3: Code updates
    log.Println("\n[Phase 3] Would update SpecMCP code:")
    log.Println("  - internal/emergent/types.go")
    log.Println("  - internal/emergent/entities.go")
    log.Println("  - internal/tools/janitor/janitor.go")
    log.Println("  - templates/specmcp-pack.json → v2.0.0")
    
    // Simulate Phase 4: Verification
    log.Println("\n[Phase 4] Would verify:")
    log.Printf("  - %d Agent entities created", len(codingAgents))
    log.Println("  - All relationships migrated correctly")
    log.Println("  - All Agent entities have agent_type property")
    
    log.Println("\n=== DRY RUN COMPLETE ===")
    log.Println("To proceed with actual migration:")
    log.Println("  1. Review pre-migration-report.json")
    log.Println("  2. Run: go run ./scripts/seed-v2.go")
    log.Println("  3. Run: go run ./scripts/migrate-agents.go")
    log.Println("  4. Run: go run ./scripts/verify-migration.go")
    log.Println("  5. Deploy updated SpecMCP code")
    log.Println("  6. Run janitor to test Agent functionality")
    
    return nil
}
```

**Run Dry Run**:
```bash
EMERGENT_TOKEN=emt_... go run ./scripts/dry-run-migration.go
```

---

## Rollback Strategy

### If Migration Fails

1. **Keep v1.0.0 assigned** - Don't unassign it until everything is working
2. **Delete bad Agent entities** - If migration created wrong data
3. **Code rollback** - Revert to previous SpecMCP version
4. **Unassign v2.0.0** - Remove new pack assignment if needed

### Rollback Script

```bash
#!/bin/bash
# rollback.sh

echo "Rolling back Agent migration..."

# 1. List and delete Agent entities created during migration
echo "Deleting Agent entities..."
# (Use API to delete)

# 2. Keep CodingAgent entities intact
echo "CodingAgent entities preserved"

# 3. Unassign SpecMCP v2.0.0
echo "Unassigning SpecMCP v2.0.0..."
# (Use API to unassign)

# 4. Redeploy previous SpecMCP version
echo "Redeploying SpecMCP v0.6.3..."
# (Standard deployment)

echo "Rollback complete"
```

---

## Migration Checklist

### Pre-Migration

- [ ] Run pre-migration report
- [ ] Review report for any unexpected data
- [ ] Run dry-run migration
- [ ] Review dry-run output
- [ ] Backup Emergent database (if possible)
- [ ] Test migration on staging environment first

### Migration Steps

- [ ] Create `templates/specmcp-pack-v2.json`
- [ ] Run `go run ./scripts/seed-v2.go` to register v2.0.0
- [ ] Verify both packs are assigned
- [ ] Run `go run ./scripts/migrate-agents.go` to copy data
- [ ] Run `go run ./scripts/verify-migration.go` to check results
- [ ] Update SpecMCP code to use Agent
- [ ] Build and deploy new SpecMCP version
- [ ] Test janitor with new Agent type
- [ ] Run janitor to verify it can create improvements

### Post-Migration

- [ ] Monitor logs for errors
- [ ] Verify janitor creates Agent (not CodingAgent)
- [ ] Verify improvements link to Agent via proposed_by
- [ ] Run full test suite
- [ ] Generate post-migration report
- [ ] (Optional) Clean up old CodingAgent entities
- [ ] (Optional) Unassign v1.0.0 template pack
- [ ] Update documentation

---

## Recommended Approach

### My Recommendation

**Approach**: Parallel Assignment with Gradual Transition

1. **Week 1**: Register v2.0.0, run migration, verify data
2. **Week 2**: Deploy code using Agent, monitor in production
3. **Week 3**: If stable, clean up old CodingAgent data
4. **Week 4**: Unassign v1.0.0 pack

This approach:
- ✅ Allows rollback at any time
- ✅ No data loss risk
- ✅ Can verify each step before proceeding
- ✅ Production-safe with both types available

### Alternative: All-at-Once

If you want faster migration:
1. Create v2.0.0, run migration, deploy code all in one maintenance window
2. Higher risk but faster completion
3. Requires more thorough testing beforehand

---

## Questions for Emergent Team

1. **Template Pack Versioning**: Is there a best practice for major schema changes like renaming entity types?

2. **Type Conflicts**: What happens if two packs define different schemas for the same type name?

3. **Relationship Migration**: Is there a bulk API for migrating relationships, or do we need to do one at a time?

4. **Compiled Types Cache**: After assigning v2.0.0, how quickly do compiled types update? Is there a cache invalidation delay?

5. **Safe Deletion**: Is there a way to "soft delete" CodingAgent entities or mark them as deprecated rather than hard deleting?

---

## Next Steps

Once you approve this strategy, I will:

1. Create the v2.0.0 template pack JSON
2. Create all migration scripts (seed, migrate, verify, dry-run)
3. Run dry-run migration locally
4. Share results for your review
5. Execute migration on mcj-emergent if approved
