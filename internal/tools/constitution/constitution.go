// Package constitution implements the SpecMCP constitution enforcement tools:
// spec_validate_constitution.
package constitution

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk/graph"
	"github.com/emergent-company/specmcp/internal/emergent"
	"github.com/emergent-company/specmcp/internal/mcp"
)

// --- spec_validate_constitution ---

type validateParams struct {
	ChangeID string `json:"change_id"`
}

// ValidateConstitution checks if a change's entities comply with the
// governing constitution's required and forbidden pattern rules.
type ValidateConstitution struct {
	factory *emergent.ClientFactory
}

func NewValidateConstitution(factory *emergent.ClientFactory) *ValidateConstitution {
	return &ValidateConstitution{factory: factory}
}

func (t *ValidateConstitution) Name() string { return "spec_validate_constitution" }
func (t *ValidateConstitution) Description() string {
	return "Validate a change against its governing constitution. Checks that all entities use required patterns and none use forbidden patterns. Returns violations and compliance status."
}
func (t *ValidateConstitution) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "change_id": {
      "type": "string",
      "description": "ID of the change to validate"
    }
  },
  "required": ["change_id"]
}`)
}

func (t *ValidateConstitution) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
	var p validateParams
	if err := json.Unmarshal(params, &p); err != nil {
		return mcp.ErrorResult(fmt.Sprintf("invalid parameters: %v", err)), nil
	}
	if p.ChangeID == "" {
		return mcp.ErrorResult("change_id is required"), nil
	}

	client, err := t.factory.ClientFor(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating client: %w", err)
	}

	// Resolve change to get current version ID
	change, err := client.GetObject(ctx, p.ChangeID)
	if err != nil {
		return mcp.ErrorResult(fmt.Sprintf("change not found: %v", err)), nil
	}

	// Get the constitution governing this change using GetObjectEdges
	// for canonical-aware lookup instead of ListRelationships
	govEdges, err := client.GetObjectEdges(ctx, change.ID, &graph.GetObjectEdgesOptions{
		Types:     []string{emergent.RelGovernedBy},
		Direction: "outgoing",
	})
	if err != nil {
		return nil, fmt.Errorf("looking up constitution: %w", err)
	}
	if len(govEdges.Outgoing) == 0 {
		return mcp.JSONResult(map[string]any{
			"change_id":    change.ID,
			"canonical_id": change.CanonicalID,
			"status":       "no_constitution",
			"message":      "No constitution governs this change. Add one with spec_artifact.",
		})
	}

	constitutionID := govEdges.Outgoing[0].DstID
	constitution, err := client.GetObject(ctx, constitutionID)
	if err != nil {
		return nil, fmt.Errorf("getting constitution: %w", err)
	}

	// Get required patterns
	requiredPatterns, err := t.getRelatedPatterns(ctx, client, constitutionID, emergent.RelRequiresPattern)
	if err != nil {
		return nil, fmt.Errorf("getting required patterns: %w", err)
	}

	// Get forbidden patterns
	forbiddenPatterns, err := t.getRelatedPatterns(ctx, client, constitutionID, emergent.RelForbidsPattern)
	if err != nil {
		return nil, fmt.Errorf("getting forbidden patterns: %w", err)
	}

	// Collect all entities that belong to this change (specs, contexts, components, actions)
	// These are the entity types that can use patterns
	patternEntities, err := t.collectPatternEntities(ctx, client, change.ID)
	if err != nil {
		return nil, fmt.Errorf("collecting entities: %w", err)
	}

	// Check each entity against required and forbidden patterns
	var violations []map[string]any
	var compliant []map[string]any

	for _, entity := range patternEntities {
		// Get patterns used by this entity
		usedPatterns, err := t.getUsedPatterns(ctx, client, entity["id"].(string))
		if err != nil {
			continue
		}
		usedSet := make(map[string]bool)
		for _, pid := range usedPatterns {
			usedSet[pid] = true
		}

		entityResult := map[string]any{
			"id":   entity["id"],
			"type": entity["type"],
			"name": entity["name"],
		}

		entityViolations := make([]map[string]any, 0)

		// Check required patterns
		for pid, pname := range requiredPatterns {
			if !usedSet[pid] {
				entityViolations = append(entityViolations, map[string]any{
					"rule":         "required_pattern_missing",
					"pattern_id":   pid,
					"pattern_name": pname,
					"message":      fmt.Sprintf("Required pattern %q is not applied", pname),
				})
			}
		}

		// Check forbidden patterns
		for pid, pname := range forbiddenPatterns {
			if usedSet[pid] {
				entityViolations = append(entityViolations, map[string]any{
					"rule":         "forbidden_pattern_used",
					"pattern_id":   pid,
					"pattern_name": pname,
					"message":      fmt.Sprintf("Forbidden pattern %q is applied", pname),
				})
			}
		}

		if len(entityViolations) > 0 {
			entityResult["violations"] = entityViolations
			violations = append(violations, entityResult)
		} else {
			compliant = append(compliant, entityResult)
		}
	}

	constitutionName := ""
	if constitution.Key != nil {
		constitutionName = *constitution.Key
	}

	status := "compliant"
	if len(violations) > 0 {
		status = "violations_found"
	}

	return mcp.JSONResult(map[string]any{
		"change_id":    change.ID,
		"canonical_id": change.CanonicalID,
		"constitution": map[string]any{
			"id":                 constitutionID,
			"canonical_id":       constitution.CanonicalID,
			"name":               constitutionName,
			"required_patterns":  len(requiredPatterns),
			"forbidden_patterns": len(forbiddenPatterns),
		},
		"status":           status,
		"violations":       violations,
		"violation_count":  len(violations),
		"compliant":        compliant,
		"compliant_count":  len(compliant),
		"entities_checked": len(patternEntities),
	})
}

// getRelatedPatterns returns a map of pattern ID → name for patterns linked via relType.
// Uses GetObjectEdges for canonical-aware lookup instead of ListRelationships.
func (t *ValidateConstitution) getRelatedPatterns(ctx context.Context, client *emergent.Client, srcID, relType string) (map[string]string, error) {
	edges, err := client.GetObjectEdges(ctx, srcID, &graph.GetObjectEdgesOptions{
		Types:     []string{relType},
		Direction: "outgoing",
	})
	if err != nil {
		return nil, err
	}
	if len(edges.Outgoing) == 0 {
		return make(map[string]string), nil
	}
	// Batch-fetch all pattern objects
	ids := make([]string, len(edges.Outgoing))
	for i, rel := range edges.Outgoing {
		ids[i] = rel.DstID
	}
	objs, err := client.GetObjects(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(objs))
	for _, obj := range objs {
		name := ""
		if obj.Key != nil {
			name = *obj.Key
		}
		// Key by obj.ID (the server-resolved primary ID)
		result[obj.ID] = name
		// Also key by CanonicalID if different, for cross-call consistency
		if obj.CanonicalID != "" && obj.CanonicalID != obj.ID {
			result[obj.CanonicalID] = name
		}
	}
	return result, nil
}

// collectPatternEntities gathers all entities belonging to a change that can use patterns.
// Uses IDSet for canonical-aware self-skip and dedup by CanonicalID.
func (t *ValidateConstitution) collectPatternEntities(ctx context.Context, client *emergent.Client, changeID string) ([]map[string]any, error) {
	// Get all entities reachable from the change within 2 hops
	expanded, err := client.ExpandGraph(ctx, &graph.GraphExpandRequest{
		RootIDs:   []string{changeID},
		Direction: "outgoing",
		MaxDepth:  2,
		MaxNodes:  200,
		MaxEdges:  500,
		RelationshipTypes: []string{
			emergent.RelHasSpec,
			emergent.RelHasTask,
			emergent.RelImplements,
		},
		IncludeRelationshipProperties: false,
	})
	if err != nil {
		return nil, err
	}

	// Filter to entity types that can have patterns
	patternTypes := map[string]bool{
		emergent.TypeContext:     true,
		emergent.TypeUIComponent: true,
		emergent.TypeAction:      true,
		emergent.TypeSpec:        true,
	}

	// Build IDSet for change so self-skip works regardless of ID variant
	changeIDs := emergent.NewIDSet(changeID, "")
	if expanded.Nodes != nil {
		for _, node := range expanded.Nodes {
			if node.ID == changeID || node.CanonicalID == changeID {
				changeIDs = emergent.NewIDSet(node.ID, node.CanonicalID)
				break
			}
		}
	}

	// Dedup by CanonicalID to avoid processing multiple versions of the same entity
	seen := make(map[string]bool)
	entities := make([]map[string]any, 0)
	if expanded.Nodes != nil {
		for _, node := range expanded.Nodes {
			if changeIDs[node.ID] {
				continue
			}
			if !patternTypes[node.Type] {
				continue
			}
			dedupKey := node.CanonicalID
			if dedupKey == "" {
				dedupKey = node.ID
			}
			if seen[dedupKey] {
				continue
			}
			seen[dedupKey] = true
			name := ""
			if node.Key != nil {
				name = *node.Key
			}
			entities = append(entities, map[string]any{
				"id":   node.ID,
				"type": node.Type,
				"name": name,
			})
		}
	}

	return entities, nil
}

// getUsedPatterns returns the IDs of patterns applied to an entity.
// Uses GetObjectEdges for canonical-aware lookup instead of ListRelationships.
// Returns resolved object IDs (from GetObjects) to ensure consistency with
// getRelatedPatterns lookups.
func (t *ValidateConstitution) getUsedPatterns(ctx context.Context, client *emergent.Client, entityID string) ([]string, error) {
	edges, err := client.GetObjectEdges(ctx, entityID, &graph.GetObjectEdgesOptions{
		Types:     []string{emergent.RelUsesPattern},
		Direction: "outgoing",
	})
	if err != nil {
		return nil, err
	}
	if len(edges.Outgoing) == 0 {
		return nil, nil
	}
	// Batch-fetch pattern objects to get their resolved IDs
	relIDs := make([]string, len(edges.Outgoing))
	for i, rel := range edges.Outgoing {
		relIDs[i] = rel.DstID
	}
	objs, err := client.GetObjects(ctx, relIDs)
	if err != nil {
		// Fall back to raw DstIDs if GetObjects fails
		return relIDs, nil
	}
	ids := make([]string, 0, len(objs)*2)
	for _, obj := range objs {
		ids = append(ids, obj.ID)
		if obj.CanonicalID != "" && obj.CanonicalID != obj.ID {
			ids = append(ids, obj.CanonicalID)
		}
	}
	return ids, nil
}
