package validation

import (
	"context"
	"errors"
	"fmt"

	"github.com/emergent-company/specmcp/internal/emergent"
)

// Common errors
var (
	ErrInvalidTransition = errors.New("invalid state transition")
	ErrNeedTasks         = errors.New("at least one task required")
	ErrTasksIncomplete   = errors.New("all tasks must be completed")
	ErrChildrenNotReady  = errors.New("child artifacts must be ready first")
	ErrAlreadyInState    = errors.New("already in target state")
)

// TransitionContext holds data needed for validation
type TransitionContext struct {
	Client *emergent.Client
	Ctx    context.Context
	Force  bool // Bypass soft guards if true
}

// Validator is a function that checks if a transition is allowed
type Validator interface {
	Validate(from, to string, ctx *TransitionContext, entityID string) error
}

// ValidatorFunc is a function type that implements Validator
type ValidatorFunc func(from, to string, ctx *TransitionContext, entityID string) error

func (f ValidatorFunc) Validate(from, to string, ctx *TransitionContext, entityID string) error {
	return f(from, to, ctx, entityID)
}

// Registry maps entity types to their validators
type Registry struct {
	validators map[string]Validator
}

// NewRegistry creates a new validator registry
func NewRegistry() *Registry {
	r := &Registry{
		validators: make(map[string]Validator),
	}

	// Register all validators
	r.Register(emergent.TypeImprovement, NewImprovementValidator())
	r.Register(emergent.TypeTask, NewTaskValidator())
	r.Register(emergent.TypeChange, NewChangeValidator())

	// Workflow artifacts share a validator
	artifactValidator := NewWorkflowArtifactValidator()
	r.Register(emergent.TypeProposal, artifactValidator)
	r.Register(emergent.TypeSpec, artifactValidator)
	r.Register(emergent.TypeRequirement, artifactValidator)
	r.Register(emergent.TypeScenario, artifactValidator)
	r.Register(emergent.TypeDesign, artifactValidator)

	return r
}

// Register adds a validator for an entity type
func (r *Registry) Register(entityType string, validator Validator) {
	r.validators[entityType] = validator
}

// Validate checks if a state transition is allowed
func (r *Registry) Validate(entityType, from, to string, ctx *TransitionContext, entityID string) error {
	if from == to {
		return ErrAlreadyInState
	}

	validator, ok := r.validators[entityType]
	if !ok {
		// No validator means no restrictions
		return nil
	}

	return validator.Validate(from, to, ctx, entityID)
}

// Helper to check if transition is in allowed list
func isAllowedTransition(from, to string, transitions map[string][]string) bool {
	allowed, ok := transitions[from]
	if !ok {
		return false
	}

	for _, allowedTo := range allowed {
		if allowedTo == to {
			return true
		}
	}
	return false
}

// Helper to format transition error
func transitionError(from, to string) error {
	return fmt.Errorf("%w: cannot transition from '%s' to '%s'", ErrInvalidTransition, from, to)
}
