# Agent Memory & Notes System for SpecMCP

## Research Summary

Based on analysis of Mem0, modern agent memory architectures, and cognitive science patterns, here's a proposal for SpecMCP's memory/notes system.

## The Problem

Coding agents need a way to capture **unstructured observations** during work that don't fit into formal specifications:

- "User prefers snake_case over camelCase"
- "This API endpoint is rate-limited, saw 429 errors"  
- "Cannot use ORM in this project (hit error, user rejected)"
- "User wants minimal comments, code should be self-documenting"
- "This codebase uses optimistic locking for concurrency"

These are **episodic memories** (specific experiences) and emerging **semantic knowledge** (general rules) that should inform future work, but don't warrant:
- Immediate Constitution updates (too reactive)
- Full Change workflows (not structural changes)
- Pattern creation (not mature enough yet)

## Core Architecture: Memory Layers

### 1. **AgentNote** Entity (Episodic Memory)

Unstructured observations captured during agent work.

```go
type AgentNote struct {
    ID          string     `json:"id,omitempty"`
    Content     string     `json:"content"`         // Freeform observation
    Category    string     `json:"category"`        // capability, limitation, preference, pattern, error, insight
    Context     string     `json:"context"`         // What task was being done
    Importance  string     `json:"importance"`      // low, medium, high, critical
    CreatedAt   *time.Time `json:"created_at"`
    CreatedBy   string     `json:"created_by"`      // Agent name
    Tags        []string   `json:"tags,omitempty"`
    
    // Rich context (optional)
    RelatedChange     string `json:"related_change,omitempty"`     // Change ID if applicable
    RelatedFile       string `json:"related_file,omitempty"`       // File path
    RelatedEntity     string `json:"related_entity,omitempty"`     // Entity ID
    TriggerType       string `json:"trigger_type,omitempty"`       // What caused this note
}
```

### 2. **Memory Compilation** → Periodic Synthesis

Notes are **not immediately actionable**. Instead, they accumulate and are periodically compiled into:

- **Constitution updates** (when patterns emerge)
- **Pattern definitions** (when repeated solutions surface)
- **Improvement suggestions** (when inefficiencies are noted)
- **Knowledge base entries** (when facts solidify)

## Note-Taking Triggers for Coding Agents

Based on research, agents should create notes when:

### Category: Capability Discovery
**Trigger**: Agent successfully completes a new type of task
```
Content: "Successfully generated OpenAPI spec from TypeScript types using ts-to-openapi"
Category: capability
Importance: medium
TriggerType: success
```

### Category: Limitation Discovery  
**Trigger**: Agent encounters a blocker or constraint
```
Content: "Cannot modify files in /vendor directory - permission denied. Asked user, they confirmed it's read-only"
Category: limitation
Importance: high
TriggerType: error_resolution
```

### Category: User Preference
**Trigger**: User corrects agent's output or expresses preference
```
Content: "User prefers functional components over class components in React. Rewrote LoginForm as requested."
Category: preference
Importance: high
TriggerType: user_feedback
```

### Category: Pattern Observation
**Trigger**: Agent notices a repeated code structure
```
Content: "All API handlers in this codebase follow: validate → authorize → transform → execute → log pattern"
Category: pattern
Importance: medium
TriggerType: code_analysis
```

### Category: Error/Warning
**Trigger**: Unexpected behavior or environmental issue
```
Content: "Database connection pool exhausted at 100 concurrent users. Load testing revealed limit."
Category: error
Importance: critical
TriggerType: testing
```

### Category: Insight
**Trigger**: Agent makes a connection or realizes something important
```
Content: "The 'optimistic' tag in tasks indicates they need rollback handlers. Correlation with has_rollback relationship."
Category: insight
Importance: medium
TriggerType: reflection
```

## Prompting Strategy: When to Note Things

### System Prompt Addition (for coding agents)

```markdown
## Memory & Learning

You have access to a note-taking system for capturing observations that will help you improve over time.

### When to Create Notes

Create a note whenever you:
1. **Discover a capability**: You successfully do something new or use a tool/pattern for the first time
2. **Hit a limitation**: You encounter something you cannot do, or the user tells you a constraint
3. **Receive feedback**: The user corrects your work or expresses a preference
4. **Observe a pattern**: You notice the codebase consistently does things a certain way
5. **Encounter errors**: Unexpected behavior, failed tests, or environmental issues
6. **Have an insight**: You make a connection between different parts of the system

### Note-Taking Guidelines

- **Be specific**: Include context, files, error messages, or user quotes
- **Be concise**: One clear observation per note (not essays)
- **Tag relevantly**: Use tags like file paths, entity types, or domain areas
- **Set importance**: 
  - Critical: Blocks work or causes bugs
  - High: User preference or significant constraint
  - Medium: Useful pattern or insight
  - Low: Minor observation

### DO NOT:
- Immediately update the Constitution based on a single observation
- Create patterns after one instance
- Over-note routine successful operations

Instead, let notes accumulate. They will be periodically reviewed and compiled into formal knowledge.

### Example

❌ **Bad** (after one instance):
> Updates Constitution: "All components must be functional"

✅ **Good** (creates note):
```json
{
  "content": "User prefers functional components. Converted LoginForm from class to function.",
  "category": "preference",
  "importance": "high",
  "related_file": "src/components/LoginForm.tsx"
}
```
```

## Compilation/Synthesis Process

### Phase 1: Automatic Clustering (Weekly/On-Demand)

An agent reviews accumulated notes and identifies clusters:

```
Tool: compile_notes

Input:
- time_range: last_7_days
- min_cluster_size: 3

Output:
- "Pattern: All error handlers log to structured logger"
  - 8 notes referencing this
  - Suggest: Create pattern "structured-error-logging"
  
- "Preference: User consistently requests snake_case"
  - 5 notes about naming corrections
  - Suggest: Update Constitution naming conventions
  
- "Limitation: Cannot access production database"
  - 3 notes hitting this
  - Suggest: Document in project README
```

### Phase 2: Proposal Generation

Compilation generates:
1. **Improvement** for quick fixes
2. **Pattern** for repeated solutions
3. **Constitution amendment** for established rules
4. **Documentation update** for facts/constraints

### Phase 3: User Review

User receives summary:
```
Weekly Note Compilation (23 notes reviewed)

Proposed Actions:
1. Add Pattern: "structured-error-logging" (8 occurrences)
2. Update Constitution: Enforce snake_case naming (5 corrections)
3. Create Improvement: Document database access limitations (3 blockers)

Approve all? [Y/n]
Review individually? [y/N]
```

## Storage & Retrieval

### Storage (in Emergent Graph)

```go
// Relationships
RelNotedDuring      = "noted_during"       // AgentNote → Change (context)
RelNotesEntity      = "notes_entity"       // AgentNote → Entity (references)
RelCompiledInto     = "compiled_into"      // AgentNote → Pattern/Constitution/Improvement
RelSupersededBy     = "superseded_by"      // AgentNote → AgentNote (newer observation)
```

### Retrieval (for agent context)

Before starting a task, agent retrieves relevant notes:

```
Query: Get recent notes for:
- Files I'm about to modify
- Similar tasks (semantic search)
- This Change's scope
- High importance, uncompleted notes
```

This gives the agent **learned context** without bloating the Constitution.

## Implementation Plan

### Entity: AgentNote

```json
{
  "AgentNote": {
    "type": "object",
    "required": ["content", "category", "importance", "created_at", "created_by"],
    "properties": {
      "content": {"type": "string"},
      "category": {
        "type": "string",
        "enum": ["capability", "limitation", "preference", "pattern", "error", "insight"]
      },
      "importance": {
        "type": "string",
        "enum": ["low", "medium", "high", "critical"]
      },
      "context": {"type": "string"},
      "created_at": {"type": "string", "format": "date-time"},
      "created_by": {"type": "string"},
      "related_change": {"type": "string"},
      "related_file": {"type": "string"},
      "related_entity": {"type": "string"},
      "trigger_type": {
        "type": "string",
        "enum": ["success", "error_resolution", "user_feedback", "code_analysis", "testing", "reflection"]
      },
      "compiled": {"type": "boolean", "default": false},
      "tags": {"type": "array", "items": {"type": "string"}}
    }
  }
}
```

### Tools

```go
// internal/tools/memory/
note_create.go       // Create a note during work
note_list.go         // Query notes by filters
note_compile.go      // Trigger compilation/synthesis
note_archive.go      // Mark notes as compiled/obsolete
```

### Compilation Agent (Separate Tool/Workflow)

```
Tool: compile_agent_notes

1. Retrieve uncompiled notes from last N days
2. Use LLM to cluster by semantic similarity
3. For each cluster:
   - Identify pattern/rule/preference
   - Generate proposal (Pattern, Constitution amendment, Improvement)
4. Present to user for approval
5. Mark compiled notes as processed
```

## Comparison to Mem0

| Feature | Mem0 | SpecMCP Memory |
|---------|------|----------------|
| Storage | Vector DB + Graph DB | Emergent Graph (native) |
| Extraction | LLM extracts "facts" | Explicit note creation |
| Update | ADD/UPDATE/DELETE/NOOP | Accumulate + periodic compilation |
| Retrieval | Semantic search | Graph relationships + semantic |
| Forgetting | Dynamic decay | Manual compilation & archival |
| Use case | General conversational memory | Coding agent episodic learning |

## Benefits

✅ **Non-disruptive**: Notes don't immediately change project rules  
✅ **Evidence-based**: Patterns emerge from multiple observations  
✅ **Transparent**: User sees what agent learned  
✅ **Contextual**: Notes linked to Changes, files, entities  
✅ **Compilable**: Periodic synthesis into actionable items  
✅ **Lightweight**: Simple entity, no complex ML pipeline  

## Example Workflow

```
Day 1: Agent working on auth feature
- Creates note: "JWT secret must be 32+ chars (validation error)"
- Creates note: "User prefers Zod for validation schemas"

Day 3: Agent working on payments
- Creates note: "User prefers Zod (used for payment schema validation)"

Day 7: Compilation run
- Cluster detected: 2 notes about Zod preference
- Suggestion: Create Improvement "standardize-on-zod"

User approves → Improvement created → Notes marked compiled
```

## Next Steps

Would you like me to:
1. Add AgentNote entity to template pack
2. Create note-taking tools
3. Design the compilation agent workflow
4. Add prompting guidance for when to create notes
