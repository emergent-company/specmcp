# Janitor Logging Improvements

**Version**: 0.6.3  
**Date**: 2026-02-17  
**Status**: Deployed

## Overview

Enhanced the janitor agent's logging to provide better visibility into findings, especially for scheduled runs and monitoring.

## What Changed

### Added Structured Logging

The janitor now logs a summary of its findings to the server logs (systemd journal, stderr) in addition to returning results via MCP.

### Log Entries

1. **Overall Summary**
   ```json
   {
     "msg": "janitor run complete",
     "total_issues": 51,
     "critical": 0,
     "warnings": 51,
     "entity_count": 6
   }
   ```

2. **Breakdown by Issue Type**
   ```json
   {
     "msg": "janitor findings by type",
     "naming_convention": 10,
     "orphaned_entity": 40,
     "missing_relationship": 1,
     "stale_change": 0,
     "empty_change": 0,
     "invalid_state": 0
   }
   ```

3. **Critical Issues (if any)**
   - When critical issues are detected, each one is logged individually with:
     - Issue type
     - Entity type and ID
     - Full description

## Use Cases

### 1. Monitoring Scheduled Runs

When janitor runs on a schedule (stdio mode), the logs are visible in server logs:

```bash
# View recent janitor runs
sudo journalctl -u specmcp -n 100 | grep janitor

# Monitor for critical issues
sudo journalctl -u specmcp -f | grep "critical issues detected"
```

### 2. Alerting on Critical Issues

Can set up alerts based on log patterns:

```bash
# Alert if critical issues detected
if journalctl -u specmcp --since "5 minutes ago" | grep -q "critical issues detected"; then
  send_alert "SpecMCP janitor found critical issues"
fi
```

### 3. Trend Analysis

Track issue counts over time:

```bash
# Extract issue counts from logs
journalctl -u specmcp --since "1 week ago" | \
  grep "janitor run complete" | \
  jq -r '.time + " " + (.total_issues|tostring)'
```

## Example Output

### Healthy Project

```
{"msg":"starting janitor run","scope":"all","create_proposal":false}
{"msg":"janitor run complete","total_issues":0,"critical":0,"warnings":0,"entity_count":6}
```

### Project with Warnings

```
{"msg":"starting janitor run","scope":"all","create_proposal":false}
{"msg":"janitor run complete","total_issues":51,"critical":0,"warnings":51,"entity_count":6}
{"msg":"janitor findings by type","naming_convention":10,"orphaned_entity":40,"missing_relationship":1}
```

### Project with Critical Issues

```
{"msg":"starting janitor run","scope":"all","create_proposal":true}
{"msg":"janitor run complete","total_issues":23,"critical":3,"warnings":20,"entity_count":8}
{"msg":"janitor findings by type","naming_convention":5,"orphaned_entity":15,"invalid_state":3}
{"level":"WARN","msg":"critical issues detected","count":3}
{"level":"WARN","msg":"critical issue","type":"invalid_state","entity_type":"Spec","entity_id":"abc-123","description":"Spec marked ready but has draft requirements"}
{"level":"WARN","msg":"critical issue","type":"invalid_state","entity_type":"Requirement","entity_id":"def-456","description":"..."}
{"msg":"created maintenance proposal","proposal_id":"xyz-789","author":"janitor"}
```

## Implementation Details

### Code Changes

**File**: `internal/tools/janitor/janitor.go`

Added `logFindings()` method that:
- Groups issues by type and severity
- Logs overall statistics
- Logs breakdown by issue type
- Logs individual critical issues with WARN level

Called after report generation in `Execute()`:
```go
report.Summary = t.generateSummary(report)
t.logFindings(report)  // <- New call
```

### Log Levels

- **INFO**: Summary and breakdown (standard findings)
- **WARN**: Critical issues detected (actionable problems)

## Benefits

1. **Visibility**: Don't need to call MCP tool to see janitor status
2. **Monitoring**: Can monitor logs for issues without polling
3. **Alerting**: Can set up alerts on critical issues
4. **Trending**: Can track issue counts over time
5. **Debugging**: Easier to correlate janitor runs with other events

## Future Enhancements

Potential improvements:

1. Add metrics export (Prometheus format)
2. Add configurable log verbosity (full issue list vs summary)
3. Add issue change detection (new issues since last run)
4. Add dashboard-ready JSON format option
5. Add email/webhook notifications for critical issues

## Related

- **Janitor CodingAgent** (v0.6.2): Janitor tracks itself as an agent
- **Long Outage Handling** (v0.6.1): Connection resilience for janitor runs
- **Janitor Scheduled Runs**: Config in `specmcp.toml` for automated checks

## Deployment

```bash
# Version: v0.6.3
# Deployed: 2026-02-17
# Server: mcj-emergent
# Service: specmcp.service

# Verify logging
sudo journalctl -u specmcp -f

# Run janitor manually to see logs
curl -X POST http://localhost:21452/mcp \
  -H "Authorization: Bearer $EMERGENT_TOKEN" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"spec_janitor_run","arguments":{"scope":"all"}}}'
```
