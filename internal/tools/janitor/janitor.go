// Package janitor implements the SpecMCP janitor agent.
// The janitor validates artifact compliance, verifies relationships,
// and creates maintenance proposals when issues are found.
package janitor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk/graph"
	"github.com/emergent-company/specmcp/internal/emergent"
	"github.com/emergent-company/specmcp/internal/mcp"
)

// Issue represents a detected problem in the knowledge graph.
type Issue struct {
	Severity    string `json:"severity"` // critical, warning, info
	Type        string `json:"type"`     // missing_relationship, invalid_state, orphaned_entity, etc.
	EntityID    string `json:"entity_id"`
	EntityType  string `json:"entity_type"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion,omitempty"`
}

// Report summarizes the janitor's findings.
type Report struct {
	Timestamp      string         `json:"timestamp"`
	EntityCounts   map[string]int `json:"entity_counts"`
	IssuesFound    int            `json:"issues_found"`
	CriticalIssues int            `json:"critical_issues"`
	Warnings       int            `json:"warnings"`
	Issues         []Issue        `json:"issues"`
	Summary        string         `json:"summary"`
}

// --- spec_janitor_run ---

type janitorRunParams struct {
	CreateProposal bool   `json:"create_proposal,omitempty"`
	Scope          string `json:"scope,omitempty"` // "all", "changes", "artifacts"
	AutoFix        bool   `json:"auto_fix,omitempty"`
}

type JanitorRun struct {
	factory *emergent.ClientFactory
	logger  *slog.Logger
}

func NewJanitorRun(factory *emergent.ClientFactory, logger *slog.Logger) *JanitorRun {
	return &JanitorRun{
		factory: factory,
		logger:  logger,
	}
}

func (t *JanitorRun) Name() string { return "spec_janitor_run" }

func (t *JanitorRun) Description() string {
	return `Run the janitor agent to verify artifact compliance and project health.
The janitor checks for:
- Missing or invalid relationships between artifacts
- Orphaned entities (no incoming/outgoing relationships)
- Invalid state transitions (e.g., ready artifacts with draft dependencies)
- Naming convention violations (should use kebab-case)
- Incomplete artifact hierarchies (specs without requirements, etc.)
- Stale or abandoned changes

If serious issues are found and create_proposal=true, a maintenance proposal is created.`
}

func (t *JanitorRun) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "create_proposal": {
      "type": "boolean",
      "description": "If true and critical issues are found, create a maintenance proposal"
    },
    "scope": {
      "type": "string",
      "enum": ["all", "changes", "artifacts", "relationships"],
      "description": "Scope of verification (default: all)"
    },
    "auto_fix": {
      "type": "boolean",
      "description": "Automatically fix minor issues (naming, etc.)"
    }
  }
}`)
}

func (t *JanitorRun) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
	var p janitorRunParams
	if err := json.Unmarshal(params, &p); err != nil {
		return mcp.ErrorResult(fmt.Sprintf("invalid parameters: %v", err)), nil
	}

	client, err := t.factory.ClientFor(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating client: %w", err)
	}

	if p.Scope == "" {
		p.Scope = "all"
	}

	t.logger.Info("starting janitor run", "scope", p.Scope, "create_proposal", p.CreateProposal)

	report := &Report{
		Timestamp:    time.Now().Format(time.RFC3339),
		EntityCounts: make(map[string]int),
		Issues:       make([]Issue, 0),
	}

	// Run verification checks based on scope
	if p.Scope == "all" || p.Scope == "changes" {
		if err := t.verifyChanges(ctx, client, report); err != nil {
			t.logger.Error("error verifying changes", "error", err)
		}
	}

	if p.Scope == "all" || p.Scope == "artifacts" {
		if err := t.verifyArtifacts(ctx, client, report); err != nil {
			t.logger.Error("error verifying artifacts", "error", err)
		}
	}

	if p.Scope == "all" || p.Scope == "relationships" {
		if err := t.verifyRelationships(ctx, client, report); err != nil {
			t.logger.Error("error verifying relationships", "error", err)
		}
	}

	// Count issues by severity
	for _, issue := range report.Issues {
		switch issue.Severity {
		case "critical":
			report.CriticalIssues++
		case "warning":
			report.Warnings++
		}
	}
	report.IssuesFound = len(report.Issues)

	// Generate summary
	report.Summary = t.generateSummary(report)

	// Create maintenance proposal if requested and critical issues exist
	var proposalID string
	if p.CreateProposal && report.CriticalIssues > 0 {
		proposal, err := t.createMaintenanceProposal(ctx, client, report)
		if err != nil {
			t.logger.Error("error creating maintenance proposal", "error", err)
		} else {
			proposalID = proposal.ID
			t.logger.Info("created maintenance proposal", "proposal_id", proposalID)
		}
	}

	result := map[string]any{
		"report": report,
	}
	if proposalID != "" {
		result["proposal_id"] = proposalID
		result["message"] = fmt.Sprintf("Found %d critical issues. Maintenance proposal created: %s",
			report.CriticalIssues, proposalID)
	} else {
		result["message"] = fmt.Sprintf("Janitor run complete. Found %d issues (%d critical, %d warnings)",
			report.IssuesFound, report.CriticalIssues, report.Warnings)
	}

	return mcp.JSONResult(result)
}

// verifyChanges checks all Change entities for issues.
func (t *JanitorRun) verifyChanges(ctx context.Context, client *emergent.Client, report *Report) error {
	changes, err := client.ListObjects(ctx, &graph.ListObjectsOptions{
		Type: emergent.TypeChange,
	})
	if err != nil {
		return fmt.Errorf("listing changes: %w", err)
	}

	report.EntityCounts[emergent.TypeChange] = len(changes)

	for _, ch := range changes {
		// Check naming conventions
		if ch.Key != nil && !isKebabCase(*ch.Key) {
			report.Issues = append(report.Issues, Issue{
				Severity:    "warning",
				Type:        "naming_convention",
				EntityID:    ch.ID,
				EntityType:  emergent.TypeChange,
				Description: fmt.Sprintf("Change key '%s' should use kebab-case", *ch.Key),
				Suggestion:  fmt.Sprintf("Rename to: %s", toKebabCase(*ch.Key)),
			})
		}

		// Check for stale changes (created more than 30 days ago with no updates)
		if ch.Properties != nil {
			if createdAt, ok := ch.Properties["created_at"].(string); ok {
				created, err := time.Parse(time.RFC3339, createdAt)
				if err == nil && time.Since(created) > 30*24*time.Hour {
					status := ch.Properties["status"]
					if status == "draft" || status == "proposed" {
						report.Issues = append(report.Issues, Issue{
							Severity:   "info",
							Type:       "stale_change",
							EntityID:   ch.ID,
							EntityType: emergent.TypeChange,
							Description: fmt.Sprintf("Change '%s' has been in '%s' state for over 30 days",
								safeKey(ch.Key), status),
							Suggestion: "Consider archiving or completing this change",
						})
					}
				}
			}
		}

		// Check for changes with no artifacts
		edgesResp, err := client.GetObjectEdges(ctx, ch.ID, &graph.GetObjectEdgesOptions{
			Direction: "outgoing",
		})
		if err == nil && len(edgesResp.Outgoing) == 0 {
			report.Issues = append(report.Issues, Issue{
				Severity:    "warning",
				Type:        "empty_change",
				EntityID:    ch.ID,
				EntityType:  emergent.TypeChange,
				Description: fmt.Sprintf("Change '%s' has no associated artifacts", safeKey(ch.Key)),
				Suggestion:  "Add specs, requirements, or designs to this change",
			})
		}
	}

	return nil
}

// verifyArtifacts checks workflow artifacts (Proposals, Specs, Requirements, etc.).
func (t *JanitorRun) verifyArtifacts(ctx context.Context, client *emergent.Client, report *Report) error {
	artifactTypes := []string{
		emergent.TypeProposal,
		emergent.TypeSpec,
		emergent.TypeRequirement,
		emergent.TypeScenario,
		emergent.TypeDesign,
	}

	for _, artType := range artifactTypes {
		artifacts, err := client.ListObjects(ctx, &graph.ListObjectsOptions{
			Type: artType,
		})
		if err != nil {
			t.logger.Error("error listing artifacts", "type", artType, "error", err)
			continue
		}

		report.EntityCounts[artType] = len(artifacts)

		for _, art := range artifacts {
			// Check naming conventions
			if art.Key != nil && !isKebabCase(*art.Key) {
				report.Issues = append(report.Issues, Issue{
					Severity:    "warning",
					Type:        "naming_convention",
					EntityID:    art.ID,
					EntityType:  artType,
					Description: fmt.Sprintf("%s key '%s' should use kebab-case", artType, *art.Key),
					Suggestion:  fmt.Sprintf("Rename to: %s", toKebabCase(*art.Key)),
				})
			}

			// Check readiness cascade for specs and requirements
			if artType == emergent.TypeSpec || artType == emergent.TypeRequirement {
				if err := t.verifyReadinessCascade(ctx, client, art, report); err != nil {
					t.logger.Error("error verifying readiness cascade", "entity_id", art.ID, "error", err)
				}
			}

			// Check for orphaned artifacts (no parent change)
			if err := t.checkOrphaned(ctx, client, art, report); err != nil {
				t.logger.Error("error checking orphaned status", "entity_id", art.ID, "error", err)
			}
		}
	}

	return nil
}

// verifyRelationships checks for broken or missing relationships.
func (t *JanitorRun) verifyRelationships(ctx context.Context, client *emergent.Client, report *Report) error {
	// Check that all Specs have at least one Requirement
	specs, err := client.ListObjects(ctx, &graph.ListObjectsOptions{
		Type: emergent.TypeSpec,
	})
	if err != nil {
		return fmt.Errorf("listing specs: %w", err)
	}

	for _, spec := range specs {
		edgesResp, err := client.GetObjectEdges(ctx, spec.ID, &graph.GetObjectEdgesOptions{
			Direction: "outgoing",
			Type:      emergent.RelHasRequirement,
		})
		if err == nil && len(edgesResp.Outgoing) == 0 {
			report.Issues = append(report.Issues, Issue{
				Severity:    "critical",
				Type:        "missing_relationship",
				EntityID:    spec.ID,
				EntityType:  emergent.TypeSpec,
				Description: fmt.Sprintf("Spec '%s' has no requirements", safeKey(spec.Key)),
				Suggestion:  "Add at least one requirement to this spec using spec_artifact",
			})
		}
	}

	// Check that all Requirements have at least one Scenario
	reqs, err := client.ListObjects(ctx, &graph.ListObjectsOptions{
		Type: emergent.TypeRequirement,
	})
	if err != nil {
		return fmt.Errorf("listing requirements: %w", err)
	}

	for _, req := range reqs {
		edgesResp, err := client.GetObjectEdges(ctx, req.ID, &graph.GetObjectEdgesOptions{
			Direction: "outgoing",
			Type:      emergent.RelHasScenario,
		})
		if err == nil && len(edgesResp.Outgoing) == 0 {
			report.Issues = append(report.Issues, Issue{
				Severity:    "warning",
				Type:        "missing_relationship",
				EntityID:    req.ID,
				EntityType:  emergent.TypeRequirement,
				Description: fmt.Sprintf("Requirement '%s' has no scenarios", safeKey(req.Key)),
				Suggestion:  "Add at least one scenario to validate this requirement",
			})
		}
	}

	return nil
}

// verifyReadinessCascade checks that ready artifacts have ready dependencies.
func (t *JanitorRun) verifyReadinessCascade(ctx context.Context, client *emergent.Client, obj *graph.GraphObject, report *Report) error {
	if obj.Properties == nil {
		return nil
	}

	status, ok := obj.Properties["status"].(string)
	if !ok || status != "ready" {
		return nil
	}

	// Get all outgoing relationships to dependencies
	edgesResp, err := client.GetObjectEdges(ctx, obj.ID, &graph.GetObjectEdgesOptions{
		Direction: "outgoing",
	})
	if err != nil {
		return fmt.Errorf("getting edges: %w", err)
	}

	for _, edge := range edgesResp.Outgoing {
		// Fetch the target entity
		target, err := client.GetObject(ctx, edge.DstID)
		if err != nil {
			continue
		}

		if target.Properties != nil {
			targetStatus, ok := target.Properties["status"].(string)
			if ok && targetStatus != "ready" {
				report.Issues = append(report.Issues, Issue{
					Severity:   "critical",
					Type:       "invalid_state",
					EntityID:   obj.ID,
					EntityType: obj.Type,
					Description: fmt.Sprintf("%s '%s' is marked ready but depends on %s '%s' which is '%s'",
						obj.Type, safeKey(obj.Key), target.Type, safeKey(target.Key), targetStatus),
					Suggestion: fmt.Sprintf("Either mark the dependency ready or mark this %s as draft", obj.Type),
				})
			}
		}
	}

	return nil
}

// checkOrphaned verifies that artifacts are connected to a parent Change.
func (t *JanitorRun) checkOrphaned(ctx context.Context, client *emergent.Client, obj *graph.GraphObject, report *Report) error {
	edgesResp, err := client.GetObjectEdges(ctx, obj.ID, &graph.GetObjectEdgesOptions{
		Direction: "incoming",
	})
	if err != nil {
		return fmt.Errorf("getting incoming edges: %w", err)
	}

	// Check if any incoming edge comes from a Change
	hasChangeParent := false
	for _, edge := range edgesResp.Incoming {
		source, err := client.GetObject(ctx, edge.SrcID)
		if err == nil && source.Type == emergent.TypeChange {
			hasChangeParent = true
			break
		}
	}

	if !hasChangeParent {
		report.Issues = append(report.Issues, Issue{
			Severity:    "warning",
			Type:        "orphaned_entity",
			EntityID:    obj.ID,
			EntityType:  obj.Type,
			Description: fmt.Sprintf("%s '%s' is not connected to any Change", obj.Type, safeKey(obj.Key)),
			Suggestion:  "Associate this artifact with a Change or archive it",
		})
	}

	return nil
}

// createMaintenanceProposal creates a proposal for fixing critical issues.
func (t *JanitorRun) createMaintenanceProposal(ctx context.Context, client *emergent.Client, report *Report) (*graph.GraphObject, error) {
	// Build description from critical issues
	var criticalIssues []string
	for _, issue := range report.Issues {
		if issue.Severity == "critical" {
			criticalIssues = append(criticalIssues, fmt.Sprintf("- %s", issue.Description))
		}
	}

	description := fmt.Sprintf(`# Maintenance Required

The janitor agent detected %d critical issues that need attention:

%s

## Recommended Actions
Review each issue and apply the suggested fixes. Use the janitor report for detailed information.

## Impact
Fixing these issues will improve the consistency and reliability of the knowledge graph.
`,
		report.CriticalIssues,
		strings.Join(criticalIssues, "\n"))

	key := fmt.Sprintf("maintenance-%s", time.Now().Format("20060102-150405"))
	props := map[string]any{
		"description": description,
		"status":      "proposed",
		"created_at":  time.Now().Format(time.RFC3339),
		"created_by":  "janitor-agent",
		"priority":    "high",
	}

	return client.CreateObject(ctx, emergent.TypeProposal, &key, props, nil)
}

// generateSummary creates a human-readable summary of the report.
func (t *JanitorRun) generateSummary(report *Report) string {
	if report.IssuesFound == 0 {
		return "No issues found. The knowledge graph is healthy."
	}

	return fmt.Sprintf("Found %d issues: %d critical, %d warnings, %d informational",
		report.IssuesFound,
		report.CriticalIssues,
		report.Warnings,
		report.IssuesFound-report.CriticalIssues-report.Warnings)
}

// Utility functions

func isKebabCase(s string) bool {
	if s == "" {
		return true
	}
	// Kebab case: lowercase letters, numbers, and hyphens
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return false
		}
	}
	return true
}

func toKebabCase(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, " ", "-")
	return s
}

func safeKey(key *string) string {
	if key == nil {
		return "<unnamed>"
	}
	return *key
}
