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
	"github.com/emergent-company/specmcp/internal/config"
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
	CreateProposal     bool   `json:"create_proposal,omitempty"`
	CreateImprovements bool   `json:"create_improvements,omitempty"` // Create Improvement entities grouped by issue type
	Scope              string `json:"scope,omitempty"`               // "all", "changes", "artifacts"
	AutoFix            bool   `json:"auto_fix,omitempty"`
}

type JanitorRun struct {
	factory *emergent.ClientFactory
	logger  *slog.Logger
	cfg     config.JanitorConfig
}

func NewJanitorRun(factory *emergent.ClientFactory, logger *slog.Logger, cfg ...config.JanitorConfig) *JanitorRun {
	jr := &JanitorRun{
		factory: factory,
		logger:  logger,
	}
	if len(cfg) > 0 {
		jr.cfg = cfg[0]
	}
	return jr
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
    "create_improvements": {
      "type": "boolean",
      "description": "If true, create Improvement entities grouped by issue type with subtask Tasks for each specific issue"
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

	// Ensure the janitor Agent exists in the graph
	janitorAgent, err := t.ensureJanitorAgent(ctx, client)
	if err != nil {
		t.logger.Warn("failed to ensure janitor agent exists", "error", err)
		// Continue anyway - the proposal will be created without an author
	}

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

	// Log findings summary for visibility
	t.logFindings(report)

	// Create maintenance proposal if requested and critical issues exist
	var proposalID string
	if p.CreateProposal && report.CriticalIssues > 0 {
		var janitorAgentID string
		if janitorAgent != nil {
			janitorAgentID = janitorAgent.ID
		}
		proposal, err := t.createMaintenanceProposal(ctx, client, report, janitorAgentID)
		if err != nil {
			t.logger.Error("error creating maintenance proposal", "error", err)
		} else {
			proposalID = proposal.ID
			t.logger.Info("created maintenance proposal", "proposal_id", proposalID, "author", "janitor")
		}
	}

	// Create Improvement entities if requested (or if config enables it)
	shouldCreateImprovements := p.CreateImprovements || t.cfg.CreateImprovements
	var improvements []improvementResult
	if shouldCreateImprovements && report.IssuesFound > 0 {
		var janitorAgentID string
		if janitorAgent != nil {
			janitorAgentID = janitorAgent.ID
		}
		thresholds := t.cfg.ImprovementThresholds
		if len(thresholds) == 0 {
			thresholds = []string{"critical", "warning"} // default
		}
		improvements, err = t.createImprovements(ctx, client, report, janitorAgentID, thresholds)
		if err != nil {
			t.logger.Error("error creating improvements", "error", err)
		} else {
			t.logger.Info("created improvements from janitor findings",
				"improvement_count", len(improvements),
				"total_tasks", countImprovementTasks(improvements))
		}
	}

	result := map[string]any{
		"report": report,
	}
	if proposalID != "" {
		result["proposal_id"] = proposalID
		result["message"] = fmt.Sprintf("Found %d critical issues. Maintenance proposal created: %s",
			report.CriticalIssues, proposalID)
	} else if len(improvements) > 0 {
		result["improvements"] = improvements
		result["message"] = fmt.Sprintf("Janitor run complete. Found %d issues (%d critical, %d warnings). Created %d improvements with %d tasks.",
			report.IssuesFound, report.CriticalIssues, report.Warnings,
			len(improvements), countImprovementTasks(improvements))
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
func (t *JanitorRun) createMaintenanceProposal(ctx context.Context, client *emergent.Client, report *Report, janitorAgentID string) (*graph.GraphObject, error) {
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
		"created_by":  "janitor",
		"priority":    "high",
	}

	// Create the proposal
	proposal, err := client.CreateObject(ctx, emergent.TypeProposal, &key, props, nil)
	if err != nil {
		return nil, err
	}

	// Link to janitor agent if provided
	if janitorAgentID != "" {
		_, err = client.CreateRelationship(ctx, emergent.RelProposedBy, proposal.ID, janitorAgentID, nil)
		if err != nil {
			t.logger.Warn("failed to link proposal to janitor agent", "error", err)
			// Continue - proposal is created even if linking fails
		}
	}

	return proposal, nil
}

// improvementResult tracks what was created for a given issue type.
type improvementResult struct {
	ImprovementID string   `json:"improvement_id"`
	IssueType     string   `json:"issue_type"`
	Title         string   `json:"title"`
	IssueCount    int      `json:"issue_count"`
	TaskIDs       []string `json:"task_ids"`
}

// issueTypeConfig maps issue types to human-readable titles, domains, and improvement types.
var issueTypeConfig = map[string]struct {
	title           string
	domain          string
	improvementType string
	priority        string
	effort          string
}{
	"naming_convention":    {"Fix naming convention violations", "infrastructure", "cleanup", "medium", "small"},
	"orphaned_entity":      {"Connect orphaned entities to Changes", "infrastructure", "cleanup", "medium", "medium"},
	"missing_relationship": {"Add missing required relationships", "infrastructure", "tech_debt", "high", "small"},
	"stale_change":         {"Archive or complete stale changes", "infrastructure", "cleanup", "low", "trivial"},
	"empty_change":         {"Add artifacts to empty changes or archive them", "infrastructure", "cleanup", "low", "trivial"},
	"invalid_state":        {"Fix invalid readiness states", "infrastructure", "bug_fix", "critical", "small"},
}

// createImprovements creates or updates Improvement entities grouped by issue type.
// Uses stable keys (janitor-{issueType}) so repeated runs upsert the same Improvement
// rather than creating duplicates. Old tasks are deleted and replaced with current findings.
// Only issues matching the threshold severity levels are included.
func (t *JanitorRun) createImprovements(ctx context.Context, client *emergent.Client, report *Report, janitorAgentID string, thresholds []string) ([]improvementResult, error) {
	// Build threshold set for fast lookup
	thresholdSet := make(map[string]bool, len(thresholds))
	for _, th := range thresholds {
		thresholdSet[th] = true
	}

	// Group issues by type, filtering by severity threshold
	issuesByType := make(map[string][]Issue)
	for _, issue := range report.Issues {
		if thresholdSet[issue.Severity] {
			issuesByType[issue.Type] = append(issuesByType[issue.Type], issue)
		}
	}

	if len(issuesByType) == 0 {
		t.logger.Info("no issues match improvement thresholds", "thresholds", thresholds)
		return nil, nil
	}

	now := time.Now()
	var results []improvementResult

	for issueType, issues := range issuesByType {
		cfg, ok := issueTypeConfig[issueType]
		if !ok {
			// Unknown issue type, use defaults
			cfg = struct {
				title           string
				domain          string
				improvementType string
				priority        string
				effort          string
			}{
				title:           fmt.Sprintf("Fix %s issues", issueType),
				domain:          "infrastructure",
				improvementType: "cleanup",
				priority:        "medium",
				effort:          "medium",
			}
		}

		// Stable key per issue type — no timestamp, so repeated runs upsert the same entity
		key := fmt.Sprintf("janitor-%s", issueType)
		title := fmt.Sprintf("%s (%d issues)", cfg.title, len(issues))
		description := t.buildImprovementDescription(issueType, issues)

		improvementProps := map[string]any{
			"title":       title,
			"description": description,
			"domain":      cfg.domain,
			"type":        cfg.improvementType,
			"effort":      cfg.effort,
			"priority":    cfg.priority,
			"status":      emergent.StatusProposed,
			"proposed_at": now.Format(time.RFC3339),
			"proposed_by": "janitor",
			"tags":        []string{"janitor", "automated", issueType},
		}

		// Check if an active Improvement already exists for this issue type
		existing, err := client.FindByTypeAndKey(ctx, emergent.TypeImprovement, key)
		if err != nil {
			t.logger.Warn("failed to check for existing improvement", "key", key, "error", err)
			// Fall through to create
		}

		var improvement *graph.GraphObject
		isUpdate := false

		if existing != nil {
			// Check if the existing improvement is still active (not completed)
			status, _ := existing.Properties["status"].(string)
			if status != emergent.StatusCompleted {
				// Update the existing improvement with fresh findings
				improvement, err = client.UpdateObject(ctx, existing.ID, improvementProps, nil)
				if err != nil {
					t.logger.Error("failed to update existing improvement", "id", existing.ID, "error", err)
					continue
				}
				isUpdate = true

				// Delete old tasks linked to this improvement
				t.deleteLinkedTasks(ctx, client, existing.ID)
			}
		}

		if improvement == nil {
			// Create new Improvement (either no existing, or existing was completed)
			improvement, err = client.CreateObject(ctx, emergent.TypeImprovement, &key, improvementProps, nil)
			if err != nil {
				t.logger.Error("failed to create improvement", "issue_type", issueType, "error", err)
				continue
			}

			// Link to janitor agent (only on first creation)
			if janitorAgentID != "" {
				if _, err := client.CreateRelationship(ctx, emergent.RelProposedBy, improvement.ID, janitorAgentID, nil); err != nil {
					t.logger.Warn("failed to link improvement to janitor agent", "improvement_id", improvement.ID, "error", err)
				}
			}
		}

		// Create subtask Tasks for each specific issue
		result := improvementResult{
			ImprovementID: improvement.ID,
			IssueType:     issueType,
			Title:         title,
			IssueCount:    len(issues),
			TaskIDs:       make([]string, 0, len(issues)),
		}

		for i, issue := range issues {
			taskNumber := fmt.Sprintf("J%d", i+1)
			taskKey := fmt.Sprintf("%s-task-%d", key, i+1)
			taskProps := map[string]any{
				"number":      taskNumber,
				"description": issue.Description,
				"task_type":   "maintenance",
				"status":      emergent.StatusPending,
				"tags":        []string{"janitor", "automated", issueType},
			}

			// Add suggestion as verification notes if present
			if issue.Suggestion != "" {
				taskProps["verification_notes"] = issue.Suggestion
			}

			task, err := client.CreateObject(ctx, emergent.TypeTask, &taskKey, taskProps, nil)
			if err != nil {
				t.logger.Error("failed to create task for improvement",
					"improvement_id", improvement.ID,
					"issue_index", i,
					"error", err)
				continue
			}

			// Link task to improvement
			if _, err := client.CreateRelationship(ctx, emergent.RelHasTask, improvement.ID, task.ID, nil); err != nil {
				t.logger.Warn("failed to link task to improvement",
					"task_id", task.ID,
					"improvement_id", improvement.ID,
					"error", err)
			}

			// Link task to the affected entity if we have an ID
			if issue.EntityID != "" {
				if _, err := client.CreateRelationship(ctx, emergent.RelAffectsEntity, task.ID, issue.EntityID, nil); err != nil {
					t.logger.Debug("could not link task to affected entity",
						"task_id", task.ID,
						"entity_id", issue.EntityID,
						"error", err)
				}
			}

			result.TaskIDs = append(result.TaskIDs, task.ID)
		}

		action := "created"
		if isUpdate {
			action = "updated"
		}
		results = append(results, result)
		t.logger.Info(action+" improvement with tasks",
			"improvement_id", improvement.ID,
			"issue_type", issueType,
			"task_count", len(result.TaskIDs))
	}

	return results, nil
}

// deleteLinkedTasks finds and deletes all Tasks linked to an Improvement via has_task.
func (t *JanitorRun) deleteLinkedTasks(ctx context.Context, client *emergent.Client, improvementID string) {
	edges, err := client.GetObjectEdges(ctx, improvementID, &graph.GetObjectEdgesOptions{
		Type:      emergent.RelHasTask,
		Direction: "outgoing",
	})
	if err != nil {
		t.logger.Warn("failed to get tasks for improvement", "improvement_id", improvementID, "error", err)
		return
	}

	deleted := 0
	for _, rel := range edges.Outgoing {
		// Delete the task object
		if err := client.DeleteObject(ctx, rel.DstID); err != nil {
			t.logger.Warn("failed to delete old task", "task_id", rel.DstID, "error", err)
			continue
		}
		deleted++
	}

	if deleted > 0 {
		t.logger.Info("deleted old tasks from improvement",
			"improvement_id", improvementID,
			"deleted_count", deleted)
	}
}

// buildImprovementDescription builds a markdown description for an improvement.
func (t *JanitorRun) buildImprovementDescription(issueType string, issues []Issue) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s Issues\n\n", issueType))
	sb.WriteString(fmt.Sprintf("The janitor detected %d issues of type `%s`.\n\n", len(issues), issueType))
	sb.WriteString("### Issues\n\n")

	for i, issue := range issues {
		sb.WriteString(fmt.Sprintf("%d. **%s** `%s` (severity: %s)\n",
			i+1, issue.EntityType, safeKey(nil), issue.Severity))
		sb.WriteString(fmt.Sprintf("   - %s\n", issue.Description))
		if issue.Suggestion != "" {
			sb.WriteString(fmt.Sprintf("   - Suggestion: %s\n", issue.Suggestion))
		}
	}

	return sb.String()
}

// countImprovementTasks counts the total number of tasks across all improvements.
func countImprovementTasks(improvements []improvementResult) int {
	total := 0
	for _, imp := range improvements {
		total += len(imp.TaskIDs)
	}
	return total
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

// logFindings logs a summary of janitor findings for visibility.
func (t *JanitorRun) logFindings(report *Report) {
	// Group issues by type for easier understanding
	issuesByType := make(map[string]int)
	issuesBySeverity := make(map[string]int)

	for _, issue := range report.Issues {
		issuesByType[issue.Type]++
		issuesBySeverity[issue.Severity]++
	}

	// Log overall summary
	t.logger.Info("janitor run complete",
		"total_issues", report.IssuesFound,
		"critical", report.CriticalIssues,
		"warnings", report.Warnings,
		"entity_count", len(report.EntityCounts))

	// Log breakdown by issue type if any issues found
	if report.IssuesFound > 0 {
		t.logger.Info("janitor findings by type",
			"naming_convention", issuesByType["naming_convention"],
			"orphaned_entity", issuesByType["orphaned_entity"],
			"missing_relationship", issuesByType["missing_relationship"],
			"stale_change", issuesByType["stale_change"],
			"empty_change", issuesByType["empty_change"],
			"invalid_state", issuesByType["invalid_state"])
	}

	// If there are critical issues, log them individually
	if report.CriticalIssues > 0 {
		t.logger.Warn("critical issues detected",
			"count", report.CriticalIssues)
		for _, issue := range report.Issues {
			if issue.Severity == "critical" {
				t.logger.Warn("critical issue",
					"type", issue.Type,
					"entity_type", issue.EntityType,
					"entity_id", issue.EntityID,
					"description", issue.Description)
			}
		}
	}
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

// ensureJanitorAgent gets or creates the janitor Agent entity.
func (t *JanitorRun) ensureJanitorAgent(ctx context.Context, client *emergent.Client) (*emergent.Agent, error) {
	agent := &emergent.Agent{
		Name:           "janitor",
		DisplayName:    "Janitor Agent",
		Type:           "ai",
		AgentType:      "maintenance",
		Active:         true,
		Specialization: "maintenance",
		Skills:         []string{"validation", "compliance", "cleanup"},
		Instructions: `The janitor agent maintains project health by:
- Verifying artifact compliance with naming conventions
- Identifying orphaned or disconnected entities
- Detecting invalid state transitions
- Flagging incomplete artifact hierarchies
- Finding stale or abandoned changes

The janitor creates maintenance proposals when critical issues are found.`,
		Tags: []string{"system", "automation", "maintenance"},
	}

	return client.GetOrCreateAgent(ctx, agent)
}
