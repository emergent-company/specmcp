# Simple State Transition Validation

This package provides lightweight validation for state transitions across all SpecMCP entities.

## Architecture

```
internal/validation/
├── transitions.go          # Core types, registry, common helpers
├── improvement.go          # Improvement state validation
├── task.go                 # Task state validation  
├── workflow_artifact.go    # Proposal/Spec/Requirement/Scenario/Design validation
└── change.go               # Change state validation
```

## Design Principles

1. **Simple** - No external dependencies, just maps and functions
2. **Explicit** - Transition maps clearly show allowed state changes
3. **Testable** - Pure functions easy to unit test
4. **Centralized** - All state logic in one place
5. **Extensible** - Easy to add new validators

## State Machines

### Improvement States

```
proposed ──plan──> planned ──start──> in_progress ──complete──> completed
   │                  │                    │
   ├──defer──>     defer             defer
   │                  │                    │
   └──reject──>    deferred <──resume─────┘
   
   rejected (terminal)
```

**Guards:**
- `proposed → planned`: Must have at least one task (unless force=true)
- `in_progress → completed`: All tasks must be completed (unless force=true)

### Task States

```
pending ──start──> in_progress ──complete──> completed
   │                    │
   └──block──>      block
                       │
                   blocked <──unblock─────┘
```

**Guards:**
- `in_progress → completed`: All subtasks must be completed (unless force=true)

### Workflow Artifact States (Proposal, Spec, Requirement, Scenario, Design)

```
draft ──mark_ready──> ready
  ↑                      │
  └────── unready ───────┘
```

**Guards:**
- `draft → ready` for Spec: All requirements must be ready
- `draft → ready` for Requirement: All scenarios must be ready

### Change States

```
active ──archive──> archived
  ↑                     │
  └───── unarchive ─────┘
```

**Guards:**
- `active → archived`: All tasks must be completed (unless force=true)

## Usage Example

```go
import (
    "context"
    "github.com/emergent-company/specmcp/internal/validation"
    "github.com/emergent-company/specmcp/internal/emergent"
)

// In a tool's Execute method
func (t *ImprovementMarkPlannedTool) Execute(ctx context.Context, params json.RawMessage) error {
    // Parse input
    var input struct {
        ImprovementID string `json:"improvement_id"`
        Force         bool   `json:"force"`
    }
    json.Unmarshal(params, &input)
    
    // Get current improvement
    improvement, _ := t.client.GetObject(ctx, input.ImprovementID)
    currentStatus := improvement.Properties["status"].(string)
    
    // Prepare validation context
    valCtx := &validation.TransitionContext{
        Client: t.client,
        Ctx:    ctx,
        Force:  input.Force,
    }
    
    // Validate transition
    registry := validation.NewRegistry()
    err := registry.Validate(
        emergent.TypeImprovement,
        currentStatus,
        "planned",
        valCtx,
        input.ImprovementID,
    )
    
    if err != nil {
        return fmt.Errorf("cannot transition to planned: %w", err)
    }
    
    // Update status
    improvement.Properties["status"] = "planned"
    improvement.Properties["planned_at"] = time.Now().Format(time.RFC3339)
    t.client.UpdateObject(ctx, improvement)
    
    return nil
}
```

## Adding a New Validator

1. Create a new file in `internal/validation/` (e.g., `mynewtype.go`)
2. Define the state transition map
3. Implement the validator interface
4. Register it in `transitions.go` → `NewRegistry()`

Example:

```go
// mynewtype.go
package validation

import "github.com/emergent-company/specmcp/internal/emergent"

var myNewTypeTransitions = map[string][]string{
    "state1": {"state2", "state3"},
    "state2": {"state3"},
    "state3": {},
}

type myNewTypeValidator struct{}

func NewMyNewTypeValidator() Validator {
    return &myNewTypeValidator{}
}

func (v *myNewTypeValidator) Validate(from, to string, ctx *TransitionContext, entityID string) error {
    if !isAllowedTransition(from, to, myNewTypeTransitions) {
        return transitionError(from, to)
    }
    
    // Add guards here
    switch to {
    case "state3":
        return v.guardState3(ctx, entityID)
    }
    
    return nil
}

func (v *myNewTypeValidator) guardState3(ctx *TransitionContext, entityID string) error {
    // Custom validation logic
    return nil
}
```

Then register in `NewRegistry()`:

```go
r.Register(emergent.TypeMyNewType, NewMyNewTypeValidator())
```

## Benefits Over State Machine Library

1. **Zero dependencies** - No external packages required
2. **~200 lines total** - Simple enough to understand fully
3. **Easy to debug** - No hidden magic, clear control flow
4. **Fast** - No reflection, minimal allocations
5. **Flexible** - Easy to add custom validation logic

## When to Upgrade

Consider adding a state machine library (like qmuntal/stateless) if:
- We add 10+ states per entity
- We need hierarchical/nested states
- We want auto-generated state diagrams
- We build event-driven workflows
- We need state machine introspection at runtime
