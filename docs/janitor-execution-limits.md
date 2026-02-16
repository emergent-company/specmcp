# Janitor Agent Execution Limits Analysis

## Current Situation

### Observed Run Statistics
- **Run ID**: 7d2d0437-3f55-4641-82d6-8523ededddcf
- **Duration**: 652,729ms (~11 minutes)
- **Status**: success
- **Two other runs**: Still showing "running" after 1+ hour (likely stuck/timed out)

### Unknown Details
We cannot currently access:
- Step count (how many tool calls)
- Token usage (input/output tokens)
- Individual step duration
- Failure reason for stuck runs

This suggests Emergent may not expose detailed execution metrics via the current API or CLI.

## Execution Limit Strategy

Since we cannot access Emergent's internal token/step limits, we need to implement **application-level limits** in the janitor workflow itself.

### Proposed Architecture: Single Janitor with Built-in Limits

```
Janitor Agent (runs hourly: 0 * * * *)
├── Phase 1: Execute Approved Maintenance (5 min max)
│   ├── Query MaintenanceIssues (status: approved, limit: 3)
│   ├── For each issue: execute tasks, verify, mark resolved
│   └── Stop after 3 issues OR 5 minutes
│
├── Phase 2: Detect New Issues (5 min max)
│   ├── Run spec_janitor_run with scope: all
│   ├── Create MaintenanceIssue entities (max 5 per run)
│   └── Stop when limit reached
│
└── Phase 3: Report & Exit (1 min max)
    ├── Log summary of work done
    └── Return execution report
```

### Limit Mechanisms

#### 1. **Agent Prompt Limits** (PRIMARY)
Build limits directly into the agent prompt:

```
You are the Janitor Agent for SpecMCP.

CRITICAL LIMITS (DO NOT EXCEED):
- Phase 1: Execute maximum 3 approved MaintenanceIssues
- Phase 2: Create maximum 5 new MaintenanceIssues
- Total tool calls: Maximum 20 per run
- Self-terminate after completing both phases

Your workflow:
1. Check for approved maintenance (spec_list_maintenance_issues status=approved limit=3)
2. Execute each approved issue's tasks
3. After 3 issues OR if none approved, move to Phase 2
4. Run spec_janitor_run to detect new issues
5. Create up to 5 MaintenanceIssue entities
6. Return summary and EXIT

Never exceed these limits. If approaching limits, wrap up current work and exit gracefully.
```

#### 2. **Tool-Level Limits** (SECONDARY)
Enforce in SpecMCP tools themselves:

```go
// In spec_list_maintenance_issues
const MaxIssuesPerQuery = 10

// In spec_janitor_run  
const MaxIssuesCreatedPerRun = 5

// In spec_complete_maintenance_task
const MaxTasksPerRun = 15
```

#### 3. **Timeout Guards** (TERTIARY)
Add timeout metadata to agent config:

```json
{
  "config": {
    "execution_timeout_minutes": 15,
    "max_tool_calls": 20,
    "mcp_servers": [...]
  }
}
```

**Note**: These may not be enforced by Emergent unless the system supports them. Need to verify.

## Recommended Configuration

### Agent Prompt (Updated)

```markdown
You are the Janitor Agent for SpecMCP - a maintenance automation system.

EXECUTION LIMITS (STRICT):
- Total runtime: 15 minutes maximum
- Tool calls: 20 maximum per run
- Phase 1 work: 3 approved issues maximum
- Phase 2 detection: 5 new issues maximum

WORKFLOW:

Phase 1: Execute Approved Maintenance (5 min, 10 tool calls max)
1. Call spec_list_maintenance_issues(status="approved", limit=3)
2. For each approved MaintenanceIssue:
   a. Get tasks via spec_get_maintenance_issue
   b. Execute each task in order
   c. Verify via verification_method
   d. Mark completed
   e. Update issue status to "resolved"
3. Track tool call count - stop at 10 calls

Phase 2: Detect New Issues (5 min, 10 tool calls max)
4. Call spec_janitor_run(scope="all", create_issues=true, max_issues=5)
5. Review created MaintenanceIssues
6. Log summary

Phase 3: Report & Exit
7. Return execution summary
8. EXIT - do not continue beyond limits

Safety Rules:
- Count your tool calls - stop at 20
- If a task verification fails, mark as "blocked", don't retry infinitely
- If approaching time limit, finish current issue and exit
- Never create more than 5 new issues per run
```

### Tool Implementation: Counting Mechanism

Add a new tool `spec_janitor_status` that tracks execution state:

```go
type JanitorStatus struct {
    RunID           string
    StartTime       time.Time
    ToolCallCount   int
    IssuesExecuted  int
    IssuesCreated   int
    Phase           string // "execution", "detection", "complete"
}

// Each tool call increments counter
func (t *Tool) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
    status := getRunStatus(ctx)
    status.ToolCallCount++
    
    if status.ToolCallCount > MaxToolCallsPerRun {
        return mcp.ErrorResult("Tool call limit reached. Exiting gracefully."), nil
    }
    
    // ... actual tool logic
}
```

## Testing Plan

1. **Measure baseline**: Trigger janitor manually, measure:
   - Actual tool call count
   - Time spent per phase
   - Token usage (if accessible via LLM API)

2. **Test limits**: Set very low limits (e.g., max 2 issues) and verify agent respects them

3. **Test stuck prevention**: Simulate failures, verify agent doesn't infinite loop

4. **Monitor production**: After deploying limits, watch for:
   - Runs that hit limits (need increase?)
   - Runs that timeout (need better phase balancing?)

## Implementation Priority

**Phase 1** (Immediate):
1. ✅ Update agent prompt with explicit limits
2. ✅ Add limit parameters to spec_janitor_run tool
3. ✅ Add spec_list_maintenance_issues with limit parameter
4. ✅ Update janitor to create MaintenanceIssue entities

**Phase 2** (Short-term):
1. Implement tool call counter (if possible via Emergent context)
2. Add execution status tracking
3. Build monitoring dashboard

**Phase 3** (Long-term):
1. Request Emergent API for detailed run metrics
2. Implement adaptive limits based on graph size
3. Add cost/token budgeting

## Unknown Factors (Need Investigation)

1. ❓ Does Emergent enforce max_steps/max_tokens server-side?
2. ❓ Can we access token usage post-run?
3. ❓ What causes the "stuck running" state?
4. ❓ Is there a hidden timeout that killed those runs?
5. ❓ Can we configure timeout in agent settings?

**Action**: Need to contact Emergent team or review their documentation for:
- Agent execution model details
- Available limits and how to set them
- Run metrics API
- Debugging stuck runs
