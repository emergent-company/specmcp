// Command specmcp runs the SpecMCP MCP server.
//
// It supports two transport modes selected via SPECMCP_TRANSPORT:
//
//   - "stdio" (default): Communicates over stdin/stdout using JSON-RPC 2.0.
//     Used when launched as a subprocess by an MCP client. Requires EMERGENT_TOKEN.
//
//   - "http": Runs as a standalone HTTP server implementing the MCP Streamable
//     HTTP transport (spec 2025-03-26). Clients send their Emergent project token
//     as the Bearer token in each request. No server-side auth config is needed.
//
// Required environment variables:
//
//	EMERGENT_TOKEN        - Project-scoped token (emt_*) for Emergent API (stdio mode only)
//
// Optional environment variables:
//
//	EMERGENT_URL          - Emergent server URL (default: http://localhost:3002)
//	SPECMCP_TRANSPORT     - Transport mode: "stdio" or "http" (default: stdio)
//	SPECMCP_PORT          - HTTP listen port (default: 21452, http mode only)
//	SPECMCP_HOST          - HTTP listen address (default: 0.0.0.0, http mode only)
//	SPECMCP_CORS_ORIGINS  - Comma-separated CORS origins (default: *, http mode only)
//	SPECMCP_LOG_LEVEL     - Log level: debug, info, warn, error (default: info)
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/emergent-company/specmcp/internal/config"
	"github.com/emergent-company/specmcp/internal/content"
	"github.com/emergent-company/specmcp/internal/emergent"
	"github.com/emergent-company/specmcp/internal/mcp"
	"github.com/emergent-company/specmcp/internal/scheduler"
	"github.com/emergent-company/specmcp/internal/tools/constitution"
	"github.com/emergent-company/specmcp/internal/tools/janitor"
	"github.com/emergent-company/specmcp/internal/tools/patterns"
	"github.com/emergent-company/specmcp/internal/tools/query"
	gosync "github.com/emergent-company/specmcp/internal/tools/sync"
	"github.com/emergent-company/specmcp/internal/tools/tasks"
	"github.com/emergent-company/specmcp/internal/tools/workflow"
)

// Version is set via ldflags at build time.
var Version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "specmcp: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Handle subcommands before flag parsing.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "info":
			runInfo(os.Args[2:])
			return nil
		case "version":
			runVersion()
			return nil
		case "upgrade":
			handleUpgradeCommand(os.Args[2:])
			return nil
		}
	}

	// Parse flags
	configPath := flag.String("config", "", "path to specmcp.toml config file")
	flag.Parse()

	// Load configuration (file + env vars)
	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Set up structured logging to stderr (stdout is reserved for MCP protocol in stdio mode)
	logLevel := parseLogLevel(cfg.Log.Level)
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	}))

	version := cfg.Server.Version
	if Version != "dev" {
		version = Version
	}

	logger.Info("starting specmcp",
		"version", version,
		"transport", cfg.Transport.Mode,
		"emergent_url", cfg.Emergent.URL,
	)

	// Set up signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Create tool registry and register tools
	registry := mcp.NewRegistry()

	// Create Emergent client factory for per-request clients.
	// In stdio mode, each request reuses the configured token.
	// In HTTP mode, each request's token comes from the Authorization header
	// (injected into context by the HTTP transport layer).
	emFactory := emergent.NewClientFactory(cfg.Emergent.URL, logger)

	// Register workflow tools
	specArtifact := workflow.NewSpecArtifact(emFactory)
	registry.Register(workflow.NewSpecNew(emFactory))
	registry.Register(specArtifact)
	registry.Register(workflow.NewSpecBatchArtifact(specArtifact))
	registry.Register(workflow.NewSpecArchive(emFactory))
	registry.Register(workflow.NewSpecVerify(emFactory))
	registry.Register(workflow.NewSpecMarkReady(emFactory))
	registry.Register(workflow.NewSpecStatus(emFactory))

	// Register query tools
	registry.Register(query.NewListChanges(emFactory))
	registry.Register(query.NewGetChange(emFactory))
	registry.Register(query.NewGetContext(emFactory))
	registry.Register(query.NewGetComponent(emFactory))
	registry.Register(query.NewGetAction(emFactory))
	registry.Register(query.NewGetDataModel(emFactory))
	registry.Register(query.NewGetApp(emFactory))
	registry.Register(query.NewGetScenario(emFactory))
	registry.Register(query.NewGetPatterns(emFactory))
	registry.Register(query.NewImpactAnalysis(emFactory))
	registry.Register(query.NewSearch(emFactory))

	// Register task management tools
	registry.Register(tasks.NewGenerateTasks(emFactory))
	registry.Register(tasks.NewGetAvailableTasks(emFactory))
	registry.Register(tasks.NewAssignTask(emFactory))
	registry.Register(tasks.NewCompleteTask(emFactory))
	registry.Register(tasks.NewGetCriticalPath(emFactory))

	// Register pattern tools
	registry.Register(patterns.NewSuggestPatterns(emFactory))
	registry.Register(patterns.NewApplyPattern(emFactory))
	registry.Register(patterns.NewSeedPatterns(emFactory))

	// Register constitution tools
	registry.Register(constitution.NewCreateConstitution(emFactory))
	registry.Register(constitution.NewValidateConstitution(emFactory))

	// Register sync tools
	registry.Register(gosync.NewSyncStatus(emFactory))
	registry.Register(gosync.NewSync(emFactory))
	registry.Register(gosync.NewGraphSummary(emFactory))

	// Register janitor tool
	registry.Register(janitor.NewJanitorRun(emFactory, logger))

	// Register prompts (actionable - gather info or kick off workflows)
	registry.RegisterPrompt(&content.CreateConstitutionPrompt{})
	registry.RegisterPrompt(&content.StartChangePrompt{})
	registry.RegisterPrompt(&content.SetupAppPrompt{})

	// Register resources (reference material)
	registry.RegisterResource(&content.GuideResource{})
	registry.RegisterResource(&content.WorkflowResource{})
	registry.RegisterResource(&content.EntityModelResource{})
	registry.RegisterResource(&content.GuardrailsResource{})
	registry.RegisterResource(&content.ToolReferenceResource{})

	// Create core MCP server (transport-agnostic)
	server := mcp.NewServer(registry, mcp.ServerInfo{
		Name:    cfg.Server.Name,
		Version: version,
	}, logger)

	// Start scheduler if enabled (for stdio mode only - HTTP mode doesn't support background jobs)
	var sched *scheduler.Scheduler
	if cfg.Janitor.Enabled && cfg.Transport.Mode == "stdio" {
		sched = scheduler.NewScheduler(logger)
		janitorJob := janitor.NewJanitorJob(emFactory, logger, cfg.Emergent.Token)
		interval := time.Duration(cfg.Janitor.IntervalHours) * time.Hour
		sched.AddJob(janitorJob, interval)
		sched.Start(ctx)
		defer sched.Stop()
		logger.Info("janitor scheduler enabled", "interval_hours", cfg.Janitor.IntervalHours)
	}

	// Select transport
	switch cfg.Transport.Mode {
	case "http":
		return runHTTP(ctx, server, cfg, logger)
	default:
		// Stdio mode: inject the configured token into the context so
		// ClientFactory.ClientFor can create per-request clients.
		ctx = emergent.WithToken(ctx, cfg.Emergent.Token)
		return server.Run(ctx)
	}
}

// runHTTP starts the Streamable HTTP transport server.
func runHTTP(ctx context.Context, server *mcp.Server, cfg *config.Config, logger *slog.Logger) error {
	httpServer := mcp.NewHTTPServer(
		server,
		cfg.Transport.CORSOrigins,
		logger,
	)

	addr := net.JoinHostPort(cfg.Transport.Host, cfg.Transport.Port)

	srv := &http.Server{
		Addr:              addr,
		Handler:           httpServer.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB
	}

	// Start HTTP server in a goroutine.
	errCh := make(chan error, 1)
	go func() {
		logger.Info("HTTP server listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("HTTP server error: %w", err)
		}
		close(errCh)
	}()

	// Wait for shutdown signal or server error.
	select {
	case <-ctx.Done():
		logger.Info("shutting down HTTP server")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("HTTP server shutdown: %w", err)
		}
		logger.Info("HTTP server stopped")
		return nil
	case err := <-errCh:
		return err
	}
}

func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
