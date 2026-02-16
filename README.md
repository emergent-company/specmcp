# SpecMCP

Spec-driven development MCP server backed by the [Emergent](https://emergent.sh) knowledge graph.

SpecMCP enables AI coding assistants to work with structured specifications, track implementation progress, and coordinate parallel development work through the [Model Context Protocol](https://modelcontextprotocol.io/).

## Overview

SpecMCP provides a spec-anchored development workflow where specifications evolve continuously alongside code. Unlike file-based approaches, SpecMCP uses Emergent's knowledge graph for:

- **Relationship tracking** - Entities connected via typed relationships
- **Impact analysis** - Query what's affected by changes
- **Versioning** - Built-in entity versioning and branching
- **Parallel execution** - Multiple agents working on related tasks

## Prerequisites

- Go 1.25+
- An [Emergent](https://emergent.sh) project with an API token

## Installation

### From source

```bash
git clone https://github.com/emergent-company/specmcp.git
cd specmcp
task build
```

The binary will be output to `dist/specmcp`.

### Go install

```bash
go install github.com/emergent-company/specmcp/cmd/specmcp@latest
```

## Configuration

SpecMCP uses layered configuration: **environment variables > config file > defaults**.

### Config file

Copy the example and edit:

```bash
cp specmcp.example.toml specmcp.toml
```

Config file search order (first found wins):
1. `--config` flag (e.g. `specmcp --config /path/to/specmcp.toml`)
2. `SPECMCP_CONFIG` env var
3. `./specmcp.toml` (current directory)
4. `~/.config/specmcp/specmcp.toml`

The config file is TOML format and entirely optional — env vars alone are sufficient.

### Environment variables

Environment variables always override config file values.

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `EMERGENT_TOKEN` | stdio only | - | Project-scoped token (`emt_*`) or API key. Required for stdio mode; not needed for HTTP mode (clients send their own token). |
| `EMERGENT_URL` | No | `http://localhost:3002` | Emergent server URL |
| `EMERGENT_PROJECT_ID` | No | - | Required when using standalone API keys |
| `SPECMCP_TRANSPORT` | No | `stdio` | Transport mode: `stdio` or `http` |
| `SPECMCP_PORT` | No | `21452` | HTTP listen port (http mode only) |
| `SPECMCP_HOST` | No | `0.0.0.0` | HTTP listen address (http mode only) |
| `SPECMCP_CORS_ORIGINS` | No | `*` | Comma-separated CORS origins (http mode only) |
| `SPECMCP_LOG_LEVEL` | No | `info` | `debug`, `info`, `warn`, `error` |

## Usage

### Stdio mode (default)

SpecMCP runs as a stdio MCP server. Add it to your MCP client configuration:

```json
{
  "mcpServers": {
    "specmcp": {
      "command": "specmcp",
      "env": {
        "EMERGENT_TOKEN": "emt_your_token_here",
        "EMERGENT_URL": "https://your-emergent-instance.com"
      }
    }
  }
}
```

Or run directly:

```bash
EMERGENT_TOKEN=emt_... ./dist/specmcp
```

### HTTP mode

Run as a standalone HTTP server. Clients send their own Emergent token as the Bearer header — no server-side token is needed:

```bash
EMERGENT_URL=http://your-emergent:3002 \
SPECMCP_TRANSPORT=http \
./dist/specmcp
```

Clients connect via `POST /mcp` with `Authorization: Bearer <emergent_token>`. The token is the client's own Emergent project token (`emt_*`).

Health check: `GET /health`

### Docker

```bash
docker build -t specmcp .
docker run -p 21452:21452 -e EMERGENT_URL=http://your-emergent:3002 specmcp
```

## Capabilities

### Tools (31)

- **Workflow** (7): `spec_new`, `spec_artifact`, `spec_batch_artifact`, `spec_archive`, `spec_verify`, `spec_mark_ready`, `spec_status`
- **Query** (11): `list_changes`, `get_change`, `get_context`, `get_component`, `get_action`, `get_data_model`, `get_service`, `get_scenario`, `get_patterns`, `impact_analysis`, `search`
- **Tasks** (5): `generate_tasks`, `get_available_tasks`, `assign_task`, `complete_task`, `get_critical_path`
- **Patterns** (3): `suggest_patterns`, `apply_pattern`, `seed_patterns`
- **Constitution** (2): `create_constitution`, `validate_constitution`
- **Sync** (3): `sync_status`, `sync`, `graph_summary`

### Prompts (2)

- `specmcp-guide` - Comprehensive usage guide
- `specmcp-workflow` - Step-by-step workflow guide

### Resources (3)

- `specmcp://entity-model` - Entity type and relationship reference
- `specmcp://guardrails` - Guardrail system documentation
- `specmcp://tool-reference` - Tool usage reference

## Seeding Templates

To register the SpecMCP template pack with your Emergent project:

```bash
EMERGENT_TOKEN=emt_... task seed
```

## Development

```bash
task build    # Build binary
task test     # Run tests
task fmt      # Format code
task vet      # Run go vet
task tidy     # Tidy dependencies
task clean    # Remove build artifacts
```

## License

MIT
