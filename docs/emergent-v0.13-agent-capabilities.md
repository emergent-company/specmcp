# Emergent v0.13.0 - New Agent Capabilities Research

## Version Information
- **CLI Version**: 0.13.0 (commit: d5b916a, built: 2026-02-16)
- **SDK Version**: v0.13.0 available
- **Current Project SDK**: v0.11.0 (needs upgrade)

## 🎯 Key New Features

### 1. Agent Definitions

A new **agent-definitions** system that separates agent configuration from runtime agents:

#### Available Configuration Options:

```bash
emergent-cli agent-definitions create \
  --name "janitor-definition" \
  --system-prompt "You are the Janitor Agent..." \
  --max-steps 20 \                    # ✅ MAXIMUM STEPS PER RUN!
  --default-timeout 900 \             # ✅ TIMEOUT IN SECONDS (15 min)
  --model "gemini-2.0-flash" \        # ✅ MODEL SELECTION
  --tools "tool1,tool2,tool3" \       # ✅ TOOL ALLOWLIST
  --flow-type "single" \              # single, multi, coordinator
  --visibility "project"              # external, project, internal
```

**This gives us exactly what we need:**
- ✅ **`--max-steps`**: Hard limit on tool calls per run
- ✅ **`--default-timeout`**: Time budget in seconds  
- ✅ **`--model`**: Choose appropriate model (fast vs smart)
- ✅ **`--tools`**: Restrict which tools agent can use

### 2. MCP Server Management

New commands for managing MCP servers at the project level:

```bash
emergent-cli mcp-servers create --name "specmcp" --url "http://localhost:21452/mcp"
emergent-cli mcp-servers list
emergent-cli mcp-servers sync     # Sync tools from MCP server
```

**Note**: Currently showing "TO BE IMPLEMENTED" but API exists at `/api/admin/mcp-servers`

### 3. Enhanced Agent Updates

Runtime agents can now be updated with:
- cron schedules
- execution modes
- trigger types
- descriptions
- prompts

## Architecture: Agent Definitions vs Runtime Agents

### Definitions (Template)
- **Purpose**: Define agent capabilities, limits, and behavior
- **Reusable**: One definition → many runtime instances
- **Configuration**: max_steps, timeout, model, tools, prompt
- **Lifecycle**: Create once, update as needed

### Runtime Agents (Instance)
- **Purpose**: Scheduled or triggered executions
- **References**: Links to an agent definition
- **Runtime Config**: cron schedule, enabled/disabled, execution mode
- **Lifecycle**: Created, triggered, runs, completes

## Recommended Janitor Configuration

### Step 1: Create Agent Definition

```bash
emergent-cli agent-definitions create \
  --name "janitor-agent-definition" \
  --description "Maintenance automation for SpecMCP knowledge graph" \
  --system-prompt "$(cat /root/specmcp/docs/janitor-agent-prompt.txt)" \
  --max-steps 20 \
  --default-timeout 900 \
  --model "gemini-2.0-flash-thinking" \
  --flow-type "single" \
  --visibility "project"
```

**Limits Explanation:**
- `max-steps 20`: Maximum 20 tool calls per run
  - Phase 1 (fix): ~12 calls (3 issues × 4 calls each)
  - Phase 2 (detect): ~8 calls (scan + create issues)
- `default-timeout 900`: 15 minutes maximum runtime
  - Provides buffer beyond expected 11 minutes
- `model gemini-2.0-flash-thinking`: Fast + reasoning capability
- `flow-type single`: Single-turn execution (not multi-agent)

### Step 2: Update Runtime Agent (Link to Definition)

**Question**: How do we link a runtime agent to a definition?

Options to investigate:
1. Delete existing agent, create new one with `--definition-id`
2. Update existing agent with a new flag
3. Agent definitions work independently from runtime agents

**Need to check:**
```bash
emergent-cli agents create --help | grep -i def
emergent-cli agents update --help | grep -i def
```

## Token Usage & Cost Control

### SDK v0.13.0 Likely Additions

Based on the pattern, v0.13.0 SDK probably includes:

```go
// In agents package
type AgentDefinition struct {
    ID             string
    Name           string
    SystemPrompt   string
    MaxSteps       int       // NEW!
    DefaultTimeout int       // NEW! (seconds)
    Model          string    // NEW!
    Tools          []string  // NEW!
    FlowType       string    // NEW!
    Visibility     string
}

type AgentRun struct {
    ID            string
    Status        string
    Steps         int       // NEW! (current step count)
    MaxSteps      int       // NEW! (limit)
    TokenUsage    *TokenUsage // NEW!
    StartedAt     time.Time
    CompletedAt   *time.Time
    Duration      int
}

type TokenUsage struct {
    InputTokens   int
    OutputTokens  int
    TotalTokens   int
    EstimatedCost float64
}
```

### Getting Run Metrics

After upgrading SDK, we should be able to:

```go
run, err := client.Agents.GetRun(ctx, agentID, runID)
if err != nil {
    return err
}

fmt.Printf("Steps: %d/%d\n", run.Steps, run.MaxSteps)
fmt.Printf("Tokens: %d (cost: $%.4f)\n", 
    run.TokenUsage.TotalTokens, 
    run.TokenUsage.EstimatedCost)
fmt.Printf("Duration: %v\n", run.Duration)
```

## Immediate Action Items

### 1. Upgrade SDK (Priority 1)

```bash
cd /root/specmcp
go get github.com/emergent-company/emergent/apps/server-go/pkg/sdk@v0.13.0
go mod tidy
```

Check for breaking changes in:
- `internal/emergent/client.go`
- Agent-related code if any

### 2. Create Agent Definition (Priority 2)

```bash
# Use the new system to define limits
emergent-cli agent-definitions create \
  --name "janitor-v2" \
  --system-prompt "$(cat docs/janitor-agent-prompt.txt)" \
  --max-steps 20 \
  --default-timeout 900 \
  --model "gemini-2.0-flash-thinking"
```

### 3. Test Limits (Priority 3)

Create a test definition with very low limits:
```bash
emergent-cli agent-definitions create \
  --name "janitor-test" \
  --system-prompt "Test agent with strict limits" \
  --max-steps 3 \
  --default-timeout 60
```

Run it and verify:
- Agent stops at 3 tool calls
- Agent stops at 60 seconds
- Error handling is graceful

### 4. Monitor & Tune (Priority 4)

After production deployment:
- Check run metrics via SDK
- Adjust max-steps if consistently hitting limit
- Adjust timeout if runs are too short/long
- Monitor token costs

## Questions to Investigate

1. **Definition Linking**: How do runtime agents reference definitions?
   - Is there a `--definition-id` flag on `agents create`?
   - Can we update existing agents to use a definition?

2. **Tools Parameter**: Does `--tools` filter available MCP tools?
   - Can we restrict janitor to only maintenance tools?
   - Format: tool names or tool IDs?

3. **Token Budgets**: Is there a `--max-tokens` parameter?
   - Not visible in current help output
   - May be coming in future version

4. **Cost Tracking**: Can we set cost alerts?
   - Per-run cost limits
   - Project-level budgets

5. **Stuck Runs**: What causes runs to stay in "running" state?
   - Network timeouts?
   - MCP server crashes?
   - Lack of graceful timeout handling?

## Next Steps

**Immediate:**
1. ✅ Research complete
2. Upgrade SDK to v0.13.0
3. Test agent-definitions with simple example
4. Verify max-steps enforcement
5. Migrate janitor to use definitions

**Short-term:**
1. Add token usage monitoring
2. Build cost dashboard
3. Fine-tune limits based on metrics
4. Document best practices

**Long-term:**
1. Implement adaptive limits (scale with graph size)
2. Add cost prediction before runs
3. Build runbook for stuck runs
4. Contribute improvements back to Emergent
