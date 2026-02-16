# Improvement Creation Guide for Agents

This document provides guidance for when and how to create improvements.

## Single Tool: `improvement_create`

One tool handles all improvement types. The **type** field determines what kind of improvement it is.

## Improvement Types

### Code Improvements (implementation work)

| Type | When to Use | Example |
|------|-------------|---------|
| `enhancement` | Add new capability to existing feature | "Add loading spinner to user list" |
| `refactor` | Restructure code without changing behavior | "Extract duplicate validation logic to helper" |
| `optimization` | Improve performance or efficiency | "Add database index to speed up queries" |
| `bug_fix` | Fix incorrect behavior | "Fix off-by-one error in pagination" |
| `tech_debt` | Address accumulated technical debt | "Replace deprecated API calls" |
| `cleanup` | Remove unused code, standardize formatting | "Remove unused imports across codebase" |
| `dx` | Developer experience improvements | "Add better error messages to CLI" |

### Knowledge Contributions (capture project decisions)

| Type | When to Use | Required Fields | Example |
|------|-------------|-----------------|---------|
| `constitution_rule` | User states a rule/constraint | `trigger_quote`, `proposed_amendment` | User says: "We never use ORMs" |
| `pattern_proposal` | Same code structure seen 3+ times | `evidence`, `proposed_pattern` | Error handling pattern in 5 files |
| `technology_choice` | User chooses/rejects technology | `trigger_quote`, `proposed_tech_choice` | User: "Use Zod for validation" |
| `best_practice` | User corrects style 2+ times | `evidence`, `proposed_best_practice` | User changes naming convention twice |

## Triggers for Knowledge Contributions

### Trigger 1: User States Rule/Constraint

**Detect these keywords:**
- "we never..."
- "we always..."
- "don't use..."
- "we use..."
- "this is required..."
- "this is prohibited..."

**Example:**
```
User: "We never use class components in this project"
```

**Action:**
```json
{
  "type": "constitution_rule",
  "domain": "ui",
  "title": "Enforce functional components in React",
  "description": "User stated requirement to never use class components. This should be enforced project-wide.",
  "trigger_quote": "We never use class components in this project",
  "proposed_amendment": {
    "guardrails": ["all-react-components-functional"],
    "rationale": "Consistency, modern React patterns, easier testing"
  },
  "evidence": ["User corrected LoginForm.tsx from class to function"],
  "importance": "high"
}
```

### Trigger 2: User Rejects Approach

**Detect these keywords:**
- "no, don't use..."
- "we don't do..."
- "that's not how we..."
- "stop using..."

**Example:**
```
User: "No, don't use an ORM - we do raw SQL here"
```

**Action:**
```json
{
  "type": "constitution_rule",
  "domain": "data",
  "title": "Use raw SQL instead of ORMs",
  "description": "User explicitly rejected ORM usage. Project uses raw SQL for all database operations.",
  "trigger_quote": "No, don't use an ORM - we do raw SQL here",
  "proposed_amendment": {
    "guardrails": ["no-orm"],
    "patterns_forbidden": ["orm-pattern"],
    "rationale": "Direct control over queries, performance visibility, explicit SQL"
  },
  "evidence": ["Attempted to use Prisma, user rejected"],
  "importance": "high"
}
```

### Trigger 3: Pattern Observed 3+ Times

**Detection:**
You notice the same code structure in 3 or more places.

**Example:**
You see this in 5 API handlers:
```typescript
try {
  // handler logic
} catch (err) {
  logger.error({ err, ctx, endpoint });
  return { error: err.message };
}
```

**Action:**
```json
{
  "type": "pattern_proposal",
  "domain": "api",
  "title": "Structured error handling pattern",
  "description": "Consistent error handling pattern observed across 5 API handlers: try/catch with structured logging and standard error response",
  "proposed_pattern": {
    "name": "structured-error-handler",
    "type": "error",
    "example_code": "try { ... } catch (err) { logger.error({ err, ctx, endpoint }); return { error: err.message }; }",
    "usage_guidance": "All API handlers should use try/catch with structured logging including error, context, and endpoint"
  },
  "evidence": [
    "handlers/auth.ts:45",
    "handlers/users.ts:78",
    "handlers/payments.ts:34",
    "handlers/products.ts:56",
    "handlers/orders.ts:23"
  ],
  "importance": "medium"
}
```

### Trigger 4: User Chooses Technology

**Detect these keywords:**
- "use X"
- "switch to Y"
- "let's use Z"
- "prefer A over B"

**Example:**
```
User: "Let's use Zod for validation"
```

**Action:**
```json
{
  "type": "technology_choice",
  "domain": "data",
  "title": "Use Zod for schema validation",
  "description": "User chose Zod as the validation library. Should be used consistently across API and data models.",
  "trigger_quote": "Let's use Zod for validation",
  "proposed_tech_choice": {
    "technology": "Zod",
    "purpose": "Schema validation",
    "scope": "api, data models",
    "alternatives_considered": "Joi, Yup",
    "rationale": "Type-safe, TypeScript-native, good DX"
  },
  "importance": "high"
}
```

### Trigger 5: User Corrects Style 2+ Times

**Detection:**
User asks for the same type of change in 2 or more places.

**Examples:**
- Changes snake_case to camelCase twice
- Adds type annotations twice
- Requests explicit error types instead of generic Error

**Example:**
User corrects you 3 times to use custom error classes:
```typescript
// You wrote:
throw new Error("Invalid email");

// User changed to:
throw new ValidationError("Invalid email");
```

**Action:**
```json
{
  "type": "best_practice",
  "domain": "dx",
  "title": "Use explicit error types over generic Error",
  "description": "User corrected 3 instances to use custom error classes (ValidationError, AuthError, NotFoundError) instead of generic Error",
  "proposed_best_practice": {
    "rule": "Define and use specific error types for each domain instead of generic Error",
    "example": "throw new ValidationError('Invalid email') instead of throw new Error('Invalid email')",
    "benefit": "Better error handling, clearer debugging, type-safe catch blocks"
  },
  "evidence": [
    "Corrected in auth.ts:45",
    "Corrected in validation.ts:78",
    "Corrected in api.ts:123"
  ],
  "importance": "medium"
}
```

## When NOT to Create Improvements

❌ **Don't create for:**
- Routine successful operations ("Successfully created file")
- Temporary workarounds ("Used console.log for debugging")
- Single instances without repetition
- Vague or unclear observations

✅ **DO create for:**
- Clear user-stated rules or preferences
- Repeated patterns (3+ occurrences)
- Technology choices or rejections
- Consistent user corrections (2+ times)

## Field Usage Guidelines

### Required for All Improvements
- `type` - One of the 11 types above
- `domain` - What area (ui, ux, performance, security, api, data, testing, infrastructure, documentation, accessibility)
- `title` - Short, clear summary
- `description` - Detailed explanation

### Required for Knowledge Types
- `trigger_quote` - Exact user words (for constitution_rule, technology_choice)
- `evidence` - List of files/observations (for pattern_proposal, best_practice)
- `proposed_*` - Structured proposal matching the type

### Optional
- `effort` - Size estimate (trivial/small/medium/large)
- `priority` - Urgency (low/medium/high/critical)
- `tags` - Additional context tags

## Workflow After Creation

1. **Agent creates improvement** with status: `proposed`
2. **User reviews** the proposal
3. **User approves** → status: `completed`, formalized into Constitution/Pattern/etc.
4. **User rejects** → status: `rejected` with reason

## Examples: Code vs Knowledge Improvements

### Code Improvement (standard)
```json
{
  "type": "refactor",
  "domain": "api",
  "title": "Extract validation logic to middleware",
  "description": "Duplicate validation code in 4 endpoints should be extracted to reusable middleware",
  "effort": "small",
  "priority": "medium"
}
```

### Knowledge Improvement (captures decision)
```json
{
  "type": "constitution_rule",
  "domain": "security",
  "title": "Require authentication on all API endpoints",
  "description": "User stated all endpoints must be authenticated. No public endpoints allowed.",
  "trigger_quote": "All our API endpoints must be authenticated, no exceptions",
  "proposed_amendment": {
    "security_requirements": "All API endpoints MUST require authentication. No public endpoints are permitted.",
    "guardrails": ["all-endpoints-authenticated"]
  },
  "importance": "critical"
}
```

## Summary: One Tool, Clear Guidance

- **One tool**: `improvement_create`
- **11 types**: 7 code-focused + 4 knowledge-focused
- **Clear triggers**: User statements, patterns, corrections
- **Structured proposals**: Ready to formalize when approved
- **Immediate action**: Create when trigger detected, not batched
