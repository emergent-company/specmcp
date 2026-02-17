# Agent Refactoring & Janitor Improvements Plan

**Created**: 2026-02-17  
**Completed**: 2026-02-17  
**Status**: ✅ Complete  
**Version**: v0.6.4 (deployed to production)

## Completion Summary

All objectives achieved:

1. ✅ **Agent Refactoring**: CodingAgent → Agent with `agent_type` field
   - Updated entity types, relationships, and all tool code
   - Migrated 2 production entities (janitor, test-agent)
   - Old entities marked inactive with `active: false`

2. ✅ **Template Pack V2**: Registered and deployed to production
   - Pack ID: `b6695ed5-c70c-4064-b320-ec11e8411685`
   - SDK upgraded: v0.13.0 → v0.14.3
   - Format conversion: object → array format

3. ✅ **Search Filter Fix**: Inactive entities excluded from search results
   - Fixed FTS to use defaultSearchTypes when user doesn't specify types
   - Added `isActive()` filter for both FTS and fallback search
   - Verified old CodingAgent entities no longer appear in searches

4. ✅ **Documentation**: All 13 docs updated with Agent terminology

5. ✅ **Production Deployment**: Service running with v0.6.4
   - Binary deployed to `/usr/bin/specmcp` on mcj-emergent
   - Service verified healthy on port 21452

**Git Commits**:
- c1445b1: Refactor CodingAgent → Agent with agent_type
- 82502de: Update documentation
- 2992e7f: Filter inactive entities from search results

---

## Original Plan Overview

Refactor `CodingAgent` → `Agent` to support multiple agent types (coding, maintenance, research, etc.) and enhance janitor to create actionable Improvement entities with subtasks for all warnings found.

---

## Current State Analysis

### Existing Schema

**Entity Types:**
- ✅ `CodingAgent` - Needs rename to `Agent`
- ✅ `Improvement` - Already exists with good schema
- ✅ `Task` - Already exists

**Relationships:**
- ✅ `has_task` - Improvement → Task (or Change → Task)
- ✅ `has_subtask` - Task → Task
- ✅ `proposed_by` - MaintenanceIssue → CodingAgent (needs to support Improvement too)
- ✅ `assigned_to` - Task → CodingAgent (will become Task → Agent)
- ✅ `owned_by` - Entity → CodingAgent (will become Entity → Agent)

### Current Janitor Behavior

1. Runs checks on artifacts, relationships, naming conventions
2. Reports 51 warnings:
   - 10 naming convention issues
   - 40 orphaned entities
   - 1 missing relationship
3. Only creates proposals for critical issues (not warnings)
4. Logs findings to systemd journal

---

## Phase 1: Rename CodingAgent → Agent

### 1.1 Update Entity Type Definition

**File**: `internal/emergent/types.go`

```go
// OLD:
const (
    TypeCodingAgent = "CodingAgent"
)

type CodingAgent struct {
    ID                     string   `json:"id,omitempty"`
    Name                   string   `json:"name"`
    DisplayName            string   `json:"display_name,omitempty"`
    Type                   string   `json:"type"` // human | ai
    Active                 bool     `json:"active"`
    Skills                 []string `json:"skills,omitempty"`
    Specialization         string   `json:"specialization,omitempty"`
    Instructions           string   `json:"instructions,omitempty"`
    VelocityPointsPerHour  float64  `json:"velocity_points_per_hour,omitempty"`
    Tags                   []string `json:"tags,omitempty"`
}

// NEW:
const (
    TypeAgent = "Agent"
)

type Agent struct {
    ID                     string   `json:"id,omitempty"`
    Name                   string   `json:"name"`
    DisplayName            string   `json:"display_name,omitempty"`
    Type                   string   `json:"type"`           // human | ai
    AgentType              string   `json:"agent_type"`     // coding | maintenance | research | testing | deployment | analysis
    Active                 bool     `json:"active"`
    Skills                 []string `json:"skills,omitempty"`
    Specialization         string   `json:"specialization,omitempty"` // frontend | backend | fullstack | maintenance | research | testing
    Instructions           string   `json:"instructions,omitempty"`
    VelocityPointsPerHour  float64  `json:"velocity_points_per_hour,omitempty"`
    Tags                   []string `json:"tags,omitempty"`
}
```

### 1.2 Update Relationship Constants

**File**: `internal/emergent/types.go`

```go
// Update comments:
RelAssignedTo   = "assigned_to"   // Task → Agent (was CodingAgent)
RelOwnedBy      = "owned_by"      // Entity → Agent (was CodingAgent)
RelProposedBy   = "proposed_by"   // MaintenanceIssue|Improvement → Agent (was CodingAgent)
```

### 1.3 Update Entity Functions

**File**: `internal/emergent/entities.go`

- Rename: `CreateCodingAgent()` → `CreateAgent()`
- Rename: `GetCodingAgent()` → `GetAgent()`
- Rename: `UpdateCodingAgent()` → `UpdateAgent()`
- Rename: `GetOrCreateCodingAgent()` → `GetOrCreateAgent()`

### 1.4 Update Janitor Code

**File**: `internal/tools/janitor/janitor.go`

```go
// Change from:
func (t *JanitorRun) ensureJanitorAgent(ctx context.Context, client *emergent.Client) (*emergent.CodingAgent, error) {
    return emergent.GetOrCreateCodingAgent(ctx, client, &emergent.CodingAgent{
        Name:           "janitor",
        DisplayName:    "Janitor Maintenance Agent",
        Type:           "ai",
        Specialization: "maintenance",
        // ...
    })
}

// Change to:
func (t *JanitorRun) ensureJanitorAgent(ctx context.Context, client *emergent.Client) (*emergent.Agent, error) {
    return emergent.GetOrCreateAgent(ctx, client, &emergent.Agent{
        Name:           "janitor",
        DisplayName:    "Janitor Maintenance Agent",
        Type:           "ai",
        AgentType:      "maintenance",
        Specialization: "maintenance",
        // ...
    })
}
```

### 1.5 Update Template Pack

**File**: `templates/specmcp-pack.json`

- Change entity type from `CodingAgent` to `Agent`
- Add `agent_type` property to schema
- Update all relationship descriptions

### 1.6 Migration Script

Need to migrate existing CodingAgent entities to Agent:

**File**: `scripts/migrate-agents.go`

```go
// Pseudo-code:
// 1. List all CodingAgent entities
// 2. For each CodingAgent:
//    - Create new Agent with same properties
//    - Set agent_type based on specialization:
//      - "maintenance" → agent_type: "maintenance"
//      - "frontend", "backend", "fullstack" → agent_type: "coding"
//    - Migrate all relationships
// 3. Delete old CodingAgent entities
```

---

## Phase 2: Enhance Task Schema

### 2.1 Add New Task Properties

**File**: `internal/emergent/types.go`

```go
type Task struct {
    // ... existing fields ...
    
    // NEW fields for janitor-created tasks:
    Severity        string `json:"severity,omitempty"`         // critical | warning | suggestion
    TaskCategory    string `json:"task_category,omitempty"`    // bug | feature | improvement | maintenance | refactor
    AutoGenerated   bool   `json:"auto_generated,omitempty"`   // true if created by janitor or automation
    FixComplexity   string `json:"fix_complexity,omitempty"`   // trivial | simple | moderate | complex
    
    // ... existing fields ...
}
```

---

## Phase 3: Janitor Creates Improvements with Subtasks

### 3.1 Issue Categorization Logic

Group janitor issues by type:

```go
type IssueCategory struct {
    Type        string   // "naming_convention", "orphaned_entity", "missing_relationship"
    Title       string   // "Fix Naming Convention Violations"
    Description string   // Detailed description
    Domain      string   // "documentation", "maintenance", "data"
    ImprovementType string // "cleanup", "tech_debt"
    Effort      string   // "trivial", "small", "medium"
    Priority    string   // "low", "medium", "high"
    Issues      []Issue  // All issues of this type
}
```

**Categorization:**

1. **Naming Convention Issues** (10 issues)
   - Title: "Fix Naming Convention Violations"
   - Domain: "documentation"
   - Type: "cleanup"
   - Effort: "trivial"
   - Priority: "low"

2. **Orphaned Entities** (40 issues)
   - Title: "Link Orphaned Artifacts to Changes"
   - Domain: "maintenance"
   - Type: "tech_debt"
   - Effort: "small"
   - Priority: "medium"

3. **Missing Relationships** (1 issue)
   - Title: "Add Missing Artifact Relationships"
   - Domain: "maintenance"
   - Type: "cleanup"
   - Effort: "small"
   - Priority: "medium"

### 3.2 Improvement Creation Logic

**File**: `internal/tools/janitor/janitor.go`

Add new method:

```go
func (t *JanitorRun) createImprovementsForWarnings(
    ctx context.Context,
    client *emergent.Client,
    report *Report,
    janitorAgentID string,
) ([]string, error) {
    
    // 1. Group issues by category
    categories := t.categorizeIssues(report.Issues)
    
    improvementIDs := []string{}
    
    // 2. For each category, create an Improvement
    for _, category := range categories {
        improvement := &emergent.Improvement{
            Title:       category.Title,
            Description: category.Description,
            Domain:      category.Domain,
            Type:        category.ImprovementType,
            Effort:      category.Effort,
            Priority:    category.Priority,
            Status:      emergent.StatusProposed,
            ProposedBy:  "janitor", // Agent name
            Tags:        []string{"janitor", "auto-generated", "maintenance"},
        }
        
        // Create improvement
        improvementObj, err := client.CreateImprovement(ctx, improvement)
        if err != nil {
            return nil, fmt.Errorf("creating improvement: %w", err)
        }
        improvementIDs = append(improvementIDs, improvementObj.ID)
        
        // 3. Create proposed_by relationship
        err = client.CreateRelationship(ctx, improvementObj.ID, 
            emergent.RelProposedBy, janitorAgentID, nil)
        if err != nil {
            t.logger.Warn("failed to link improvement to janitor agent", "error", err)
        }
        
        // 4. Create a Task for each issue in this category
        for i, issue := range category.Issues {
            task := &emergent.Task{
                Number:          fmt.Sprintf("T%d", i+1),
                Description:     issue.Description + "\n\nSuggestion: " + issue.Suggestion,
                TaskType:        "maintenance",
                Status:          emergent.StatusPending,
                ComplexityPoints: t.estimateComplexity(issue),
                Severity:        issue.Severity,
                TaskCategory:    "improvement",
                AutoGenerated:   true,
                FixComplexity:   t.getFixComplexity(issue),
                Tags:            []string{"janitor", "auto-generated", issue.Type},
            }
            
            // Create task
            taskObj, err := client.CreateTask(ctx, task)
            if err != nil {
                t.logger.Warn("failed to create task for issue", 
                    "issue_type", issue.Type, "error", err)
                continue
            }
            
            // 5. Link task to improvement
            err = client.CreateRelationship(ctx, improvementObj.ID, 
                emergent.RelHasTask, taskObj.ID, nil)
            if err != nil {
                t.logger.Warn("failed to link task to improvement", "error", err)
            }
            
            // 6. Link task to affected entity
            err = client.CreateRelationship(ctx, taskObj.ID, 
                emergent.RelAffectsEntity, issue.EntityID, nil)
            if err != nil {
                t.logger.Warn("failed to link task to affected entity", "error", err)
            }
        }
        
        t.logger.Info("created improvement with tasks",
            "improvement_id", improvementObj.ID,
            "task_count", len(category.Issues))
    }
    
    return improvementIDs, nil
}
```

### 3.3 Helper Functions

```go
func (t *JanitorRun) categorizeIssues(issues []Issue) []IssueCategory {
    categories := make(map[string]*IssueCategory)
    
    for _, issue := range issues {
        if issue.Severity != "warning" {
            continue // Only process warnings
        }
        
        var category *IssueCategory
        switch issue.Type {
        case "naming_convention":
            if categories["naming"] == nil {
                categories["naming"] = &IssueCategory{
                    Type:            "naming_convention",
                    Title:           "Fix Naming Convention Violations",
                    Description:     "Entities should use kebab-case naming. These entities use spaces or other invalid characters.",
                    Domain:          "documentation",
                    ImprovementType: "cleanup",
                    Effort:          "trivial",
                    Priority:        "low",
                    Issues:          []Issue{},
                }
            }
            category = categories["naming"]
            
        case "orphaned_entity":
            if categories["orphaned"] == nil {
                categories["orphaned"] = &IssueCategory{
                    Type:            "orphaned_entity",
                    Title:           "Link Orphaned Artifacts to Changes",
                    Description:     "These artifacts exist but are not connected to any Change. They should be linked or archived.",
                    Domain:          "maintenance",
                    ImprovementType: "tech_debt",
                    Effort:          "small",
                    Priority:        "medium",
                    Issues:          []Issue{},
                }
            }
            category = categories["orphaned"]
            
        case "missing_relationship":
            if categories["missing_rel"] == nil {
                categories["missing_rel"] = &IssueCategory{
                    Type:            "missing_relationship",
                    Title:           "Add Missing Artifact Relationships",
                    Description:     "These artifacts are missing required relationships (e.g., Requirements without Scenarios).",
                    Domain:          "maintenance",
                    ImprovementType: "cleanup",
                    Effort:          "small",
                    Priority:        "medium",
                    Issues:          []Issue{},
                }
            }
            category = categories["missing_rel"]
        }
        
        if category != nil {
            category.Issues = append(category.Issues, issue)
        }
    }
    
    // Convert map to slice
    result := []IssueCategory{}
    for _, cat := range categories {
        result = append(result, *cat)
    }
    return result
}

func (t *JanitorRun) estimateComplexity(issue Issue) int {
    switch issue.Type {
    case "naming_convention":
        return 1 // Trivial - just rename
    case "orphaned_entity":
        return 2 // Simple - link to a change
    case "missing_relationship":
        return 3 // Moderate - need to understand context
    default:
        return 2
    }
}

func (t *JanitorRun) getFixComplexity(issue Issue) string {
    switch issue.Type {
    case "naming_convention":
        return "trivial"
    case "orphaned_entity":
        return "simple"
    case "missing_relationship":
        return "simple"
    default:
        return "simple"
    }
}
```

### 3.4 Configuration for Improvement Creation

**File**: `internal/config/config.go`

```go
type JanitorConfig struct {
    Enabled          bool     `toml:"enabled"`
    IntervalHours    int      `toml:"interval_hours"`
    CreateProposal   bool     `toml:"create_proposal"`
    
    // NEW: Configuration for improvement creation
    CreateImprovements     bool     `toml:"create_improvements"`      // Create improvements for warnings
    ImprovementSeverities  []string `toml:"improvement_severities"`   // Which severities trigger improvements: ["critical", "warning"]
}
```

**File**: `specmcp.example.toml`

```toml
[janitor]
enabled = false
interval_hours = 1
create_proposal = false

# Create improvement entities with tasks for issues found
create_improvements = true
improvement_severities = ["critical", "warning"]
```

### 3.5 Update Execute() Method

**File**: `internal/tools/janitor/janitor.go`

```go
func (t *JanitorRun) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
    // ... existing code ...
    
    // Generate summary
    report.Summary = t.generateSummary(report)
    
    // Log findings summary for visibility
    t.logFindings(report)
    
    // Create maintenance proposal if requested and critical issues exist
    var proposalID string
    if p.CreateProposal && report.CriticalIssues > 0 {
        // ... existing proposal creation code ...
    }
    
    // NEW: Create improvements with subtasks for warnings
    var improvementIDs []string
    if p.CreateImprovements && (report.Warnings > 0 || report.CriticalIssues > 0) {
        var janitorAgentID string
        if janitorAgent != nil {
            janitorAgentID = janitorAgent.ID
        }
        
        ids, err := t.createImprovementsForWarnings(ctx, client, report, janitorAgentID)
        if err != nil {
            t.logger.Error("error creating improvements", "error", err)
        } else {
            improvementIDs = ids
            t.logger.Info("created improvements for warnings",
                "improvement_count", len(improvementIDs),
                "total_tasks", len(report.Issues))
        }
    }
    
    result := map[string]any{
        "report": report,
    }
    if proposalID != "" {
        result["proposal_id"] = proposalID
        result["message"] = fmt.Sprintf("Found %d critical issues. Maintenance proposal created: %s",
            report.CriticalIssues, proposalID)
    } else if len(improvementIDs) > 0 {
        result["improvement_ids"] = improvementIDs
        result["message"] = fmt.Sprintf("Janitor run complete. Found %d issues (%d critical, %d warnings). Created %d improvements.",
            report.IssuesFound, report.CriticalIssues, report.Warnings, len(improvementIDs))
    } else {
        result["message"] = fmt.Sprintf("Janitor run complete. Found %d issues (%d critical, %d warnings)",
            report.IssuesFound, report.CriticalIssues, report.Warnings)
    }
    
    return mcp.JSONResult(result)
}
```

### 3.6 Update Tool Input Schema

```go
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
      "description": "If true, create Improvement entities with subtasks for warnings (default: false)"
    },
    "improvement_severities": {
      "type": "array",
      "items": {"type": "string", "enum": ["critical", "warning"]},
      "description": "Which severity levels should trigger improvement creation (default: [\"critical\", \"warning\"])"
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
```

---

## Phase 4: Testing & Verification

### 4.1 Unit Tests

Create tests for:
- Issue categorization
- Improvement creation
- Task creation and linking
- Agent migration

### 4.2 Integration Tests

1. Run janitor with `create_improvements: true`
2. Verify improvements created with correct structure
3. Verify tasks linked to improvements
4. Verify `proposed_by` relationships
5. Verify `affects_entity` relationships

### 4.3 Manual Testing

```bash
# 1. Run janitor with improvement creation
curl -X POST http://localhost:21452/mcp \
  -H "Authorization: Bearer $EMERGENT_TOKEN" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "spec_janitor_run",
      "arguments": {
        "scope": "all",
        "create_improvements": true,
        "improvement_severities": ["warning"]
      }
    }
  }'

# 2. Query improvements
curl -X POST http://localhost:3002/api/v1/graph/query \
  -H "Authorization: Bearer $EMERGENT_TOKEN" \
  -d '{"filters":[{"key":"type","value":"Improvement"}]}'

# 3. Check tasks linked to improvements
# ... (query each improvement's tasks)
```

---

## Phase 5: Documentation

### 5.1 Update Existing Docs

- `AGENTS.md` - Update CodingAgent → Agent
- `README.md` - Update entity type references
- `docs/SPECMCP_DESIGN.md` - Update entity model
- `docs/JANITOR_LOGGING.md` - Add improvement creation section

### 5.2 Create New Docs

- `docs/AGENT_TYPES.md` - Document agent types and when to use each
- `docs/IMPROVEMENT_WORKFLOW.md` - How improvements work with tasks
- `docs/JANITOR_IMPROVEMENTS.md` - How janitor creates actionable improvements

---

## Implementation Order

1. ✅ Phase 1: Rename CodingAgent → Agent
   - Update types, relationships, functions
   - Update template pack
   - Create migration script

2. ✅ Phase 2: Enhance Task schema
   - Add new properties
   - Update template pack

3. ✅ Phase 3: Implement improvement creation in janitor
   - Issue categorization
   - Improvement creation logic
   - Task creation and linking
   - Configuration support

4. ✅ Phase 4: Testing
   - Unit tests
   - Integration tests
   - Manual verification

5. ✅ Phase 5: Documentation
   - Update existing docs
   - Create new guides

6. ✅ Deployment
   - Deploy to mcj-emergent
   - Run janitor to create improvements for existing warnings
   - Verify in production

---

## Estimated Effort

- Phase 1 (Agent rename): ~4 hours (schema, code, migration)
- Phase 2 (Task schema): ~1 hour
- Phase 3 (Improvement logic): ~6 hours (categorization, creation, linking)
- Phase 4 (Testing): ~3 hours
- Phase 5 (Documentation): ~2 hours

**Total**: ~16 hours (~2 days)

---

## Risks & Considerations

1. **Data Migration**: Need to carefully migrate existing CodingAgent entities
2. **Relationship Updates**: All existing relationships to CodingAgent must be updated
3. **Breaking Changes**: This is a breaking change - need version bump (0.7.0)
4. **Task Volume**: Creating 51 tasks might be overwhelming - may want batching
5. **Agent Skills**: Need to ensure agents can filter improvements by their skills

---

## Success Criteria

- ✅ All CodingAgent entities migrated to Agent
- ✅ Janitor creates improvements for warnings
- ✅ Each improvement has subtasks for individual issues
- ✅ Improvements properly linked to janitor agent via `proposed_by`
- ✅ Tasks properly linked to affected entities
- ✅ Configuration works for controlling improvement creation
- ✅ Logs show improvement creation
- ✅ Documentation updated
- ✅ Deployed to production and verified
