# Knowledge Contribution System via Improvements

## Core Concept

Instead of generic notes, agents create **specific, actionable Improvements** that contribute to project knowledge:
- Constitution rules
- Pattern definitions  
- Technology choices
- Best practices

An Improvement is the **draft proposal**. User reviews and approves. Once approved, it gets formalized into the actual entity (Constitution, Pattern, etc.).

## Extended Improvement Types

### Current Types (code-focused)
```
enhancement, refactor, optimization, bug_fix, tech_debt, cleanup, dx
```

### New Types (knowledge-focused)
```
constitution_rule    - Propose a new rule or constraint
constitution_update  - Modify existing Constitution principle
pattern_proposal     - Define a reusable pattern observed
technology_choice    - Document a technology decision
best_practice        - Capture a coding standard or practice
```

## Triggers → Improvement Creation

### Trigger 1: User States a Constraint

**User says**: "We never use class components in this project"

**Agent action**:
```json
{
  "type": "constitution_rule",
  "domain": "ui",
  "title": "Enforce functional components in React",
  "description": "User stated: 'We never use class components'. This should be a project-wide rule to ensure consistency.",
  "proposed_amendment": {
    "guardrails": ["all-react-components-functional"],
    "rationale": "Improves consistency, aligns with modern React patterns, easier testing"
  },
  "trigger_quote": "We never use class components in this project",
  "evidence": ["User corrected LoginForm.tsx from class to function"]
}
```

### Trigger 2: User Rejects Approach

**User says**: "No, don't use an ORM - we do raw SQL here"

**Agent action**:
```json
{
  "type": "constitution_rule",
  "domain": "data",
  "title": "Use raw SQL instead of ORMs",
  "description": "User rejected ORM usage. Project standard is raw SQL for database queries.",
  "proposed_amendment": {
    "guardrails": ["no-orm"],
    "patterns_forbidden": ["orm-pattern"],
    "rationale": "Direct control over queries, better performance visibility, explicit SQL"
  },
  "trigger_quote": "No, don't use an ORM - we do raw SQL here",
  "evidence": ["Attempted to use Prisma, user rejected", "Observed raw SQL in all existing handlers"]
}
```

### Trigger 3: Pattern Observed

**Agent notices**: All error handlers follow same structure

**Agent action**:
```json
{
  "type": "pattern_proposal",
  "domain": "api",
  "title": "Structured error handling pattern",
  "description": "Observed consistent error handling across 8 API handlers: validate → try/catch → log structured → return standard error response",
  "proposed_pattern": {
    "name": "structured-error-handler",
    "type": "error",
    "example_code": "try { ... } catch (err) { logger.error({ err, ctx }); return { error: err.message } }",
    "usage_guidance": "All API handlers should wrap business logic in try/catch with structured logging"
  },
  "evidence": ["handlers/auth.ts", "handlers/users.ts", "handlers/payments.ts"]
}
```

### Trigger 4: Technology Choice

**User chooses**: "Let's use Zod for validation"

**Agent action**:
```json
{
  "type": "technology_choice",
  "domain": "data",
  "title": "Use Zod for schema validation",
  "description": "User chose Zod as the validation library. Should be used consistently across the project.",
  "proposed_tech_stack_entry": {
    "technology": "Zod",
    "purpose": "Schema validation",
    "scope": "api, data models",
    "alternatives_rejected": ["Joi", "Yup"],
    "rationale": "Type-safe, TypeScript-native, good developer experience"
  },
  "trigger_quote": "Let's use Zod for validation"
}
```

### Trigger 5: Best Practice Emerges

**Agent observes**: User consistently requests certain code style

**Agent action**:
```json
{
  "type": "best_practice",
  "domain": "dx",
  "title": "Prefer explicit error types over generic Error",
  "description": "User corrected 3 instances to use custom error classes (ValidationError, AuthError) instead of generic Error",
  "proposed_practice": {
    "rule": "Define specific error types for each domain",
    "example": "throw new ValidationError('Invalid email') vs throw new Error('Invalid email')",
    "benefit": "Better error handling, clearer debugging, type-safe catch blocks"
  },
  "evidence": ["Corrected in auth.ts", "Corrected in validation.ts", "Corrected in api.ts"]
}
```

## Improvement Schema Extensions

```json
{
  "Improvement": {
    "properties": {
      "type": {
        "enum": [
          "enhancement", "refactor", "optimization", "bug_fix", "tech_debt", "cleanup", "dx",
          "constitution_rule", "constitution_update", "pattern_proposal", "technology_choice", "best_practice"
        ]
      },
      
      // New fields for knowledge contributions
      "trigger_quote": {
        "type": "string",
        "description": "Exact quote from user that triggered this improvement"
      },
      "evidence": {
        "type": "array",
        "items": {"type": "string"},
        "description": "Files, commits, or observations supporting this improvement"
      },
      "proposed_amendment": {
        "type": "object",
        "description": "For constitution_rule/update: structured proposal for Constitution changes"
      },
      "proposed_pattern": {
        "type": "object", 
        "description": "For pattern_proposal: pattern definition ready to formalize"
      },
      "proposed_tech_stack_entry": {
        "type": "object",
        "description": "For technology_choice: technology decision documentation"
      },
      "proposed_practice": {
        "type": "object",
        "description": "For best_practice: coding standard or guideline"
      }
    }
  }
}
```

## Workflow: Improvement → Formalization

### Step 1: Agent Creates Improvement

When trigger detected, create Improvement with status=proposed:

```
User: "We never use class components"
→ Agent creates Improvement (type: constitution_rule, status: proposed)
```

### Step 2: User Reviews

User sees proposed improvement:
```
New Constitution Rule Proposed:
  Title: Enforce functional components in React
  Trigger: "We never use class components"
  Amendment: Add guardrail "all-react-components-functional"
  
  Approve? [Y/n]
```

### Step 3: Formalization (if approved)

Once approved (status → completed), the Improvement is formalized:

```go
// For constitution_rule → Update Constitution
if improvement.Type == "constitution_rule" {
    constitution := GetConstitution()
    constitution.Guardrails = append(constitution.Guardrails, 
        improvement.ProposedAmendment.Guardrails...)
    UpdateConstitution(constitution)
    
    // Link improvement to constitution
    CreateRelationship("formalized_into", improvement.ID, constitution.ID)
}

// For pattern_proposal → Create Pattern
if improvement.Type == "pattern_proposal" {
    pattern := &Pattern{
        Name: improvement.ProposedPattern.Name,
        Type: improvement.ProposedPattern.Type,
        // ... populate from proposed_pattern
    }
    CreatePattern(pattern)
    
    CreateRelationship("formalized_into", improvement.ID, pattern.ID)
}
```

## Agent Prompting: When to Create Knowledge Improvements

### System Prompt Extension

```markdown
## Contributing to Project Knowledge

When certain events occur, create Improvements to capture project decisions and rules.

### Trigger: User States a Rule or Constraint

**Keywords**: "we never", "we always", "don't use", "we use", "this is required"

**Action**: Create improvement with type `constitution_rule`

Example:
User: "We never use any ORM in this project"
→ Create constitution_rule improvement titled "Prohibit ORM usage"

### Trigger: User Rejects Technology/Approach

**Keywords**: "no don't use X", "we don't do Y", "that's not how we"

**Action**: Create improvement with type `constitution_rule` or `technology_choice`

Example:
User: "No, we don't use Jest. We use Vitest."
→ Create technology_choice improvement documenting Vitest as standard

### Trigger: Pattern Emerges (3+ instances)

**Detection**: You notice the same code structure 3+ times

**Action**: Create improvement with type `pattern_proposal`

Example:
You see try/catch/log/return in 5 handlers
→ Create pattern_proposal for error handling pattern

### Trigger: User Corrects Your Code Style (2+ times)

**Detection**: User asks for same change type twice

**Action**: Create improvement with type `best_practice`

Example:
User changes snake_case to camelCase twice
→ Create best_practice improvement for naming convention

### How to Create

Use the `improvement_propose_knowledge` tool:

```json
{
  "type": "constitution_rule",
  "domain": "ui",
  "title": "Enforce functional components",
  "trigger_quote": "We never use class components",
  "proposed_amendment": {
    "guardrails": ["all-react-components-functional"]
  }
}
```

### DO:
- Capture user's exact words in trigger_quote
- Provide evidence (files, observations)
- Make specific, actionable proposals
- Create immediately when trigger occurs

### DON'T:
- Wait to "accumulate notes" - propose immediately
- Create vague or generic improvements
- Propose without clear trigger or evidence
```

## Tool: `improvement_propose_knowledge`

Specialized tool for knowledge contribution improvements:

```go
type ProposeKnowledgeInput struct {
    Type             string                 `json:"type"`              // constitution_rule, pattern_proposal, etc.
    Domain           string                 `json:"domain"`
    Title            string                 `json:"title"`
    Description      string                 `json:"description"`
    TriggerQuote     string                 `json:"trigger_quote"`
    Evidence         []string               `json:"evidence"`
    ProposedAmendment *ConstitutionAmendment `json:"proposed_amendment,omitempty"`
    ProposedPattern  *PatternDefinition     `json:"proposed_pattern,omitempty"`
    // ... other proposal types
}

func (t *ProposeKnowledgeTool) Execute(ctx context.Context, input ProposeKnowledgeInput) {
    improvement := &Improvement{
        Type:          input.Type,
        Domain:        input.Domain,
        Title:         input.Title,
        Description:   input.Description,
        Status:        StatusProposed,
        TriggerQuote:  input.TriggerQuote,
        Evidence:      input.Evidence,
        // ... set proposed_* field based on type
    }
    
    CreateImprovement(ctx, improvement)
    
    return fmt.Sprintf(
        "Knowledge improvement proposed: %s\n\nTrigger: \"%s\"\n\nUser can review and approve to formalize.",
        input.Title,
        input.TriggerQuote,
    )
}
```

## Benefits of This Approach

✅ **Specific**: Not generic notes, but actionable proposals  
✅ **Immediate**: Created when trigger occurs, not batched  
✅ **Traceable**: Links trigger quote → evidence → proposal  
✅ **Reviewable**: User approves before formalizing  
✅ **Formalized**: Direct path to Constitution/Pattern/etc  
✅ **Simple**: Reuses Improvement entity, just adds fields  

## Comparison

| Approach | Generic Notes | Knowledge Improvements |
|----------|---------------|----------------------|
| Structure | Freeform text | Structured proposals |
| Timing | Accumulate → compile | Immediate on trigger |
| Actionability | "Maybe useful later" | "Ready to formalize" |
| User workflow | Review batch weekly | Approve individually |
| Output | Unclear | Constitution/Pattern/Tech choice |

## Next Steps

1. Extend Improvement type enum with knowledge types
2. Add trigger_quote, evidence, proposed_* fields to schema
3. Create `improvement_propose_knowledge` tool
4. Update agent prompts with trigger detection guidance
5. Build formalization workflow (approved improvement → Constitution update)

What do you think of this more targeted approach?
