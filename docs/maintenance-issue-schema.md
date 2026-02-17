# MaintenanceIssue Entity Schema

## Overview

A **MaintenanceIssue** represents a data integrity, compliance, or structural problem detected by the janitor agent. Unlike feature Changes, these are reactive maintenance items that restore or improve the health of the knowledge graph.

## Entity Properties

```json
{
  "type": "MaintenanceIssue",
  "key": "orphaned-specs-20260216",
  "properties": {
    "title": "Orphaned Spec Entities Detected",
    "description": "The janitor detected 3 Spec entities with no incoming relationships...",
    "severity": "critical",  // critical, warning, info
    "category": "data_integrity", // data_integrity, compliance, structural, stale_entities
    "status": "proposed",  // proposed, approved, in_progress, resolved, dismissed
    "detected_at": "2026-02-16T18:43:16Z",
    "detected_by": "janitor-agent",
    "janitor_run_id": "run-123",  // Link to the janitor execution
    "affected_count": 3,  // How many entities are affected
    "tags": ["orphaned", "specs", "auto-detected"]
  }
}
```

## Relationships

### Core Structure
- `has_task` → Task: Actionable steps to resolve the issue (1-5 tasks typical)
- `affects_entity` → *: Links to each affected entity

### Workflow Integration  
- `proposed_by` → Agent: The janitor agent
- `assigned_to` → Agent: Who will fix it (optional)
- `resolved_by_change` → Change: If fix requires code changes

### Tracking
- `parent_issue` → MaintenanceIssue: For related/grouped issues
- `blocked_by` → MaintenanceIssue: Dependencies

## Task Structure for Maintenance

Each MaintenanceIssue should have 1-5 specific Tasks with:

```json
{
  "type": "Task",
  "key": "fix-orphaned-spec-user-auth",
  "properties": {
    "number": "1",
    "description": "Link orphaned 'user-auth' spec to its parent Change",
    "task_type": "maintenance",  // NEW type for maintenance tasks
    "status": "pending",
    "verification_method": "Verify 'user-auth' spec has incoming 'has_spec' edge from a Change entity",
    "complexity_points": 2,
    "tags": ["data-integrity", "quick-fix"]
  }
}
```

### Relationship from MaintenanceIssue
```
MaintenanceIssue --has_task--> Task
```

### Relationship to affected entity
```
Task --implements--> Spec (the entity being fixed)
```

## Lifecycle States

1. **proposed**: Janitor created, awaiting human review
2. **approved**: Human says "go for it" 
3. **in_progress**: Tasks are being executed
4. **resolved**: All tasks completed, verification passed
5. **dismissed**: False positive or won't fix

## Example: Complete MaintenanceIssue

```
MaintenanceIssue: "missing-requirement-scenarios-20260216"
├── title: "Requirements Missing Scenarios"
├── description: "5 Requirement entities have no test scenarios..."
├── severity: "warning"
├── status: "proposed"
├── affected_count: 5
│
├── Task 1: "add-scenario-password-reset"
│   ├── description: "Add at least one test scenario for 'password-reset' requirement"
│   ├── verification_method: "Query requirement for outgoing 'has_scenario' edges, count >= 1"
│   └── implements --> Requirement[password-reset]
│
├── Task 2: "add-scenario-mfa-setup"
│   ├── description: "Add test scenarios for 'mfa-setup' requirement"
│   ├── verification_method: "Requirement has 2+ scenarios covering success and error cases"
│   └── implements --> Requirement[mfa-setup]
│
└── ... (3 more tasks)
```

## Integration with Janitor

The janitor's `createMaintenanceIssue()` function will:

1. Group related issues by category/type
2. Create one MaintenanceIssue per logical problem
3. Generate specific Tasks with:
   - Clear description of what to do
   - Verification method (success criteria)
   - Link to affected entity via `implements`
4. Return the MaintenanceIssue ID to the agent

## Workflow for "Go For It"

When user approves:
```
1. User: "go for it" (or calls spec_approve_maintenance_issue)
2. System: Updates status: proposed → approved
3. System: Marks all Tasks as status: pending (they become available)
4. Agent/User: Executes tasks one by one
5. Each task completion updates verification_notes
6. When all tasks complete: MaintenanceIssue status → resolved
```

## Template Pack Changes Required

Add to `templates/specmcp-pack.json`:

```json
{
  "entity_types": [
    {
      "name": "MaintenanceIssue",
      "description": "Data integrity or compliance issue detected by janitor",
      "properties": {
        "title": { "type": "string", "required": true },
        "description": { "type": "string", "required": true },
        "severity": { "type": "string", "enum": ["critical", "warning", "info"] },
        "category": { "type": "string" },
        "status": { "type": "string", "enum": ["proposed", "approved", "in_progress", "resolved", "dismissed"] },
        "detected_at": { "type": "string" },
        "detected_by": { "type": "string" },
        "janitor_run_id": { "type": "string" },
        "affected_count": { "type": "integer" },
        "tags": { "type": "array", "items": { "type": "string" } }
      }
    }
  ],
  "relationship_types": [
    { "name": "affects_entity", "description": "Links issue to affected entities" },
    { "name": "parent_issue", "description": "Groups related issues" }
  ]
}
```

## New Tools Needed

1. **spec_approve_maintenance_issue**
   - Sets status: proposed → approved
   - Makes tasks available for execution

2. **spec_dismiss_maintenance_issue**  
   - Sets status: dismissed
   - Marks as false positive or won't-fix

3. **spec_list_maintenance_issues**
   - Filter by status, severity, category
   - Show summary of pending work

4. Enhanced **spec_janitor_run**
   - Returns array of created MaintenanceIssue IDs
   - Groups issues intelligently
