package janitor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/emergent-company/specmcp/internal/emergent"
)

// JanitorJob wraps the janitor tool as a scheduled job.
type JanitorJob struct {
	factory *emergent.ClientFactory
	logger  *slog.Logger
	token   string // For stdio mode, stores the project token
}

// NewJanitorJob creates a new scheduled janitor job.
func NewJanitorJob(factory *emergent.ClientFactory, logger *slog.Logger, token string) *JanitorJob {
	return &JanitorJob{
		factory: factory,
		logger:  logger,
		token:   token,
	}
}

// Name returns the job name.
func (j *JanitorJob) Name() string {
	return "janitor"
}

// Run executes the janitor verification.
func (j *JanitorJob) Run(ctx context.Context) error {
	// Inject token into context for stdio mode
	if j.token != "" {
		ctx = emergent.WithToken(ctx, j.token)
	}

	j.logger.Info("running scheduled janitor check")

	tool := NewJanitorRun(j.factory, j.logger)

	// Run with default params - check everything, don't auto-create proposals
	params := map[string]any{
		"scope":           "all",
		"create_proposal": false, // Don't auto-create proposals in background runs
		"auto_fix":        false,
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("marshaling params: %w", err)
	}

	result, err := tool.Execute(ctx, paramsJSON)
	if err != nil {
		return fmt.Errorf("executing janitor: %w", err)
	}

	if result.IsError {
		j.logger.Error("janitor check failed", "content", result.Content)
		return fmt.Errorf("janitor check failed")
	}

	j.logger.Info("scheduled janitor check complete")
	return nil
}
