package validation

import (
	"fmt"

	"github.com/emergent-company/specmcp/internal/emergent"
)

// Improvement state transitions map
var improvementTransitions = map[string][]string{
	emergent.StatusProposed:   {"planned", "deferred", "rejected"},
	"planned":                 {"in_progress", "deferred"},
	emergent.StatusInProgress: {"completed", "deferred"},
	"deferred":                {emergent.StatusProposed},
	emergent.StatusCompleted:  {}, // terminal
	"rejected":                {}, // terminal
}

type improvementValidator struct{}

// NewImprovementValidator creates a validator for Improvement entities
func NewImprovementValidator() Validator {
	return &improvementValidator{}
}

func (v *improvementValidator) Validate(from, to string, ctx *TransitionContext, improvementID string) error {
	// Check if transition is allowed
	if !isAllowedTransition(from, to, improvementTransitions) {
		return transitionError(from, to)
	}

	// Apply guards based on target state
	switch to {
	case "planned":
		return v.guardPlanned(ctx, improvementID)
	case emergent.StatusInProgress:
		return v.guardInProgress(ctx, improvementID)
	case emergent.StatusCompleted:
		return v.guardCompleted(ctx, improvementID)
	}

	return nil
}

// guardPlanned ensures improvement has tasks before marking as planned
func (v *improvementValidator) guardPlanned(ctx *TransitionContext, improvementID string) error {
	if ctx.Force {
		return nil
	}

	// Must have at least one task
	tasks, err := ctx.Client.GetRelatedObjects(ctx.Ctx, improvementID, emergent.RelHasTask)
	if err != nil {
		return fmt.Errorf("failed to get tasks: %w", err)
	}

	if len(tasks) == 0 {
		return ErrNeedTasks
	}

	return nil
}

// guardInProgress is a placeholder for future guards when starting work
func (v *improvementValidator) guardInProgress(ctx *TransitionContext, improvementID string) error {
	// Currently no guards for starting work
	// Could add: check if assigned to an agent, check if effort/priority set, etc.
	return nil
}

// guardCompleted ensures all tasks are completed
func (v *improvementValidator) guardCompleted(ctx *TransitionContext, improvementID string) error {
	if ctx.Force {
		return nil
	}

	// All tasks must be completed
	tasks, err := ctx.Client.GetRelatedObjects(ctx.Ctx, improvementID, emergent.RelHasTask)
	if err != nil {
		return fmt.Errorf("failed to get tasks: %w", err)
	}

	for _, task := range tasks {
		status, ok := task.Properties["status"].(string)
		if !ok || status != emergent.StatusCompleted {
			return fmt.Errorf("%w: task %s is %s", ErrTasksIncomplete, task.ID, status)
		}
	}

	return nil
}
