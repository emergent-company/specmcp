package validation

import (
	"fmt"

	"github.com/emergent-company/specmcp/internal/emergent"
)

// Task state transitions map
var taskTransitions = map[string][]string{
	emergent.StatusPending:    {emergent.StatusInProgress, emergent.StatusBlocked},
	emergent.StatusInProgress: {emergent.StatusCompleted, emergent.StatusBlocked},
	emergent.StatusBlocked:    {emergent.StatusPending},
	emergent.StatusCompleted:  {}, // terminal
}

type taskValidator struct{}

// NewTaskValidator creates a validator for Task entities
func NewTaskValidator() Validator {
	return &taskValidator{}
}

func (v *taskValidator) Validate(from, to string, ctx *TransitionContext, taskID string) error {
	// Check if transition is allowed
	if !isAllowedTransition(from, to, taskTransitions) {
		return transitionError(from, to)
	}

	// Apply guards based on target state
	switch to {
	case emergent.StatusCompleted:
		return v.guardCompleted(ctx, taskID)
	}

	return nil
}

// guardCompleted ensures all subtasks are completed before completing parent
func (v *taskValidator) guardCompleted(ctx *TransitionContext, taskID string) error {
	if ctx.Force {
		return nil
	}

	// All subtasks must be completed
	subtasks, err := ctx.Client.GetRelatedObjects(ctx.Ctx, taskID, emergent.RelHasSubtask)
	if err != nil {
		return fmt.Errorf("failed to get subtasks: %w", err)
	}

	for _, subtask := range subtasks {
		status, ok := subtask.Properties["status"].(string)
		if !ok || status != emergent.StatusCompleted {
			return fmt.Errorf("cannot complete task: subtask %s is %s", subtask.ID, status)
		}
	}

	return nil
}
