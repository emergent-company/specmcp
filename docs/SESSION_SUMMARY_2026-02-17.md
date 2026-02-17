# SpecMCP Development Summary - Session 2026-02-17

## Summary

Successfully completed three major improvements to SpecMCP:

1. ✅ **Long Outage Handling** (v0.6.1) - Intelligent retry strategy for Emergent connection
2. ✅ **Janitor as Agent** (v0.6.2) - Janitor tracks itself and authors proposals
3. ✅ **Janitor Logging** (v0.6.3) - Structured logging for better monitoring

All features are now deployed to production on mcj-emergent.

---

## Feature 1: Long Outage Handling (v0.6.1)

### Problem
SpecMCP would give up trying to connect to Emergent after a few retries, making it unsuitable for long-running deployments where the backend might restart.

### Solution
Implemented two-phase retry strategy:
- **Phase 1 (Short Outages)**: Exponential backoff up to 20 failures
- **Phase 2 (Long Outages)**: Switch to 5-minute intervals, keep trying forever

### Configuration
```bash
EMERGENT_MAX_RETRIES=-1                    # Never give up
EMERGENT_LONG_OUTAGE_INTERVAL_MINS=5       # Wait 5 minutes in long outage mode
EMERGENT_LONG_OUTAGE_THRESHOLD=20          # Switch to long outage after 20 failures
```

### Deployment
- Commit: `dce8b63`
- Version: v0.6.1
- Server: mcj-emergent
- Verified: Connection resilience tested with backend restarts

### Documentation
- `docs/LONG_OUTAGE_HANDLING.md` - Usage guide
- `docs/LONG_OUTAGE_IMPLEMENTATION_SUMMARY.md` - Implementation details
- `docs/CONNECTION_RESILIENCE_SUMMARY.md` - Updated overview
- `README.md` - Environment variables documented

---

## Feature 2: Janitor as Agent (v0.6.2)

### Problem
The janitor agent creates maintenance proposals but wasn't tracking itself as the author, making it unclear who/what created them.

### Solution
- Added `GetOrCreateAgent()` helper function
- Janitor creates/gets its Agent entry on each run
- Maintenance proposals are linked via `proposed_by` relationship

### Janitor Agent
```yaml
ID: b2bcc109-7de7-48b2-af9b-906ad475e61f
Name: janitor
Type: ai
Specialization: maintenance
Skills: [validation, compliance, cleanup]
Tags: [system, automation, maintenance]
```

### Implementation
**Files Modified:**
- `internal/emergent/entities.go` - Helper functions
- `internal/tools/janitor/janitor.go` - Self-tracking logic

### Deployment
- Commit: `da4f7c2`
- Version: v0.6.2
- Server: mcj-emergent
- Verified: Janitor agent exists in graph

### Current Findings
Janitor successfully finds and reports:
- **51 warnings** (0 critical)
  - 10 naming convention issues (spaces instead of kebab-case)
  - 40 orphaned entities (not connected to any Change)
  - 1 missing relationship (Requirement without scenarios)

### Documentation
Documented in commit messages and code comments.

---

## Feature 3: Janitor Logging (v0.6.3)

### Problem
Janitor findings were only visible in MCP responses. For scheduled runs or monitoring, there was no visibility into what issues exist without calling the tool.

### Solution
Added structured logging to janitor that outputs:
1. Overall summary (total issues, critical, warnings)
2. Breakdown by issue type
3. Individual logs for critical issues

### Log Format

**Summary:**
```json
{
  "msg": "janitor run complete",
  "total_issues": 51,
  "critical": 0,
  "warnings": 51,
  "entity_count": 6
}
```

**Breakdown:**
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

**Critical Issues (when present):**
```json
{
  "level": "WARN",
  "msg": "critical issues detected",
  "count": 3
}
{
  "level": "WARN",
  "msg": "critical issue",
  "type": "invalid_state",
  "entity_type": "Spec",
  "entity_id": "abc-123",
  "description": "Spec marked ready but has draft requirements"
}
```

### Monitoring Commands

```bash
# View recent janitor runs
sudo journalctl -u specmcp -n 100 | grep janitor

# Monitor for critical issues in real-time
sudo journalctl -u specmcp -f | grep "critical issues detected"

# Track issue trends over time
journalctl -u specmcp --since "1 week ago" | grep "janitor run complete"
```

### Implementation
**Files Modified:**
- `internal/tools/janitor/janitor.go` - Added `logFindings()` method

### Deployment
- Commit: `55dcbee`
- Version: v0.6.3
- Server: mcj-emergent
- Verified: Logs appear in systemd journal

### Documentation
- `docs/JANITOR_LOGGING.md` - Comprehensive guide
- `AGENTS.md` - Added logging section with examples

---

## Deployment History

| Version | Feature | Commit | Status |
|---------|---------|--------|--------|
| v0.6.1 | Long Outage Handling | dce8b63 | ✅ Deployed |
| v0.6.2 | Janitor as Agent | da4f7c2 | ✅ Deployed |
| v0.6.3 | Janitor Logging | 55dcbee | ✅ Deployed |

**Current Production Version**: v0.6.3  
**Server**: mcj-emergent  
**Service**: specmcp.service (systemd)  
**Status**: Active and running

---

## Verification

### Long Outage Handling
```bash
# Test script exists on server
~/test-janitor-connection.sh

# Verified with backend restarts
sudo systemctl restart emergent
# SpecMCP successfully reconnected
```

### Janitor Agent
```bash
# Query for janitor agent
curl -X POST http://localhost:3002/api/v1/graph/query \
  -H "Authorization: Bearer $EMERGENT_TOKEN" \
  -d '{"filters":[{"key":"name","value":"janitor","type":"string"}]}'

# Result: Agent exists with ID b2bcc109-7de7-48b2-af9b-906ad475e61f
```

### Janitor Logging
```bash
# Run janitor and check logs
bash ~/test-janitor-connection.sh > /dev/null
sudo journalctl -u specmcp -n 10 | grep janitor

# Result: Structured logs appear with issue breakdown
```

---

## Files Modified

### Long Outage Handling
- `internal/config/config.go`
- `internal/emergent/client.go`
- `cmd/specmcp/main.go`
- `cmd/test-janitor/main.go`
- `README.md`
- `specmcp.example.toml`

### Janitor Agent
- `internal/emergent/entities.go`
- `internal/tools/janitor/janitor.go`

### Janitor Logging
- `internal/tools/janitor/janitor.go`

---

## Documentation Created

1. `docs/LONG_OUTAGE_HANDLING.md` - Usage guide for long outage feature
2. `docs/LONG_OUTAGE_IMPLEMENTATION_SUMMARY.md` - Implementation details
3. `docs/CONNECTION_RESILIENCE_SUMMARY.md` - Updated with new features
4. `docs/JANITOR_LOGGING.md` - Comprehensive logging guide
5. `AGENTS.md` - Updated with logging examples

---

## Next Steps / Future Enhancements

### Janitor Improvements
1. **Metrics Export**: Add Prometheus-format metrics for monitoring
2. **Issue Change Detection**: Track what's new/fixed since last run
3. **Dashboard Integration**: JSON format optimized for dashboards
4. **Email/Webhook Alerts**: Notifications for critical issues
5. **Auto-fix Capabilities**: Automatically fix naming convention issues

### Testing
1. **Proposal Creation Testing**: Need to trigger critical issues to verify `proposed_by` relationship
2. **Load Testing**: Test janitor performance with large graphs
3. **Scheduled Run Testing**: Verify scheduled janitor behavior in production

### Monitoring Setup
1. Set up alerts for critical issues
2. Create dashboard for tracking issue trends
3. Document runbook for responding to janitor alerts

---

## Lessons Learned

### Two-Phase Retry Strategy
- Simple exponential backoff isn't enough for long outages
- Need to balance responsiveness (quick recovery) with resource usage (long outages)
- Configurable thresholds allow tuning for different deployment scenarios

### Self-Tracking Agents
- Agents should track themselves in the graph they manage
- `GetOrCreate` pattern is useful for idempotent initialization
- Agent metadata (skills, specialization) helps understand who did what

### Structured Logging
- JSON logs are essential for monitoring and alerting
- Log levels matter: INFO for normal operations, WARN for actionable issues
- Breakdowns by category make logs actionable without manual parsing
- Individual critical issue logs enable targeted investigation

### Deployment Process
- Test locally first, verify compilation
- Use semantic versioning for releases
- Tag before deploying
- Always verify deployment with test script
- Check systemd logs to confirm behavior

---

## Related Issues / PRs

- Feature request: Long outage handling for production deployments
- Feature request: Janitor should track itself as an agent
- Enhancement: Improve visibility of janitor findings

---

## Contact

For questions or issues with these features:
- GitHub: https://github.com/emergent-company/specmcp
- Documentation: See `docs/` directory in repository
