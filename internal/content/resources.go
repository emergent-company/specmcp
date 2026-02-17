package content

import "github.com/emergent-company/specmcp/internal/mcp"

// --- specmcp://entity-model resource ---

// EntityModelResource exposes the full SpecMCP entity model as a reference
// resource. LLMs can read this to understand the graph schema.
type EntityModelResource struct{}

func (r *EntityModelResource) Definition() mcp.ResourceDefinition {
	return mcp.ResourceDefinition{
		URI:         "specmcp://entity-model",
		Name:        "SpecMCP Entity Model",
		Description: "Complete reference of all entity types, relationship types, and their properties used in the SpecMCP knowledge graph",
		MimeType:    "text/markdown",
	}
}

func (r *EntityModelResource) Read() (*mcp.ResourcesReadResult, error) {
	return &mcp.ResourcesReadResult{
		Contents: []mcp.ResourceContent{
			{
				URI:      "specmcp://entity-model",
				MimeType: "text/markdown",
				Text:     entityModelContent,
			},
		},
	}, nil
}

// --- specmcp://guardrails resource ---

// GuardrailsResource exposes the guardrail rules as a reference resource.
type GuardrailsResource struct{}

func (r *GuardrailsResource) Definition() mcp.ResourceDefinition {
	return mcp.ResourceDefinition{
		URI:         "specmcp://guardrails",
		Name:        "SpecMCP Guardrails",
		Description: "Reference of all guardrail checks, their severity levels, and when they run",
		MimeType:    "text/markdown",
	}
}

func (r *GuardrailsResource) Read() (*mcp.ResourcesReadResult, error) {
	return &mcp.ResourcesReadResult{
		Contents: []mcp.ResourceContent{
			{
				URI:      "specmcp://guardrails",
				MimeType: "text/markdown",
				Text:     guardrailsContent,
			},
		},
	}, nil
}

// --- specmcp://tool-reference resource ---

// ToolReferenceResource exposes a quick-reference card for all 27 tools.
type ToolReferenceResource struct{}

func (r *ToolReferenceResource) Definition() mcp.ResourceDefinition {
	return mcp.ResourceDefinition{
		URI:         "specmcp://tool-reference",
		Name:        "SpecMCP Tool Reference",
		Description: "Quick-reference card for all 27 SpecMCP tools with parameters and usage notes",
		MimeType:    "text/markdown",
	}
}

func (r *ToolReferenceResource) Read() (*mcp.ResourcesReadResult, error) {
	return &mcp.ResourcesReadResult{
		Contents: []mcp.ResourceContent{
			{
				URI:      "specmcp://tool-reference",
				MimeType: "text/markdown",
				Text:     toolReferenceContent,
			},
		},
	}, nil
}

// --- specmcp://guide resource ---

// GuideResource is the primary LLM-facing reference that explains the SpecMCP
// concept, workflow, guardrails, and how to use the tools.
type GuideResource struct{}

func (r *GuideResource) Definition() mcp.ResourceDefinition {
	return mcp.ResourceDefinition{
		URI:         "specmcp://guide",
		Name:        "SpecMCP Guide",
		Description: "Comprehensive reference for SpecMCP: concept, workflow, guardrails, apps, and tool usage",
		MimeType:    "text/markdown",
	}
}

func (r *GuideResource) Read() (*mcp.ResourcesReadResult, error) {
	return &mcp.ResourcesReadResult{
		Contents: []mcp.ResourceContent{
			{
				URI:      "specmcp://guide",
				MimeType: "text/markdown",
				Text:     guideContent,
			},
		},
	}, nil
}

// --- specmcp://workflow resource ---

// WorkflowResource is a reference guide for the artifact workflow sequence.
type WorkflowResource struct{}

func (r *WorkflowResource) Definition() mcp.ResourceDefinition {
	return mcp.ResourceDefinition{
		URI:         "specmcp://workflow",
		Name:        "SpecMCP Workflow Reference",
		Description: "Reference guide for the SpecMCP artifact workflow (Proposal → Specs → Design → Tasks)",
		MimeType:    "text/markdown",
	}
}

func (r *WorkflowResource) Read() (*mcp.ResourcesReadResult, error) {
	return &mcp.ResourcesReadResult{
		Contents: []mcp.ResourceContent{
			{
				URI:      "specmcp://workflow",
				MimeType: "text/markdown",
				Text:     workflowContent,
			},
		},
	}, nil
}

// --- Static content ---

const entityModelContent = `# SpecMCP Entity Model

## Entity Types (18)

### Change
Top-level container for a feature, bug fix, or refactoring effort.
- **Properties**: name (string, required), status (string: active/archived), base_commit (string), tags ([]string)
- **Relationships**:
  - has_proposal → Proposal (1:1)
  - has_spec → Spec (1:N)
  - has_design → Design (1:1)
  - has_task → Task (1:N)
  - governed_by → Constitution
  - uses_pattern → Pattern

### Proposal
Captures the intent of a change — why it exists.
- **Properties**: intent (string, required), scope (string), impact (string), status (string: draft/ready), tags ([]string)
- **Relationships**: (linked from Change via has_proposal)

### Spec
Domain-specific specification container holding requirements.
- **Properties**: name (string, required), domain (string), purpose (string), delta_type (string), status (string: draft/ready), tags ([]string)
- **Relationships**:
  - has_requirement → Requirement (1:N)

### Requirement
Specific behavior the system must have.
- **Properties**: name (string, required), description (string, required), strength (string: MUST/SHOULD/MAY), delta_type (string), status (string: draft/ready), tags ([]string)
- **Relationships**:
  - has_scenario → Scenario (1:N)

### Scenario
Concrete example of a requirement in BDD format.
- **Properties**: name (string, required), title (string), given (string), when (string), then (string), and_also ([]string), status (string: draft/ready), tags ([]string)
- **Relationships**:
  - has_step → ScenarioStep (1:N, for complex scenarios)
  - tested_by → TestCase

### ScenarioStep
Individual step in a complex scenario.
- **Properties**: sequence (int, required), description (string, required), tags ([]string)

### Design
Technical approach for implementing the change.
- **Properties**: approach (string), decisions (string), data_flow (string), file_changes ([]string), status (string: draft/ready), tags ([]string)
- **Relationships**: (linked from Change via has_design)

### Task
Implementable step with tracking.
- **Properties**: number (string, required), description (string, required), task_type (string), status (string: pending/in_progress/completed/blocked), complexity_points (int, 1-10), started_at (time), completed_at (time), actual_hours (float), artifacts ([]string), verification_method (string), verification_notes (string), tags ([]string)
- **Relationships**:
  - blocks → Task (auto-creates inverse blocked_by)
  - blocked_by → Task (auto-created)
  - has_subtask → Task
  - implements → Requirement
  - assigned_to → Agent

### Actor
User role or persona.
- **Properties**: name (string, required), display_name (string), description (string), tags ([]string)
- **Relationships**:
  - performs → Action
  - occurs_in → Context

### Agent
Developer or AI agent that works on tasks.
- **Properties**: name (string, required), display_name (string), type (string: human/ai, required), agent_type (string: coding/maintenance/research/testing/deployment/analysis), active (bool, required), skills ([]string), specialization (string), instructions (string), velocity_points_per_hour (float), tags ([]string)

### Pattern
Reusable implementation convention.
- **Properties**: name (string, required), display_name (string), type (string: naming/structural/behavioral/testing/security/api, required), scope (string), description (string), example_code (string), usage_guidance (string), tags ([]string)
- **Relationships**:
  - inherits_from → Pattern
  - extends_pattern → Pattern

### Constitution
Project-wide principles and guardrails.
- **Properties**: name (string, required), version (string, required), principles (string), guardrails ([]string), testing_requirements (string), security_requirements (string), patterns_required ([]string), patterns_forbidden ([]string), tags ([]string)
- **Relationships**:
  - requires_pattern → Pattern
  - forbids_pattern → Pattern

### TestCase
Links scenarios to executable tests.
- **Properties**: name (string, required), test_file (string), test_function (string), test_framework (string), status (string), last_run_at (time), coverage_percent (float), tags ([]string)
- **Relationships**:
  - tests → Scenario (inverse of tested_by)

### APIContract
Machine-readable API definition.
- **Properties**: name (string, required), format (string: openapi/graphql/grpc, required), version (string), file_path (string), base_url (string), description (string), auto_generated (bool), last_validated_at (time), validation_status (string), tags ([]string)
- **Relationships**:
  - implements_contract → Spec

### Context
Screen, page, modal, or interaction surface.
- **Properties**: name (string, required), display_name (string), type (string), scope (string), platform ([]string), description (string), file_path (string), tags ([]string)
- **Relationships**:
  - composed_of → UIComponent (1:N)
  - available_in → Action (1:N)
  - navigates_to → Context

### UIComponent
Reusable UI component.
- **Properties**: name (string, required), display_name (string), type (string), file_path (string), description (string), tags ([]string)
- **Relationships**:
  - nested_in → UIComponent
  - uses_component → UIComponent

### Action
User action or system operation.
- **Properties**: name (string, required), display_label (string), type (string), description (string), handler_path (string), tags ([]string)

### GraphSync
Tracks synchronization state between graph and codebase.
- **Properties**: last_synced_commit (string), last_synced_at (time), status (string: pending/in_progress/completed, required), tags ([]string)

## Relationship Types (30+)

| Relationship | Source | Target | Bidirectional? |
|-------------|--------|--------|----------------|
| has_proposal | Change | Proposal | No |
| has_spec | Change | Spec | No |
| has_design | Change | Design | No |
| has_task | Change | Task | No |
| has_requirement | Spec | Requirement | No |
| has_scenario | Requirement | Scenario | No |
| has_step | Scenario | ScenarioStep | No |
| blocks | Task | Task | Yes → blocked_by |
| blocked_by | Task | Task | Yes → blocks |
| has_subtask | Task | Task | No |
| implements | Task | Requirement | No |
| assigned_to | Task | Agent | No |
| governed_by | Change | Constitution | No |
| requires_pattern | Constitution | Pattern | No |
| forbids_pattern | Constitution | Pattern | No |
| uses_pattern | Change | Pattern | No |
| inherits_from | Pattern | Pattern | No |
| extends_pattern | Pattern | Pattern | No |
| tested_by | Scenario | TestCase | Yes → tests |
| tests | TestCase | Scenario | Yes → tested_by |
| has_contract | Spec | APIContract | No |
| implements_contract | APIContract | Spec | No |
| performs | Actor | Action | No |
| occurs_in | Actor | Context | No |
| executed_by | Scenario | Actor | No |
| variant_of | Scenario | Scenario | No |
| composed_of | Context | UIComponent | No |
| uses_component | UIComponent | UIComponent | No |
| nested_in | UIComponent | UIComponent | No |
| available_in | Context | Action | No |
| navigates_to | Context | Context | No |
| owned_by | Context | Actor | No |

## Tagging Conventions

Tags use namespaced conventions:
- ` + "`domain:auth`" + `, ` + "`domain:payments`" + ` — Domain classification
- ` + "`platform:web`" + `, ` + "`platform:mobile`" + ` — Platform targeting
- ` + "`lifecycle:stable`" + `, ` + "`lifecycle:experimental`" + ` — Maturity
- ` + "`priority:high`" + `, ` + "`priority:low`" + ` — Priority classification

## Status Values

| Status | Used By |
|--------|---------|
| active | Change |
| archived | Change |
| draft | Proposal, Spec, Requirement, Scenario, Design |
| ready | Proposal, Spec, Requirement, Scenario, Design |
| pending | Task, GraphSync |
| in_progress | Task, GraphSync |
| completed | Task, GraphSync |
| blocked | Task |
`

const guardrailsContent = `# SpecMCP Guardrails Reference

## Overview

Guardrails are composable checks that run automatically at key workflow points.
Each guard returns a result with one of four severity levels.

## Severity Levels

| Level | Code | Meaning | Override |
|-------|------|---------|---------|
| HARD_BLOCK | 1 | Cannot proceed | Must fix the issue |
| SOFT_BLOCK | 2 | Should not proceed | Use force=true |
| WARNING | 3 | Advisory | Recommended action |
| SUGGESTION | 4 | Informational | No action needed |

## Guard Sets

### Pre-Change Guards (run on spec_new)

| # | Guard | Severity | Checks |
|---|-------|----------|--------|
| 1 | kebab_case_name | HARD_BLOCK | Name matches ^[a-z][a-z0-9]*(-[a-z0-9]+)*$ |
| 2 | constitution_required | HARD_BLOCK | At least one Constitution exists in the project |
| 3 | patterns_seeded | SOFT_BLOCK | At least one Pattern exists in the project |
| 4 | context_discovery | SUGGESTION | At least one Context entity exists |
| 5 | component_discovery | SUGGESTION | At least one UIComponent entity exists |

### Artifact Guards (run on spec_artifact)

These guards check **readiness**, not just existence. Use ` + "`spec_mark_ready`" + ` to mark artifacts as ready before progressing.

| # | Guard | Severity | Checks |
|---|-------|----------|--------|
| 1 | proposal_before_spec | HARD_BLOCK | Change has a **ready** Proposal before adding Spec/Requirement/Scenario |
| 2 | spec_before_design | HARD_BLOCK | Change has **ready** Proposal + all Specs **ready** before adding Design |
| 3 | design_before_tasks | HARD_BLOCK | Change has a **ready** Design before adding Tasks |

### Archive Guards (run on spec_archive)

| # | Guard | Severity | Checks |
|---|-------|----------|--------|
| 1 | artifact_completeness | SOFT_BLOCK | Change has proposal, specs, design, and tasks |
| 2 | task_completion | SOFT_BLOCK | All tasks have status=completed |

## Verification Dimensions (spec_verify)

The spec_verify tool performs deeper analysis across three dimensions:

### 1. Completeness
- All core artifacts exist (proposal, specs, design, tasks)
- Each spec has at least one requirement
- Each requirement has at least one scenario
- All tasks are completed

### 2. Correctness
- Requirements map to scenarios (coverage check)
- Design references applicable patterns
- Tasks implement requirements

### 3. Coherence
- Implementation follows the design approach
- Code patterns match applied patterns
- Naming follows constitution conventions

## Guard Context

Guards receive a populated GuardContext containing:
- Project-level state: HasConstitution, HasPatterns, ContextCount, ComponentCount
- Change-level state: ChangeName, ArtifactType, HasProposal, HasSpec, HasDesign, HasTasks, TaskCount, CompletedTasks
- Readiness state: ProposalReady, AllSpecsReady, DesignReady

The context is populated once via graph queries, then shared across all guards to avoid N+1 query patterns.
`

// --- Guide content ---

const guideContent = `# SpecMCP — Spec-Driven Development via MCP

## What is SpecMCP?

SpecMCP is a Model Context Protocol (MCP) server that provides **spec-driven development workflow** backed by a knowledge graph. Instead of managing specifications as scattered files, SpecMCP stores all artifacts — proposals, specs, requirements, scenarios, designs, tasks, patterns, and constitutions — as graph entities with typed relationships.

This enables:
- **Traceability**: Every requirement traces to scenarios, design decisions, and tasks
- **Impact analysis**: Understand what's affected when something changes
- **Dependency management**: Tasks with explicit blocking relationships and critical path analysis
- **Pattern enforcement**: Reusable patterns with automatic detection and compliance checking
- **Guardrails**: Automated checks at every stage to prevent workflow mistakes
- **Monorepo support**: First-class support for multiple apps with shared data models

## Core Concept: Artifact Workflow

Every change follows a structured progression:

` + "```" + `
Proposal (Why) → Specs (What) → Design (How) → Tasks (Steps)
` + "```" + `

Each stage builds on the previous one, enforced by guardrails:
- You cannot add specs without a **ready** proposal
- You cannot add a design without a **ready** proposal and all specs **ready**
- You cannot add tasks without a **ready** design

## Entity Model

SpecMCP uses 18 entity types and 30+ relationship types. Key entities:

| Entity | Purpose |
|--------|---------|
| **Change** | Top-level container for a feature, fix, or refactoring |
| **Proposal** | Intent — why the change exists |
| **Spec** | Domain-specific specification container |
| **Requirement** | Specific behavior the system must have |
| **Scenario** | BDD-style example (Given/When/Then) |
| **Design** | Technical approach — how specs will be implemented |
| **Task** | Implementable step with complexity, status, blocking |
| **Pattern** | Reusable implementation convention |
| **Constitution** | Project-wide principles and guardrails |
| **App** | Deployable application in your monorepo |
| **DataModel** | Shared data structure across apps |
| **Actor** | User role or persona |
| **Context** | Screen, page, or interaction surface |
| **UIComponent** | Reusable UI component |
| **Action** | User action or system operation |
| **TestCase** | Executable test linked to a scenario |
| **APIContract** | Machine-readable API definition |
| **Agent** | Developer or AI agent |

## Monorepo Support

SpecMCP fully supports monorepo architectures with Apps and DataModels:

### App Entity
Represents a deployable application (frontend, backend, mobile, desktop, cli, library).

**Key fields**: name, app_type, platform, root_path, tech_stack, instructions, deployment_target, entry_point, port

**Relationships**:
- belongs_to_app (from Context/UIComponent/Action/APIContract)
- scoped_to_app (from Change/Spec/Design/Task)
- depends_on_app (app dependencies)
- provides_model/consumes_model (data model relationships)

### DataModel Entity
Represents shared data structures across apps.

**Key fields**: name, platform, file_path, fields, persistence, go_type, ts_type, swift_type

**Relationships**:
- provides_model (App → DataModel)
- consumes_model (App → DataModel)

### Cross-App Changes
Use ` + "`scoped_to_apps`" + ` field on Changes, Specs, Designs, and Tasks to indicate which apps are affected.

## Guardrails

SpecMCP enforces guardrails at three points:

### Pre-Change Guards (spec_new)
| Guard | Severity | What it checks |
|-------|----------|----------------|
| kebab_case_name | HARD_BLOCK | Change name must be kebab-case |
| constitution_required | HARD_BLOCK | Project should have a constitution |
| patterns_seeded | SOFT_BLOCK | Project should have patterns |
| context_discovery | SUGGESTION | Contexts should be mapped |
| component_discovery | SUGGESTION | Components should be mapped |

### Artifact Guards (spec_artifact)
| Guard | Severity | What it checks |
|-------|----------|----------------|
| proposal_before_spec | HARD_BLOCK | Proposal required before specs |
| spec_before_design | HARD_BLOCK | Specs required before design |
| design_before_tasks | HARD_BLOCK | Design required before tasks |

### Archive Guards (spec_archive)
| Guard | Severity | What it checks |
|-------|----------|----------------|
| artifact_completeness | SOFT_BLOCK | All core artifacts should exist |
| task_completion | SOFT_BLOCK | All tasks should be completed |

**Severity levels**:
- **HARD_BLOCK**: Cannot proceed. Fix the issue first.
- **SOFT_BLOCK**: Should not proceed. Use force=true to override.
- **WARNING**: Advisory — action recommended but not required.
- **SUGGESTION**: Informational tip.

## Tools Reference

### Workflow (7 tools)
- **spec_new** — Create a new change container
- **spec_artifact** — Add any artifact type to a change (18 types supported)
- **spec_batch_artifact** — Add multiple artifacts in one call
- **spec_archive** — Archive a completed change
- **spec_verify** — Verify completeness, correctness, and coherence
- **spec_mark_ready** — Mark a workflow artifact as ready (with cascading validation)
- **spec_status** — Get readiness status and next steps for a change

### Query (11 tools)
- **spec_list_changes** — List all changes (filter by status)
- **spec_get_change** — Get full change details with artifacts
- **spec_get_app** — Get an app with all relationships
- **spec_get_data_model** — Get a data model with provider/consumers
- **spec_get_context** — Get a context entity with relationships
- **spec_get_component** — Get a UI component with relationships
- **spec_get_action** — Get an action entity with relationships
- **spec_get_scenario** — Get a scenario with steps
- **spec_get_patterns** — Get all patterns (filter by type)
- **spec_impact_analysis** — Analyze what's affected by a change
- **spec_search** — Search across all entity types

### Task Management (5 tools)
- **spec_generate_tasks** — Auto-generate tasks from a design
- **spec_get_available_tasks** — Get tasks ready for work (dependencies met)
- **spec_assign_task** — Assign a task to an agent
- **spec_complete_task** — Mark a task as done with verification
- **spec_get_critical_path** — Find the longest dependency chain

### Patterns (3 tools)
- **spec_suggest_patterns** — Suggest applicable patterns for a change
- **spec_apply_pattern** — Link a pattern to a change
- **spec_seed_patterns** — Seed 15 built-in patterns

### Constitution (2 tools)
- **spec_create_constitution** — Create or update the project constitution
- **spec_validate_constitution** — Check change compliance against constitution

### Sync (3 tools)
- **spec_sync_status** — Check graph sync state
- **spec_sync** — Synchronize graph with codebase
- **spec_graph_summary** — Get entity/relationship counts

### Janitor (1 tool)
- **spec_janitor_run** — Run compliance verification and maintenance

## Recommended Workflow

1. **Bootstrap** (once per project):
   - Seed patterns: ` + "`spec_seed_patterns`" + `
   - Create constitution: ` + "`spec_create_constitution`" + `

2. **Map your monorepo**:
   - Create App entities for each deployable app
   - Create DataModel entities for shared data structures
   - Link apps to models (provides_model/consumes_model)

3. **Start a change**:
   - ` + "`spec_new`" + ` with a kebab-case name
   - Always identify which apps are affected

4. **Define the change** (follow the artifact workflow):
   - Proposal → mark ready → Specs → Requirements → Scenarios → mark ready (bottom-up) → Design → mark ready → Tasks
   - Use ` + "`spec_status`" + ` to check readiness and see next steps at any stage

5. **Implement**:
   - Work through tasks in dependency order using task management tools

6. **Verify and archive**:
   - ` + "`spec_verify`" + ` to check completeness
   - ` + "`spec_archive`" + ` to finalize
`

// --- Workflow content ---

const workflowContent = `# SpecMCP Artifact Workflow Reference

Every change follows a structured artifact progression, each stage building on the previous:

## Readiness Gating

Workflow progression is gated on **readiness**, not just existence. Each workflow artifact (Proposal, Spec, Requirement, Scenario, Design) has a status of **draft** or **ready**.

- New artifacts start as **draft**
- Use ` + "`spec_mark_ready`" + ` to mark an artifact as **ready**
- Readiness cascades: a Spec can't be ready unless all its Requirements are ready; a Requirement can't be ready unless all its Scenarios are ready
- Adding a child to a ready parent (e.g., a new Requirement to a ready Spec) automatically reverts the parent to **draft**
- Use ` + "`spec_status`" + ` to see overall readiness and next steps

## Proposal (Why)
Captures the **intent** — why this change needs to happen.
- Fields: intent (required), scope, impact
- Created via: spec_artifact with artifact_type="proposal"
- Must be marked **ready** before adding specs

## Specs (What)
Define **what** the change does. Each spec is a domain-specific container.
- Fields: name, domain, purpose, delta_type, scoped_to_apps
- Contains: Requirements (MUST/SHOULD/MAY)
- Requirements contain: Scenarios (Given/When/Then BDD format)
- Mark ready **bottom-up**: Scenarios → Requirements → Specs

## Design (How)
Describes **how** specs will be implemented.
- Fields: approach, decisions, data_flow, file_changes, scoped_to_apps
- Should reference applicable patterns
- Must be marked **ready** before adding tasks

## Tasks (Steps)
Break the design into concrete, implementable steps.
- Fields: number, description, task_type, complexity_points (1-10), scoped_to_apps
- Relationships: blocks/blocked_by (auto-bidirectional), has_subtask, assigned_to
- Auto-generation available via spec_generate_tasks

## Apps & Data Models
When changes affect apps or introduce data models:
- Create App entities (artifact_type="app") for new apps
- Create DataModel entities (artifact_type="data_model") for new data structures
- Link apps to models (provides_model/consumes_model relationships)
- Use scoped_to_apps field on workflow artifacts

## Enforcement
The workflow order is enforced by readiness-based guardrails:
- Specs require a **ready** Proposal (hard block)
- Design requires **ready** Proposal + all Specs **ready** (hard block)
- Tasks require a **ready** Design (hard block)
`

// --- Tool reference content ---

const toolReferenceContent = `# SpecMCP Tool Quick Reference

## Workflow Tools

### spec_new
Create a new change container.
- **Required**: name (string, kebab-case)
- **Optional**: intent (string), tags ([]string), force (bool)
- **Guards**: kebab_case_name, constitution_required, patterns_seeded, context_discovery, component_discovery

### spec_artifact
Add an artifact to an existing change. Supports 16 artifact types.
- **Required**: change_name (string), artifact_type (string)
- **artifact_type values**: proposal, spec, requirement, scenario, scenario_step, design, task, actor, agent, pattern, constitution, test_case, api_contract, context, ui_component, action
- **Additional params**: vary by artifact_type (see tool's input schema)
- **Guards**: proposal_before_spec, spec_before_design, design_before_tasks

### spec_archive
Archive a completed change.
- **Required**: change_name (string)
- **Optional**: force (bool)
- **Guards**: artifact_completeness, task_completion

### spec_verify
Verify change completeness, correctness, and coherence.
- **Required**: change_name (string)
- **Returns**: structured report with issues by dimension and severity

### spec_mark_ready
Mark a workflow artifact (Proposal, Spec, Requirement, Scenario, Design) as ready.
- **Required**: entity_id (string)
- **Cascading validation**: For Specs, all Requirements must be ready. For Requirements, all Scenarios must be ready.
- **Returns**: success confirmation, or blockers list with unready children (id, type, name, status)

### spec_status
Get readiness status and next steps for a change.
- **Required**: change_id (string)
- **Returns**: workflow stage, per-artifact readiness summaries, prioritized next_steps, ready_to_archive boolean

## Query Tools

### spec_list_changes
- **Optional**: status (string: active/archived)
- **Returns**: list of changes with summary

### spec_get_change
- **Required**: change_name (string)
- **Returns**: full change with all artifacts and relationships

### spec_get_context
- **Required**: name (string)
- **Returns**: context entity with composed_of, available_in, navigates_to relationships

### spec_get_component
- **Required**: name (string)
- **Returns**: UI component with nested_in, uses_component relationships

### spec_get_action
- **Required**: name (string)
- **Returns**: action entity with performs, available_in relationships

### spec_get_scenario
- **Required**: name (string)
- **Returns**: scenario with steps, tested_by relationships

### spec_get_patterns
- **Optional**: type (string: naming/structural/behavioral/testing/security/api)
- **Returns**: list of patterns

### spec_impact_analysis
- **Required**: change_name (string)
- **Returns**: affected entities, relationship chains, risk assessment

## Task Management Tools

### spec_generate_tasks
- **Required**: change_name (string)
- **Returns**: auto-generated tasks based on design analysis

### spec_get_available_tasks
- **Required**: change_name (string)
- **Returns**: tasks with all blocking dependencies satisfied

### spec_assign_task
- **Required**: task_id (string), agent_name (string)
- **Returns**: updated task with assignment

### spec_complete_task
- **Required**: task_id (string)
- **Optional**: verification_notes (string), artifacts ([]string)
- **Returns**: updated task with completion timestamp

### spec_get_critical_path
- **Required**: change_name (string)
- **Returns**: longest dependency chain with total complexity

## Pattern Tools

### spec_suggest_patterns
- **Required**: change_name (string)
- **Returns**: patterns that may apply based on change analysis

### spec_apply_pattern
- **Required**: change_name (string), pattern_name (string)
- **Returns**: confirmation of uses_pattern relationship

### spec_seed_patterns
- **Optional**: force (bool) — re-seed even if patterns exist
- **Returns**: count of patterns created (15 built-in patterns)

## Constitution & Sync Tools

### spec_create_constitution
Create or update the project constitution (no change_id required).
- **Required**: name (string), version (string), principles (string)
- **Optional**: guardrails ([]string), testing_requirements (string), security_requirements (string), patterns_required ([]string), patterns_forbidden ([]string)
- **Returns**: constitution entity with linked patterns

### spec_validate_constitution
- **Required**: change_name (string)
- **Returns**: compliance report (required patterns, forbidden patterns, guardrail checks)

### spec_sync_status
- **Returns**: last sync commit, timestamp, status

### spec_sync
- **Optional**: commit (string)
- **Returns**: sync results (entities created/updated, patterns detected)

### spec_graph_summary
- **Returns**: counts of all entity types and relationship types
`
