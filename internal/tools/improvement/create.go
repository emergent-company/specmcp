package improvement

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/emergent-company/specmcp/internal/emergent"
	"github.com/emergent-company/specmcp/internal/mcp"
)

type CreateTool struct {
	factory *emergent.ClientFactory
}

func NewCreateTool(factory *emergent.ClientFactory) *CreateTool {
	return &CreateTool{factory: factory}
}

func (t *CreateTool) Name() string {
	return "improvement_create"
}

func (t *CreateTool) Description() string {
	return `Create an improvement proposal. Use this for both code improvements and knowledge contributions.

CODE IMPROVEMENTS (implementation work):
- enhancement: Add new capability to existing feature
- refactor: Restructure code without changing behavior
- optimization: Improve performance/efficiency
- bug_fix: Fix incorrect behavior
- tech_debt: Address accumulated technical debt
- cleanup: Remove unused code, standardize formatting
- dx: Developer experience improvements

KNOWLEDGE CONTRIBUTIONS (capture project decisions):
- constitution_rule: Propose new project rule/constraint
  → Use when user says "we never", "we always", "required", "prohibited"
  → Provide: trigger_quote, proposed_amendment
  
- pattern_proposal: Define observed code pattern
  → Use when you see same structure 3+ times
  → Provide: evidence (file list), proposed_pattern
  
- technology_choice: Document tech decision
  → Use when user chooses/rejects a technology
  → Provide: trigger_quote, proposed_tech_choice
  
- best_practice: Capture coding standard
  → Use when user corrects your style 2+ times
  → Provide: evidence, proposed_best_practice

EXAMPLES:
User says "We never use class components" → type: constitution_rule
You see same error pattern in 5 files → type: pattern_proposal
User says "Use Zod for validation" → type: technology_choice`
}

func (t *CreateTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"required": ["type", "domain", "title", "description"],
		"properties": {
			"type": {
				"type": "string",
				"enum": [
					"enhancement", "refactor", "optimization", "bug_fix", "tech_debt", "cleanup", "dx",
					"constitution_rule", "pattern_proposal", "technology_choice", "best_practice"
				],
				"description": "Type of improvement"
			},
			"domain": {
				"type": "string",
				"enum": ["ui", "ux", "performance", "security", "api", "data", "testing", "infrastructure", "documentation", "accessibility"],
				"description": "What area this affects"
			},
			"title": {
				"type": "string",
				"description": "Short, clear summary"
			},
			"description": {
				"type": "string",
				"description": "Detailed explanation"
			},
			"effort": {
				"type": "string",
				"enum": ["trivial", "small", "medium", "large"],
				"description": "Size estimate (optional)"
			},
			"priority": {
				"type": "string",
				"enum": ["low", "medium", "high", "critical"],
				"description": "Urgency level (optional)"
			},
			"trigger_quote": {
				"type": "string",
				"description": "Exact user quote (required for knowledge types)"
			},
			"evidence": {
				"type": "array",
				"items": {"type": "string"},
				"description": "Files or observations supporting this"
			},
			"proposed_amendment": {
				"type": "object",
				"description": "For constitution_rule: Constitution changes"
			},
			"proposed_pattern": {
				"type": "object",
				"description": "For pattern_proposal: Pattern definition"
			},
			"proposed_tech_choice": {
				"type": "object",
				"description": "For technology_choice: Tech decision"
			},
			"proposed_best_practice": {
				"type": "object",
				"description": "For best_practice: Coding standard"
			},
			"tags": {
				"type": "array",
				"items": {"type": "string"},
				"description": "Additional tags"
			}
		}
	}`)
}

type CreateInput struct {
	Type                 string                 `json:"type"`
	Domain               string                 `json:"domain"`
	Title                string                 `json:"title"`
	Description          string                 `json:"description"`
	Effort               string                 `json:"effort,omitempty"`
	Priority             string                 `json:"priority,omitempty"`
	TriggerQuote         string                 `json:"trigger_quote,omitempty"`
	Evidence             []string               `json:"evidence,omitempty"`
	ProposedAmendment    map[string]interface{} `json:"proposed_amendment,omitempty"`
	ProposedPattern      map[string]interface{} `json:"proposed_pattern,omitempty"`
	ProposedTechChoice   map[string]interface{} `json:"proposed_tech_choice,omitempty"`
	ProposedBestPractice map[string]interface{} `json:"proposed_best_practice,omitempty"`
	Tags                 []string               `json:"tags,omitempty"`
}

func (t *CreateTool) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
	var input CreateInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	client, err := t.factory.ClientFor(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting client: %w", err)
	}

	now := time.Now()
	improvement := &emergent.Improvement{
		Title:                input.Title,
		Description:          input.Description,
		Domain:               input.Domain,
		Type:                 input.Type,
		Effort:               input.Effort,
		Priority:             input.Priority,
		Status:               emergent.StatusProposed,
		ProposedAt:           &now,
		ProposedBy:           "agent", // TODO: Get from context
		Tags:                 input.Tags,
		TriggerQuote:         input.TriggerQuote,
		Evidence:             input.Evidence,
		ProposedAmendment:    input.ProposedAmendment,
		ProposedPattern:      input.ProposedPattern,
		ProposedTechChoice:   input.ProposedTechChoice,
		ProposedBestPractice: input.ProposedBestPractice,
	}

	created, err := client.CreateImprovement(ctx, improvement)
	if err != nil {
		return nil, fmt.Errorf("creating improvement: %w", err)
	}

	// Build response message
	msg := fmt.Sprintf("Created improvement: %s\n\nType: %s\nDomain: %s\nStatus: %s\n",
		created.Title, created.Type, created.Domain, created.Status)

	if created.TriggerQuote != "" {
		msg += fmt.Sprintf("\nTriggered by: \"%s\"\n", created.TriggerQuote)
	}

	if len(created.Evidence) > 0 {
		msg += fmt.Sprintf("\nEvidence: %v\n", created.Evidence)
	}

	msg += "\nNext: User can review and approve this proposal."

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{{
			Type: "text",
			Text: msg,
		}},
	}, nil
}
