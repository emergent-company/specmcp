package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// runInfo handles the "specmcp info" subcommand.
// It prints general MCP configuration information and, with flags,
// client-specific configuration snippets.
func runInfo(args []string) {
	fs := flag.NewFlagSet("info", flag.ExitOnError)
	opencode := fs.Bool("opencode", false, "show OpenCode MCP client configuration")
	claude := fs.Bool("claude", false, "show Claude Desktop MCP client configuration")
	cursor := fs.Bool("cursor", false, "show Cursor MCP client configuration")
	fs.Parse(args)

	switch {
	case *opencode:
		printOpenCodeConfig()
	case *claude:
		printClaudeConfig()
	case *cursor:
		printCursorConfig()
	default:
		printGeneralInfo()
	}
}

func printGeneralInfo() {
	fmt.Fprintf(os.Stdout, `SpecMCP %s — Spec-driven development MCP server

SpecMCP is a Model Context Protocol (MCP) server backed by the Emergent
knowledge graph. It enables AI coding assistants to work with structured
specifications through a spec-anchored workflow:

  Proposal (Why) → Specs (What) → Design (How) → Tasks (Steps)

TRANSPORT MODES

  stdio (default)
    Communicates over stdin/stdout using JSON-RPC 2.0. Used when launched
    as a subprocess by an MCP client.

    Requires: EMERGENT_TOKEN (project-scoped token, emt_*)

  http
    Runs as a standalone HTTP server (MCP Streamable HTTP transport,
    spec 2025-03-26). Clients send their own Emergent token as the
    Bearer header in each request.

    Endpoint:      POST /mcp
    Health check:  GET /health
    Default port:  21452

TOOLS (31)

  Workflow (7):     spec_new, spec_artifact, spec_batch_artifact,
                    spec_archive, spec_verify, spec_mark_ready, spec_status
  Query (11):       list_changes, get_change, get_context, get_component,
                    get_action, get_data_model, get_service, get_scenario,
                    get_patterns, impact_analysis, search
  Tasks (5):        generate_tasks, get_available_tasks, assign_task,
                    complete_task, get_critical_path
  Patterns (3):     suggest_patterns, apply_pattern, seed_patterns
  Constitution (2): create_constitution, validate_constitution
  Sync (3):         sync_status, sync, graph_summary

PROMPTS (2)

  specmcp-guide      Comprehensive usage guide (focus: overview/workflow/
                     guardrails/tools/patterns)
  specmcp-workflow   Step-by-step workflow guide for a specific change

RESOURCES (3)

  specmcp://entity-model    Entity type and relationship reference
  specmcp://guardrails      Guardrail system documentation
  specmcp://tool-reference  Tool usage quick reference

GETTING STARTED

  1. Bootstrap (once per project):
     - Seed patterns:       spec_seed_patterns
     - Create constitution: spec_create_constitution

  2. Start a change:        spec_new (name must be kebab-case)

  3. Follow the workflow:   Proposal → mark ready → Specs → Requirements →
                            Scenarios → mark ready (bottom-up) → Design →
                            mark ready → Tasks

  4. Implement:             Work through tasks using task management tools

  5. Finalize:              spec_verify → spec_archive

CLIENT CONFIGURATION

  To see configuration for a specific MCP client, run:

    specmcp info --opencode    OpenCode (.opencode.json)
    specmcp info --claude      Claude Desktop (claude_desktop_config.json)
    specmcp info --cursor      Cursor (.cursor/mcp.json)
`, Version)
}

func printOpenCodeConfig() {
	printStdioConfig("OpenCode", ".opencode.json or opencode.json", `{
  "mcpServers": {
    "specmcp": {
      "command": "specmcp",
      "env": {
        "EMERGENT_TOKEN": "emt_your_token_here",
        "EMERGENT_URL": "https://your-emergent-instance.com"
      }
    }
  }
}`)

	printHTTPConfig("OpenCode", ".opencode.json or opencode.json", `{
  "mcpServers": {
    "specmcp": {
      "type": "streamable-http",
      "url": "http://your-specmcp-server:21452/mcp",
      "headers": {
        "Authorization": "Bearer emt_your_token_here"
      }
    }
  }
}`)
}

func printClaudeConfig() {
	printStdioConfig("Claude Desktop", "claude_desktop_config.json", `{
  "mcpServers": {
    "specmcp": {
      "command": "specmcp",
      "env": {
        "EMERGENT_TOKEN": "emt_your_token_here",
        "EMERGENT_URL": "https://your-emergent-instance.com"
      }
    }
  }
}`)

	printHTTPConfig("Claude Desktop", "claude_desktop_config.json", `{
  "mcpServers": {
    "specmcp": {
      "type": "streamable-http",
      "url": "http://your-specmcp-server:21452/mcp",
      "headers": {
        "Authorization": "Bearer emt_your_token_here"
      }
    }
  }
}`)
}

func printCursorConfig() {
	printStdioConfig("Cursor", ".cursor/mcp.json", `{
  "mcpServers": {
    "specmcp": {
      "command": "specmcp",
      "env": {
        "EMERGENT_TOKEN": "emt_your_token_here",
        "EMERGENT_URL": "https://your-emergent-instance.com"
      }
    }
  }
}`)

	printHTTPConfig("Cursor", ".cursor/mcp.json", `{
  "mcpServers": {
    "specmcp": {
      "type": "streamable-http",
      "url": "http://your-specmcp-server:21452/mcp",
      "headers": {
        "Authorization": "Bearer emt_your_token_here"
      }
    }
  }
}`)
}

func printStdioConfig(client, file, config string) {
	fmt.Fprintf(os.Stdout, `%s — stdio mode
%s

Add to %s:

%s

The EMERGENT_TOKEN is your Emergent project token (emt_*).
SpecMCP runs as a subprocess — no server needed.

`, client, strings.Repeat("─", len(client)+14), file, config)
}

func printHTTPConfig(client, file, config string) {
	fmt.Fprintf(os.Stdout, `%s — HTTP mode (remote server)
%s

Add to %s:

%s

The Authorization header contains your Emergent project token (emt_*).
SpecMCP passes it through to Emergent on your behalf.

`, client, strings.Repeat("─", len(client)+30), file, config)
}
