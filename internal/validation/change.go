package validation

import (
	"fmt"

	"github.com/emergent-company/specmcp/internal/emergent"
)

// Change state transitions map
var changeTransitions = map[string][]string{
	emergent.StatusActive:   {emergent.StatusArchived},
	emergent.StatusArchived: {emergent.StatusActive}, // Allow unarchiving
}

type changeValidator struct{}

// NewChangeValidator creates a validator for Change entities
func NewChangeValidator() Validator {
	return &changeValidator{}
}

func (v *changeValidator) Validate(from, to string, ctx *TransitionContext, changeID string) error {
	// Check if transition is allowed
	if !isAllowedTransition(from, to, changeTransitions) {
		return transitionError(from, to)
	}

	// Apply guards when archiving
	if to == emergent.StatusArchived {
		return v.guardArchive(ctx, changeID)
	}

	return nil
}

// guardArchive ensures all tasks are completed before archiving
func (v *changeValidator) guardArchive(ctx *TransitionContext, changeID string) error {
	if ctx.Force {
		return nil
	}

	// All tasks must be completed
	tasks, err := ctx.Client.GetRelatedObjects(ctx.Ctx, changeID, emergent.RelHasTask)
	if err != nil {
		return fmt.Errorf("failed to get tasks: %w", err)
	}

	for _, task := range tasks {
		status, ok := task.Properties["status"].(string)
		if !ok || status != emergent.StatusCompleted {
			return fmt.Errorf("cannot archive: task %s is %s", task.ID, status)
		}
	}

	return nil
}
