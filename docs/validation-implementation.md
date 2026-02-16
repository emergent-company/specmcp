# State Transition Validation - Implementation Summary

## What We Built

Simple, dependency-free state transition validation for SpecMCP entities.

**Total code: 401 lines across 5 files**

### Package Structure

```
internal/validation/
├── transitions.go (104 lines)      # Registry, interfaces, common helpers
├── improvement.go (91 lines)       # Improvement state validation
├── task.go (59 lines)              # Task state validation
├── workflow_artifact.go (91 lines) # Proposal/Spec/Requirement/Scenario/Design
├── change.go (56 lines)            # Change state validation
└── README.md                        # Documentation and usage examples
```

## Entities Covered

1. **Improvement** (new) - 6 states with guards for planning and completion
2. **Task** - 4 states with subtask completion guard
3. **Change** - 2 states with task completion guard  
4. **Workflow Artifacts** (Proposal, Spec, Requirement, Scenario, Design) - 2 states with cascade validation

## Key Features

✅ **Explicit transition maps** - Clear allowed state changes  
✅ **Guard functions** - Enforce business rules (tasks must exist, children must be ready)  
✅ **Force flag** - Bypass soft guards when needed  
✅ **Centralized** - Single source of truth for all state logic  
✅ **Zero dependencies** - No external packages  
✅ **Easy to test** - Pure functions with clear inputs/outputs  
✅ **Type-safe** - Uses entity type constants from emergent package  

## Example Transition Map

```go
// Improvement state transitions
var improvementTransitions = map[string][]string{
    "proposed":    {"planned", "deferred", "rejected"},
    "planned":     {"in_progress", "deferred"},
    "in_progress": {"completed", "deferred"},
    "deferred":    {"proposed"},
    "completed":   {}, // terminal
    "rejected":    {}, // terminal
}
```

## Example Guard Function

```go
// guardPlanned ensures improvement has tasks before marking as planned
func (v *improvementValidator) guardPlanned(ctx *TransitionContext, improvementID string) error {
    if ctx.Force {
        return nil // Allow if force=true
    }

    tasks, _ := ctx.Client.GetRelatedObjects(ctx.Ctx, improvementID, emergent.RelHasTask)
    if len(tasks) == 0 {
        return ErrNeedTasks
    }

    return nil
}
```

## Usage Pattern

```go
// Create registry
registry := validation.NewRegistry()

// Validate a transition
err := registry.Validate(
    emergent.TypeImprovement,  // entity type
    "proposed",                 // current state
    "planned",                  // desired state
    &validation.TransitionContext{
        Client: client,
        Ctx:    ctx,
        Force:  false,
    },
    improvementID,
)

if err != nil {
    // Transition not allowed
    return err
}

// Transition is valid, proceed with update
```

## Added to types.go

```go
const (
    TypeImprovement = "Improvement"  // New entity type
    StatusProposed  = "proposed"     // New status constant
)
```

## Why Simple Validation Won This Design

| Consideration | Simple Validation | State Machine Library |
|---------------|-------------------|----------------------|
| Lines of code | 401 | ~500+ (lib + usage) |
| Dependencies | 0 | 1 (qmuntal/stateless) |
| Complexity | Low | Medium |
| Our use case fit | Perfect | Overkill |
| Learning curve | Minutes | Hours |
| Debugging | Trivial | Harder |
| Visualization | Manual (simple diagrams) | Auto-generated |
| Extensibility | Easy | Easy |

## When to Reconsider

We should revisit this decision if:
- Any entity exceeds 10 states
- We need hierarchical/nested states
- We want runtime state machine introspection
- We build event-driven workflows
- We need automatic diagram generation

Until then, this simple approach is perfect.

## Next Steps

1. ✅ Validation package created
2. ⏭️ Add Improvement entity type to template pack
3. ⏭️ Create Improvement CRUD operations in entities.go
4. ⏭️ Create tools for Improvement management
5. ⏭️ Integrate validation into existing tools (spec_mark_ready, spec_archive, etc.)
6. ⏭️ Add unit tests for validators

## Files Modified

- `internal/emergent/types.go` - Added TypeImprovement and StatusProposed constants
- Created `internal/validation/` package (5 new files)

## Files to Create Next

- `templates/specmcp-pack.json` - Add Improvement schema
- `internal/emergent/entities.go` - Add Improvement CRUD functions
- `internal/tools/improvement/` - Add improvement management tools
