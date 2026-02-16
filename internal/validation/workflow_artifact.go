package validation

import (
	"fmt"

	"github.com/emergent-company/specmcp/internal/emergent"
)

// Workflow artifact state transitions (Proposal, Spec, Requirement, Scenario, Design)
var workflowArtifactTransitions = map[string][]string{
	emergent.StatusDraft: {emergent.StatusReady},
	emergent.StatusReady: {emergent.StatusDraft}, // Allow reverting to draft
}

type workflowArtifactValidator struct{}

// NewWorkflowArtifactValidator creates a validator for workflow artifacts
func NewWorkflowArtifactValidator() Validator {
	return &workflowArtifactValidator{}
}

func (v *workflowArtifactValidator) Validate(from, to string, ctx *TransitionContext, artifactID string) error {
	// Check if transition is allowed
	if !isAllowedTransition(from, to, workflowArtifactTransitions) {
		return transitionError(from, to)
	}

	// Apply guards when marking as ready
	if to == emergent.StatusReady {
		return v.guardReady(ctx, artifactID)
	}

	return nil
}

// guardReady ensures child artifacts are ready before parent can be marked ready
func (v *workflowArtifactValidator) guardReady(ctx *TransitionContext, artifactID string) error {
	if ctx.Force {
		return nil
	}

	// Get the artifact to check its type
	artifact, err := ctx.Client.GetObject(ctx.Ctx, artifactID)
	if err != nil {
		return fmt.Errorf("failed to get artifact: %w", err)
	}

	// Check child readiness based on artifact type
	switch artifact.Type {
	case emergent.TypeSpec:
		// All requirements must be ready
		return v.checkChildrenReady(ctx, artifactID, emergent.RelHasRequirement, "requirements")

	case emergent.TypeRequirement:
		// All scenarios must be ready
		return v.checkChildrenReady(ctx, artifactID, emergent.RelHasScenario, "scenarios")

	case emergent.TypeScenario:
		// Scenarios with steps must have all steps defined
		// (ScenarioSteps don't have status, just check they exist if referenced)
		// This is lenient - we allow scenarios without steps
		return nil

	case emergent.TypeProposal, emergent.TypeDesign:
		// These don't have children, always OK
		return nil
	}

	return nil
}

// checkChildrenReady is a helper to verify all children have status=ready
func (v *workflowArtifactValidator) checkChildrenReady(ctx *TransitionContext, parentID, relationship, childType string) error {
	children, err := ctx.Client.GetRelatedObjects(ctx.Ctx, parentID, relationship)
	if err != nil {
		return fmt.Errorf("failed to get %s: %w", childType, err)
	}

	if len(children) == 0 {
		return fmt.Errorf("cannot mark as ready: no %s defined", childType)
	}

	for _, child := range children {
		status, ok := child.Properties["status"].(string)
		if !ok || status != emergent.StatusReady {
			return fmt.Errorf("%w: %s %s is %s", ErrChildrenNotReady, childType, child.ID, status)
		}
	}

	return nil
}
