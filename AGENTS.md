# SpecMCP

Spec-driven development MCP server backed by the Emergent knowledge graph. Written in Go 1.25+.

## What This Project Does

SpecMCP is an MCP (Model Context Protocol) server that enables AI coding assistants to work with structured specifications. It supports two transport modes:

- **stdio** (default): Communicates over stdin/stdout using JSON-RPC 2.0. Used when launched as a subprocess by an MCP client.
- **http**: Runs as a standalone HTTP server implementing the MCP Streamable HTTP transport (spec 2025-03-26). Suitable for deployment on Linux servers and consumption by remote agents.

All persistence is delegated to the Emergent Graph API (REST). There is no local database.

The core workflow enforces artifact progression: **Proposal (Why) -> Specs (What) -> Design (How) -> Tasks (Steps)**. Each stage requires the previous stage's artifacts to exist AND be marked "ready" before proceeding.

## Project Structure

```
cmd/specmcp/main.go          # Entry point - wires tools, prompts, resources, selects transport
internal/
  config/config.go            # Env var configuration loading (including transport config)
  mcp/
    server.go                 # Core MCP server (transport-agnostic JSON-RPC dispatch)
    http.go                   # Streamable HTTP transport (MCP spec 2025-03-26)
    types.go                  # JSON-RPC and MCP protocol types
    registry.go               # Tool/Prompt/Resource registry with interfaces
  emergent/
    client.go                 # SDK wrapper with CRUD, relationship, search operations
    types.go                  # 18 entity types, 30+ relationship constants, Go structs
    entities.go               # Typed CRUD operations (CreateChange, CreateProposal, etc.)
    idmap.go                  # NodeIndex, ObjectIndex, IDSet for ID disambiguation
  guards/
    guards.go                 # Guard interface, Runner, Severity levels (HARD/SOFT_BLOCK, WARNING, SUGGESTION)
    checks.go                 # All guard implementations and guard sets
    populate.go               # PopulateProjectState/PopulateChangeState via ExpandGraph
  tools/
    workflow/                 # 7 tools: spec_new, spec_artifact, spec_batch_artifact, spec_archive, spec_verify, spec_mark_ready, spec_status
    query/                    # 11 query tools + search tool
    tasks/                    # 5 task management tools
    patterns/                 # suggest_patterns, apply_pattern, seed_patterns
    constitution/             # create_constitution, validate_constitution
    sync/                     # sync_status, sync, graph_summary
    janitor/                  # janitor_run - compliance verification and maintenance
  scheduler/                  # Background job scheduler for periodic tasks
  content/
    prompts.go                # GuidePrompt, WorkflowPrompt
    resources.go              # EntityModel, Guardrails, ToolReference resources
templates/specmcp-pack.json   # Emergent template pack (entity/relationship schemas)
scripts/seed.go               # Script to register template pack with Emergent
Dockerfile                    # Multi-stage build for Linux deployment
docker-compose.yml            # Example compose config for standalone deployment
specmcp.example.toml          # Example TOML config file
docs/                         # Design specs and implementation plans
```

## Build & Run

```bash
task build       # -> dist/specmcp binary
task test        # go test ./...
task fmt         # go fmt ./...
task vet         # go vet ./...
task tidy        # go mod tidy
task seed        # Register template pack with Emergent
task docker      # Build Docker image
task clean       # Remove dist/
```

### Running as Stdio (default)

```bash
# With env var:
EMERGENT_TOKEN=emt_... ./dist/specmcp

# Or with config file:
cp specmcp.example.toml specmcp.toml  # edit token
./dist/specmcp

# Or with explicit config path:
./dist/specmcp --config /path/to/specmcp.toml
```

### Running as Standalone HTTP Server

```bash
# Only the Emergent URL is needed server-side.
# Clients send their own Emergent token as the Bearer header.
EMERGENT_URL=http://your-emergent:3002 \
SPECMCP_TRANSPORT=http \
./dist/specmcp

# Or with config file (set transport.mode = "http" etc. in specmcp.toml):
./dist/specmcp
```

### Running with Docker

```bash
docker build -t specmcp .
docker run -p 21452:21452 \
  -e EMERGENT_URL=http://your-emergent:3002 \
  specmcp
```

Or with docker-compose:

```bash
cp .env.example .env  # Fill in your secrets
docker compose up
```

## Configuration

SpecMCP uses layered configuration with precedence: **environment variables > config file > defaults**.

Config file search order (first found wins):
1. `--config` flag
2. `SPECMCP_CONFIG` env var
3. `./specmcp.toml` (current directory)
4. `~/.config/specmcp/specmcp.toml`

The config file is TOML format. See `specmcp.example.toml` for all options. The config file is optional — env vars alone are sufficient.

## Environment Variables

Environment variables always override config file values.

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `EMERGENT_TOKEN` | stdio only | - | Project-scoped token (`emt_*`) or API key. Required for stdio mode; not needed for HTTP mode (clients send their own token). |
| `EMERGENT_ADMIN_TOKEN` | http mode (for janitor) | - | Admin token for server-side operations in HTTP mode (janitor, health checks). Required when `SPECMCP_JANITOR_ENABLED=true` in HTTP mode. |
| `EMERGENT_URL` | No | `http://localhost:3002` | Emergent server URL |
| `EMERGENT_PROJECT_ID` | No | - | Required when using standalone API keys |
| `SPECMCP_TRANSPORT` | No | `stdio` | Transport mode: `stdio` or `http` |
| `SPECMCP_PORT` | No | `21452` | HTTP listen port (http mode only) |
| `SPECMCP_HOST` | No | `0.0.0.0` | HTTP listen address (http mode only) |
| `SPECMCP_CORS_ORIGINS` | No | `*` | Comma-separated CORS origins (http mode only) |
| `SPECMCP_LOG_LEVEL` | No | `info` | `debug`, `info`, `warn`, `error` |
| `SPECMCP_JANITOR_ENABLED` | No | `false` | Enable scheduled janitor runs (stdio mode only) |
| `SPECMCP_JANITOR_INTERVAL_HOURS` | No | `1` | How often to run janitor (in hours) |
| `SPECMCP_JANITOR_CREATE_PROPOSAL` | No | `false` | Auto-create maintenance proposals for critical issues |

## Janitor Agent

The janitor agent maintains project health by verifying artifact compliance and identifying issues. It can be run on-demand via the `spec_janitor_run` tool or scheduled to run automatically at regular intervals.

### Features

- **Artifact Verification**: Checks naming conventions (kebab-case), readiness cascades, and completeness
- **Relationship Validation**: Ensures specs have requirements, requirements have scenarios, etc.
- **Orphan Detection**: Identifies artifacts not connected to any Change
- **Stale Change Detection**: Flags changes in draft/proposed state for over 30 days
- **Automatic Proposals**: Can create maintenance proposals for critical issues

### Running On-Demand

Use the `spec_janitor_run` tool via any MCP client:

```json
{
  "scope": "all",              // "all", "changes", "artifacts", "relationships"
  "create_proposal": true,     // Create a proposal if critical issues found
  "auto_fix": false            // Automatically fix minor issues (future)
}
```

**HTTP Mode Note**: When running in HTTP mode, the janitor requires server-side authentication. You must configure an admin token:

```bash
# Environment variable
EMERGENT_ADMIN_TOKEN=emt_...

# Or in specmcp.toml
[emergent]
admin_token = "emt_..."
```

Without an admin token, the janitor will fail with authentication errors when called via MCP in HTTP mode.

### Scheduled Runs

Enable automatic hourly runs in stdio mode by setting configuration:

**Environment variables:**
```bash
SPECMCP_JANITOR_ENABLED=true
SPECMCP_JANITOR_INTERVAL_HOURS=1
SPECMCP_JANITOR_CREATE_PROPOSAL=false
```

**Or in `specmcp.toml`:**
```toml
[janitor]
enabled = true
interval_hours = 1
create_proposal = false
```

Scheduled runs check everything but don't auto-create proposals by default (only log findings). This prevents noise while still providing visibility into project health.

**Note**: Scheduled janitor runs work best in HTTP mode. HTTP mode runs as a persistent daemon process, making it ideal for background tasks. stdio mode is tied to client sessions and terminates when clients disconnect, making it unsuitable for long-running schedules. The `interval_hours` config supports fractional values (e.g., `0.5` for 30 minutes).

### Logging and Monitoring

The janitor logs structured summaries of its findings to the server logs (systemd journal, stderr), making it easy to monitor project health without calling the MCP tool.

**Log entries include:**

1. **Overall summary** - Total issues, critical count, warnings
2. **Breakdown by type** - Naming violations, orphaned entities, missing relationships, etc.
3. **Critical issue details** - Individual WARN-level logs for each critical issue

**Example logs:**

```json
{"msg":"janitor run complete","total_issues":51,"critical":0,"warnings":51,"entity_count":6}
{"msg":"janitor findings by type","naming_convention":10,"orphaned_entity":40,"missing_relationship":1}
```

**Monitoring commands:**

```bash
# View recent janitor runs
sudo journalctl -u specmcp -n 100 | grep janitor

# Monitor for critical issues
sudo journalctl -u specmcp -f | grep "critical issues detected"

# Track issue trends over time
journalctl -u specmcp --since "1 week ago" | grep "janitor run complete"
```

See [docs/JANITOR_LOGGING.md](docs/JANITOR_LOGGING.md) for detailed logging documentation.


## HTTP Transport Details

The HTTP mode implements the MCP Streamable HTTP transport (spec 2025-03-26):

- **Endpoint**: `POST /mcp` for JSON-RPC messages, `DELETE /mcp` for session termination
- **Health check**: `GET /health` returns `{"status":"ok"}`
- **Authentication**: Clients send `Authorization: Bearer <emergent_token>` — the token is the client's own Emergent project token (`emt_*`). SpecMCP passes it through to Emergent. No server-side API key is needed.
- **Sessions**: The server assigns an `Mcp-Session-Id` on `initialize` and tracks sessions with activity timestamps
- **CORS**: Configurable via `SPECMCP_CORS_ORIGINS`
- **Request limit**: 10MB max request body
- **Timeouts**: 
  - Request timeout: 5 minutes (configurable via `SPECMCP_REQUEST_TIMEOUT_MINUTES`)
  - Idle timeout: 5 minutes (configurable via `SPECMCP_IDLE_TIMEOUT_MINUTES`)
  - Connection keep-alive: 30 seconds
  - Connection pool: 100 max idle connections, 50 max per host

Clients send JSON-RPC messages via HTTP POST and receive JSON responses. The server supports both single messages and JSON-RPC batches.

### Connection Management

To prevent timeouts and keep connections alive:

1. **HTTP Client** (internal/emergent/client.go):
   - Uses connection pooling with keep-alive
   - 5-minute timeout for long operations (sync, large queries)
   - Retains connections for 90 seconds in idle pool
   - Maximum 50 connections per host

2. **HTTP Server** (cmd/specmcp/main.go):
   - 5-minute read/write timeouts (configurable)
   - 5-minute idle timeout to keep sessions alive (configurable)
   - 30-second connection establishment timeout

3. **Configuration**:
   ```bash
   # Increase timeouts for very long operations
   SPECMCP_REQUEST_TIMEOUT_MINUTES=10
   SPECMCP_IDLE_TIMEOUT_MINUTES=10
   ```

## Key Dependency

The only significant external dependency is the Emergent SDK:
`github.com/emergent-company/emergent/apps/server-go/pkg/sdk v0.9.4`

No HTTP framework is used — the HTTP transport is built on Go's `net/http` stdlib.

## Architecture & Patterns

### Transport Architecture
The server is split into transport-agnostic core (`server.go`) and transport layers:
- `Server.HandleMessage(ctx, []byte) *Response` — parses and dispatches any JSON-RPC message, independent of transport
- `Server.Run(ctx)` — stdio transport (reads from stdin, writes to stdout)
- `HTTPServer` — wraps `Server` with Streamable HTTP transport, authentication, CORS, and session management

### Tool Interface Pattern
All tools implement the `mcp.Tool` interface defined in `internal/mcp/registry.go`:
```go
type Tool interface {
    Name() string
    Description() string
    InputSchema() json.RawMessage
    Execute(ctx context.Context, params json.RawMessage) (*ToolsCallResult, error)
}
```
Tools are registered in a thread-safe `Registry` with ordered registration. Each tool struct holds a `*emergent.ClientFactory` and optionally a `*guards.Runner`. In `Execute()`, tools obtain a per-request `*emergent.Client` via `t.factory.ClientFor(ctx)`, where the context carries the caller's Emergent token.

### Domain Client Wrapper
`internal/emergent/client.go` wraps the Emergent SDK. The `ClientFactory` holds a shared `*http.Client` (for TCP connection reuse) and the Emergent server URL. It creates lightweight per-request `*Client` instances via `ClientFor(ctx)`, extracting the caller's token from context (`emergent.WithToken`/`emergent.TokenFrom`). `entities.go` provides typed CRUD using JSON round-trip for property conversion (generic `toProps`/`fromProps` helpers). Always use the domain client for graph operations instead of the raw SDK.

### ID Disambiguation
Emergent uses both version-specific IDs and canonical IDs. The codebase uses `NodeIndex`, `ObjectIndex`, `IDSet`, and `CanonicalizeEdgeIDs()` in `internal/emergent/idmap.go`. Be aware of this dual-indexing when working with entity references.

### Guardrail System
Guards are composable checks in `internal/guards/`. Three severity levels can block operations:
- `HARD_BLOCK` - Cannot proceed
- `SOFT_BLOCK` - Blocked unless `force=true`
- `WARNING` / `SUGGESTION` - Advisory only

Guard context is populated via single `ExpandGraph` calls (not N+1) in `populate.go` and shared across all guards.

### Entity Model
18 entity types and 30+ relationship types are defined in `internal/emergent/types.go`. The template pack in `templates/specmcp-pack.json` must stay in sync with these type constants.

### Workflow Readiness Cascade
Workflow artifacts (Proposal, Spec, Requirement, Scenario, Design) use `draft`/`ready` status. Readiness cascades: a Spec cannot be ready unless all its Requirements are ready, which need all Scenarios ready, etc.

### Deduplication / Upsert
Entity creation uses upsert-style logic - checks for existing type+key before creating. Change tracking relationships (`change_creates`, `change_modifies`, `change_references`) record what state a Change was designed against.

### Batch Over Individual
Prefer `ExpandGraph` and `GetObjects` (batch) over individual lookups. Fallback paths exist for error cases.

## Code Conventions

- All Go code uses `log/slog` structured logging (JSON handler to stderr)
- Use `kebab-case` for entity keys/names (e.g. `add-user-permissions`)
- Tool input schemas are inline JSON in `InputSchema()` methods, not generated
- Error wrapping follows `fmt.Errorf("doing X: %w", err)` pattern
- No test files currently exist. When adding tests, follow standard Go `_test.go` conventions
- MCP protocol version is `2024-11-05`
- stdout is reserved for MCP protocol in stdio mode; never log to stdout
- HTTP transport uses Go stdlib `net/http` only — no external HTTP framework

## Common Tasks

### Adding a New Tool
1. Create a new file in the appropriate `internal/tools/<category>/` directory
2. Define a struct that holds `*emergent.ClientFactory` (and `*guards.Runner` if guards are needed)
3. Implement the `mcp.Tool` interface: `Name()`, `Description()`, `InputSchema()`, `Execute()`
4. In `Execute()`, obtain a per-request client via `t.factory.ClientFor(ctx)`
5. Register in `cmd/specmcp/main.go` with `registry.Register()`

### Adding a New Entity Type
1. Add the type constant to `internal/emergent/types.go`
2. Add a Go struct with JSON tags
3. Add typed CRUD functions to `internal/emergent/entities.go`
4. Add the schema to `templates/specmcp-pack.json`
5. Run `task seed` to register with Emergent

### Adding a New Guard
1. Add a `GuardFunc` or implement the `Guard` interface in `internal/guards/checks.go`
2. Add it to the appropriate guard set (`PreChangeGuards`, `ArtifactGuards`, or `ArchiveGuards`)
3. If the guard needs new state, extend `GuardContext` and update `populate.go`

### Deploying to Linux
1. Build Docker image: `task docker` or `docker build -t specmcp .`
2. Set required env vars: `EMERGENT_URL`
3. Run: `docker run -p 21452:21452 -e EMERGENT_URL=http://your-emergent:3002 specmcp`
4. Health check: `curl http://localhost:21452/health`
5. MCP endpoint: `POST http://localhost:21452/mcp` with `Authorization: Bearer <emergent_token>`
