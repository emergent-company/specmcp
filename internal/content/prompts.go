// Package content provides MCP prompts and resources for the SpecMCP server.
package content

import "github.com/emergent-company/specmcp/internal/mcp"

// --- create-constitution prompt ---

// CreateConstitutionPrompt is an actionable prompt that gathers information
// to create a project constitution.
type CreateConstitutionPrompt struct{}

func (p *CreateConstitutionPrompt) Definition() mcp.PromptDefinition {
	return mcp.PromptDefinition{
		Name:        "create-constitution",
		Description: "Interactive guide for creating a project constitution. Gathers principles, guardrails, and pattern requirements.",
		Arguments:   []mcp.PromptArgument{},
	}
}

func (p *CreateConstitutionPrompt) Get(arguments map[string]string) (*mcp.PromptsGetResult, error) {
	return &mcp.PromptsGetResult{
		Description: "Guide for creating a project constitution",
		Messages: []mcp.PromptMessage{
			{
				Role:    "user",
				Content: mcp.TextContent(createConstitutionGuide),
			},
		},
	}, nil
}

const createConstitutionGuide = `# Create Project Constitution

You are helping a user create or update their project's constitution. The constitution defines project-wide principles, guardrails, and pattern requirements.

## Your Role

1. Ask clarifying questions to understand the project
2. Help articulate principles and standards
3. Identify required and forbidden patterns
4. Create the constitution using spec_create_constitution tool

## Step 1: Gather Project Context

Ask these questions:

**Project Type:**
- What kind of project is this? (web app, mobile, backend service, library, etc.)
- What's the tech stack? (languages, frameworks, platforms)
- How many people work on this project?

**Project Stage:**
- Is this a new project or existing codebase?
- What's the maturity level? (prototype, production, legacy)

## Step 2: Define Principles

Ask about project values and coding principles:

**Core Principles:**
- What are the most important coding values for this project?
  (Examples: simplicity, performance, maintainability, security, consistency)
- Any specific architectural principles?
  (Examples: DRY, SOLID, separation of concerns, immutability)
- Team conventions or philosophy?
  (Examples: explicit over implicit, fail fast, composition over inheritance)

Capture 3-7 clear, concise principles. Examples:
- "Prefer explicit error handling over implicit failures"
- "Optimize for readability over cleverness"
- "Every public API must have documentation"
- "Security by default in all user inputs"

## Step 3: Define Guardrails

Ask about quality gates and checks:

**Testing Requirements:**
- What's the minimum test coverage? (percentage or policy)
- Which types of tests are required? (unit, integration, e2e)
- Any specific testing conventions?

**Security Requirements:**
- Security scanning requirements?
- Input validation policies?
- Authentication/authorization standards?

**Code Quality:**
- Linting requirements?
- Code review policies?
- Documentation standards?

Capture specific, measurable guardrails. Examples:
- "All public functions must have unit tests"
- "Code coverage must be >= 80%"
- "All user inputs must be validated at API boundaries"
- "No hardcoded secrets or credentials"

## Step 4: Pattern Requirements

Ask about coding patterns:

**Required Patterns** (patterns that MUST be used):
- Are there patterns you want to enforce?
  (Examples: repository pattern, factory pattern, error wrapping)
- Naming conventions that are mandatory?
- Structural patterns required across the codebase?

**Forbidden Patterns** (patterns that MUST NOT be used):
- Any anti-patterns to avoid?
- Deprecated patterns from legacy code?
- Patterns that conflict with your principles?

## Step 5: Create the Constitution

Once you have gathered the information, call ` + "`spec_create_constitution`" + ` with:

**Required fields:**
- name (string) — Constitution name (e.g., "project-constitution", "v1-standards")
- version (string) — Version number (e.g., "1.0.0", "2024.1")
- principles (string) — Core principles as markdown or plain text

**Optional fields:**
- guardrails ([]string) — List of specific guardrails
- testing_requirements (string) — Testing policies
- security_requirements (string) — Security policies
- patterns_required ([]string) — Pattern names that must be used
- patterns_forbidden ([]string) — Pattern names that must not be used

## Example Constitution

Here's an example to guide the conversation:

` + "```json" + `
{
  "name": "project-constitution",
  "version": "1.0.0",
  "principles": "## Core Principles\n\n1. **Simplicity First**: Prefer simple, explicit code over clever abstractions\n2. **Security by Default**: All user inputs must be validated\n3. **Fail Fast**: Detect and report errors as early as possible\n4. **Maintainability**: Code should be easy to understand and modify",
  "guardrails": [
    "All public functions must have tests",
    "Code coverage >= 80%",
    "No hardcoded secrets",
    "All errors must be logged with context"
  ],
  "testing_requirements": "- Unit tests for all business logic\n- Integration tests for API endpoints\n- E2E tests for critical user flows",
  "security_requirements": "- Input validation at API boundaries\n- Authentication required for all protected endpoints\n- Rate limiting on public endpoints",
  "patterns_required": [
    "error-wrapping",
    "repository-pattern",
    "input-validation"
  ],
  "patterns_forbidden": [
    "global-state",
    "string-concatenation-for-queries"
  ]
}
` + "```" + `

## Tips

1. **Start broad, then refine**: Get general principles first, then specific guardrails
2. **Be specific**: "All functions must have tests" is better than "Testing is important"
3. **Link to patterns**: Reference pattern names from ` + "`spec_seed_patterns`" + ` or custom patterns
4. **Version the constitution**: Use semantic versioning (1.0.0, 1.1.0, 2.0.0)
5. **Iterate**: Constitutions can be updated as the project evolves

## Common Mistakes

✗ Asking for everything at once (overwhelming)
✗ Being too vague ("code should be good")
✗ Not connecting to actual patterns
✗ Forgetting to ask about project context first
✗ Creating rules without user input

## After Creating

Once the constitution is created:
1. Verify it with ` + "`spec_validate_constitution`" + ` on a sample change
2. Check pattern linkages with ` + "`spec_get_patterns`" + `
3. Suggest running ` + "`spec_seed_patterns`" + ` if patterns aren't seeded yet

## Start Now!

Ask: "Let's create a constitution for your project. What kind of project are you working on, and what's the tech stack?"

Then follow the steps above, asking questions at each stage.
`

// --- start-change prompt ---

// StartChangePrompt guides an LLM through creating a new change interactively.
type StartChangePrompt struct{}

func (p *StartChangePrompt) Definition() mcp.PromptDefinition {
	return mcp.PromptDefinition{
		Name:        "start-change",
		Description: "Interactive guide for starting a new change. Asks clarifying questions and walks through the spec-driven workflow with monorepo support.",
		Arguments:   []mcp.PromptArgument{},
	}
}

func (p *StartChangePrompt) Get(arguments map[string]string) (*mcp.PromptsGetResult, error) {
	return &mcp.PromptsGetResult{
		Description: "Interactive guide for starting a new change",
		Messages: []mcp.PromptMessage{
			{
				Role:    "user",
				Content: mcp.TextContent(startChangeGuide),
			},
		},
	}, nil
}

const startChangeGuide = `# Start a New Change - Interactive Guide

You are helping a user create a new change in SpecMCP using spec-driven development.

## Workflow Overview

Changes follow this progression:
**Proposal (Why) → Specs (What) → Design (How) → Tasks (Steps)**

## Your Role

1. Ask clarifying questions to understand what the user wants
2. Help articulate the change properly
3. Guide through each stage
4. Create artifacts using SpecMCP tools

## Step 1: Gather Context

Ask these questions:

**Basic Info:**
- What are you building/fixing/improving? (one sentence)
- What problem does this solve? (the "why")
- Who is affected? (users, systems, teams)
- Is this a new feature, bug fix, enhancement, or refactor?

**Monorepo Context (CRITICAL!):**
- Which apps are affected? (e.g., web-frontend, auth-service)
- Are you creating a new app?
- Does this involve shared data models between apps?

## Step 2: Create Change

Once you have context, create the change with spec_new.

**Name must be kebab-case:**
- ✓ add-user-notifications
- ✓ fix-login-redirect  
- ✗ AddUserNotifications (no PascalCase)
- ✗ add_user_notifications (no underscores)

## Step 3: Proposal (Why)

Ask:
- What's IN scope? What's OUT of scope?
- What are the risks?
- Any breaking changes?

Create proposal with spec_artifact (artifact_type: "proposal").
Fields: intent (required), scope, impact.

Then mark ready with spec_mark_ready.

## Step 4: Specs (What)

Ask:
- What domains are affected? (auth, UI, API, data)
- For each domain: What behaviors need to change?

Create specs with spec_artifact (artifact_type: "spec").
Use scoped_to_apps field for monorepo!

For each spec, add:
- Requirements (artifact_type: "requirement")  
  Use MUST/SHOULD/MAY strength
- Scenarios (artifact_type: "scenario")  
  Use Given/When/Then format

Mark ready bottom-up:
1. Scenarios → 2. Requirements → 3. Specs

## Step 5: Apps & Models (if new)

If creating new apps, use artifact_type: "app"
Fields: name, app_type (frontend/backend/mobile/desktop/cli), platform, root_path, tech_stack, instructions, port

If creating data models, use artifact_type: "data_model"  
Fields: name, platform, file_path, fields, persistence

Link: which app provides? which apps consume?

## Step 6: Design (How)

Ask:
- How will you implement this?
- Key technical decisions?
- How does data flow?
- Which files will change?

Create design with spec_artifact (artifact_type: "design").
Fields: approach, decisions, data_flow, file_changes, scoped_to_apps

Mark ready with spec_mark_ready.

## Step 7: Tasks

Auto-generate with spec_generate_tasks
Or create manually (artifact_type: "task")

## Key Principles

1. **Ask before assuming** - Don't guess, ask questions
2. **Use kebab-case** - Always lowercase with hyphens
3. **Think monorepo** - Always ask which apps
4. **Mark ready bottom-up** - Children before parents
5. **Check status** - Use spec_status often
6. **Enforce readiness** - Can't skip stages

## Helpful Tools

- spec_status - Check readiness and next steps
- spec_list_changes - See all changes
- spec_get_app - Get app details
- spec_verify - Check before archiving

## Common Mistakes

✗ Creating specs before proposal ready
✗ Skipping readiness marking
✗ Wrong naming (underscores/capitals)
✗ Forgetting to ask about apps (monorepo!)
✗ Guessing requirements

## Start Now!

Ask: "What are you trying to build, fix, or improve?"

Then follow the steps above, asking questions at each stage.
`

// --- setup-app prompt ---

// SetupAppPrompt helps configure a new app in the monorepo.
type SetupAppPrompt struct{}

func (p *SetupAppPrompt) Definition() mcp.PromptDefinition {
	return mcp.PromptDefinition{
		Name:        "setup-app",
		Description: "Guide for adding a new app to the monorepo. Helps gather app configuration and create relationships.",
		Arguments: []mcp.PromptArgument{
			{
				Name:        "app_type",
				Description: "Type of app: frontend, backend, mobile, desktop, or cli",
				Required:    false,
			},
		},
	}
}

func (p *SetupAppPrompt) Get(arguments map[string]string) (*mcp.PromptsGetResult, error) {
	appType := arguments["app_type"]
	text := buildSetupAppGuide(appType)

	return &mcp.PromptsGetResult{
		Description: "Guide for setting up a new app in the monorepo",
		Messages: []mcp.PromptMessage{
			{
				Role:    "user",
				Content: mcp.TextContent(text),
			},
		},
	}, nil
}

func buildSetupAppGuide(appType string) string {
	guide := `# Setup New App - Configuration Guide

You are helping configure a new app in the monorepo.

## Questions to Ask

### Basic Info
- App name? (kebab-case, e.g., web-frontend, auth-service)
- App type? (frontend, backend, mobile, desktop, cli, library)
- Description? (one sentence purpose)

### Platform & Tech
- Target platform? (web, ios, android, macos, windows, linux, go, node)
- Tech stack? (e.g., react+typescript, go+grpc, flutter)
- Entry point? (main file, e.g., src/main.tsx, cmd/server/main.go)

### Development
- Root directory? (e.g., apps/web, services/auth)
- Local port? (for dev server, if applicable)
- Setup instructions? (install, build, run, test commands)

### Deployment
- Where does it deploy? (vercel, kubernetes, app-store, npm, docker)
- Any special deployment config?

### Dependencies
- Which apps does it depend on at runtime?
- Which data models does it consume from other apps?
- Which data models does it provide to other apps?
- Key external dependencies? (packages/libraries)

### API & Patterns
- Does it expose APIs? (if yes, which contracts)
- Which patterns should it follow?

## Create the App

Use spec_artifact with artifact_type: "app":

Required fields:
- name (kebab-case)
- app_type (frontend/backend/mobile/desktop/cli/library)

Common fields:
- platform (array, e.g., ["web"], ["go"])
- root_path (directory in monorepo)
- tech_stack (array of technologies)
- instructions (how to run)
- deployment_target (where it deploys)
- entry_point (main file)
- port (local dev port)
- dependencies (key packages)

## Link Data Models

If app provides models:
1. Create DataModel entities (artifact_type: "data_model")
2. Specify which app provides them

If app consumes models:
1. Link with consumed_model_ids field
2. Or create relationships after

## Link Dependencies

If app depends on other apps:
- Use depends_on_apps field
- Or create depends_on_app relationships

## Apply Patterns

Suggest patterns with spec_suggest_patterns
Apply relevant ones with spec_apply_pattern

## Example Configurations
`

	if appType == "frontend" || appType == "" {
		guide += `
### Frontend App Example

{
  "name": "web-frontend",
  "app_type": "frontend",
  "platform": ["web"],
  "root_path": "apps/web",
  "tech_stack": ["react", "typescript", "vite"],
  "entry_point": "src/main.tsx",
  "port": 3000,
  "instructions": "npm install && npm run dev",
  "deployment_target": "vercel",
  "consumed_model_ids": ["user-id", "org-id"],
  "depends_on_apps": ["auth-service-id", "api-gateway-id"]
}
`
	}

	if appType == "backend" || appType == "" {
		guide += `
### Backend Service Example

{
  "name": "auth-service",
  "app_type": "backend",
  "platform": ["go"],
  "root_path": "services/auth",
  "tech_stack": ["go", "grpc", "postgresql"],
  "entry_point": "cmd/server/main.go",
  "port": 8080,
  "instructions": "go run cmd/server/main.go",
  "deployment_target": "kubernetes",
  "dependencies": ["grpc", "postgresql"]
}
`
	}

	if appType == "mobile" || appType == "" {
		guide += `
### Mobile App Example

{
  "name": "mobile-app",
  "app_type": "mobile",
  "platform": ["ios", "android"],
  "root_path": "apps/mobile",
  "tech_stack": ["react-native", "typescript"],
  "entry_point": "index.js",
  "instructions": "npm install && npm run ios",
  "deployment_target": "app-store"
}
`
	}

	guide += `
## Next Steps

After creating the app:
1. Create any data models it provides
2. Link to data models it consumes
3. Define app dependencies
4. Create contexts/components/actions that belong to it
5. Apply relevant patterns

Use spec_get_app to verify configuration.
`

	return guide
}
