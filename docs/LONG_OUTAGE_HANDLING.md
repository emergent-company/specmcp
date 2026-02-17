# Long Outage Handling

**Status**: Implemented  
**Version**: 1.1.0  
**Last Updated**: 2026-02-17

## Overview

SpecMCP is designed to maintain persistent connections to the Emergent API and automatically reconnect when the connection is lost. The retry system uses an intelligent strategy that adapts to different types of failures:

- **Short outages** (seconds): Fast exponential backoff for quick recovery
- **Long outages** (hours): Slower, less aggressive retrying to avoid resource waste

## Retry Strategy

### Phase 1: Exponential Backoff (Fast Recovery)

For the first 20 consecutive failures (configurable), SpecMCP uses exponential backoff:

```
Attempt 1:  500ms delay
Attempt 2:  1s delay
Attempt 3:  2s delay
Attempt 4:  4s delay
Attempt 5:  8s delay
Attempt 6:  16s delay
Attempt 7:  32s delay
Attempt 8+: 60s delay (capped)
```

**Total time before long outage mode**: ~2 minutes (with 20 attempts at 60s intervals = ~20 minutes)

This phase is optimized for:
- Network hiccups
- Brief service restarts
- Temporary API unavailability
- Connection pool exhaustion

### Phase 2: Long Outage Mode (Conservative Retry)

After 20 consecutive failures (default, configurable via `EMERGENT_LONG_OUTAGE_THRESHOLD`), SpecMCP switches to **long outage mode**:

- Retries every **5 minutes** (default, configurable via `EMERGENT_LONG_OUTAGE_INTERVAL_MINS`)
- Continues indefinitely if `EMERGENT_MAX_RETRIES=-1` (infinite mode)
- Logs status every 10 attempts to track progress

This phase is optimized for:
- Extended Emergent downtime (maintenance, upgrades)
- Infrastructure outages (database, network)
- Long-running backup/restore operations

### Example Timeline

If Emergent goes down for 1 hour:

```
00:00 - Operation fails, attempt 1 starts
00:00:30 - Retry attempt 1 (500ms backoff)
00:01:30 - Retry attempt 2 (1s backoff)
00:03:30 - Retry attempt 3 (2s backoff)
00:07:30 - Retry attempt 4 (4s backoff)
00:15:30 - Retry attempt 5 (8s backoff)
00:31:30 - Retry attempt 6 (16s backoff)
01:03:30 - Retry attempt 7 (32s backoff)
02:03:30 - Retry attempt 8 (60s backoff)
03:03:30 - Retry attempt 9 (60s backoff)
...
20:03:30 - Retry attempt 20 (60s backoff)
20:08:30 - SWITCH TO LONG OUTAGE MODE (5 min backoff)
20:13:30 - Retry attempt 21 (5 min backoff)
20:18:30 - Retry attempt 22 (5 min backoff)
...
60:00 - Emergent comes back online
60:00 - Next retry succeeds!
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `EMERGENT_MAX_RETRIES` | `5` | Max retry attempts. Set to `-1` for infinite retries. |
| `EMERGENT_LONG_OUTAGE_INTERVAL_MINS` | `5` | Minutes between retries in long outage mode. |
| `EMERGENT_LONG_OUTAGE_THRESHOLD` | `20` | Consecutive failures before switching to long outage mode. |

### Config File (specmcp.toml)

```toml
[emergent]
# Never give up on reconnecting
max_retries = -1

# After 20 failures, wait 5 minutes between retries
long_outage_interval_mins = 5
long_outage_threshold = 20
```

## Use Cases

### Use Case 1: Infinite Reconnection

**Scenario**: You want SpecMCP to never give up and always reconnect when Emergent comes back online, even after hours of downtime.

**Configuration**:
```bash
EMERGENT_MAX_RETRIES=-1
EMERGENT_LONG_OUTAGE_INTERVAL_MINS=5
EMERGENT_LONG_OUTAGE_THRESHOLD=20
```

**Behavior**:
- Retries forever with exponential backoff
- After 20 failures (~20 minutes), switches to 5-minute intervals
- Keeps trying indefinitely until Emergent returns

### Use Case 2: Aggressive Reconnection

**Scenario**: You want faster recovery but still handle long outages gracefully.

**Configuration**:
```bash
EMERGENT_MAX_RETRIES=-1
EMERGENT_LONG_OUTAGE_INTERVAL_MINS=1  # Try every 1 minute
EMERGENT_LONG_OUTAGE_THRESHOLD=10     # Switch to long outage mode sooner
```

**Behavior**:
- Switches to long outage mode after 10 failures (~10 minutes)
- Retries every 1 minute in long outage mode
- More aggressive but uses more resources

### Use Case 3: Conservative Reconnection

**Scenario**: You want to minimize resource usage during extended outages.

**Configuration**:
```bash
EMERGENT_MAX_RETRIES=-1
EMERGENT_LONG_OUTAGE_INTERVAL_MINS=15  # Try every 15 minutes
EMERGENT_LONG_OUTAGE_THRESHOLD=30      # Be patient before switching
```

**Behavior**:
- Stays in exponential backoff longer (30 attempts)
- Retries every 15 minutes in long outage mode
- Minimal resource usage, slower recovery

### Use Case 4: Limited Retries with Long Outage Support

**Scenario**: You want to give up eventually, but still handle brief long outages.

**Configuration**:
```bash
EMERGENT_MAX_RETRIES=100               # Give up after 100 attempts
EMERGENT_LONG_OUTAGE_INTERVAL_MINS=5
EMERGENT_LONG_OUTAGE_THRESHOLD=20
```

**Behavior**:
- Tries 100 times total
- After 20 failures, switches to 5-minute intervals
- Gives up after ~8 hours (20 fast retries + 80 slow retries at 5 min each)

## Logging

### Normal Retry Log

```json
{
  "level": "warn",
  "msg": "retrying operation after error",
  "operation": "create Change object",
  "attempt": 5,
  "max_retries": -1,
  "backoff": "8s",
  "error": "connection refused"
}
```

### Long Outage Mode Log

```json
{
  "level": "warn",
  "msg": "retrying operation in long outage mode",
  "operation": "create Change object",
  "attempt": 25,
  "consecutive_failures": 25,
  "backoff": "5m0s",
  "error": "connection refused"
}
```

### Mode Switch Log

```json
{
  "level": "warn",
  "msg": "switching to long outage mode",
  "operation": "create Change object",
  "consecutive_failures": 20,
  "new_interval": "5m0s"
}
```

### Success After Retry Log

```json
{
  "level": "info",
  "msg": "operation succeeded after retry",
  "operation": "create Change object",
  "attempts": 26,
  "consecutive_failures": 25
}
```

## Technical Details

### Retryable vs Non-Retryable Errors

**Retryable errors** (network/connection issues):
- `connection refused`
- `connection reset by peer`
- `no such host`
- `timeout`
- `EOF` / `unexpected EOF`
- `broken pipe`
- Network errors (`net.Error`)
- Operation errors (`net.OpError`)

**Non-retryable errors** (application/validation issues):
- `404 Not Found` (entity doesn't exist)
- `400 Bad Request` (invalid data)
- `401 Unauthorized` (bad token)
- `403 Forbidden` (insufficient permissions)
- Validation errors
- Schema errors

### Backoff Calculation

```go
// Phase 1: Exponential backoff
multiplier := 1 << uint(attempt-1)  // 1, 2, 4, 8, 16, 32...
backoff := initialBackoff * multiplier
if backoff > maxBackoff {
    backoff = maxBackoff  // Cap at 60 seconds
}

// Phase 2: Long outage mode
if consecutiveFailures >= longOutageThreshold {
    backoff = longOutageInterval  // e.g., 5 minutes
}
```

### Consecutive Failure Tracking

The system tracks consecutive failures per operation. A successful operation resets the counter:

```go
consecutiveFailures := 0
for {
    err := operation()
    if err == nil {
        // Success! Reset counter
        if consecutiveFailures > 0 {
            log("recovered after N failures")
        }
        return nil
    }
    
    // Failed - increment counter
    consecutiveFailures++
    
    // Check if we should switch to long outage mode
    if consecutiveFailures >= threshold {
        // Use long outage interval
    }
}
```

## Best Practices

### Production Deployment

For production systems that must stay running:

```bash
# Never give up, reasonable long outage handling
EMERGENT_MAX_RETRIES=-1
EMERGENT_LONG_OUTAGE_INTERVAL_MINS=5
EMERGENT_LONG_OUTAGE_THRESHOLD=20
```

### Development Environment

For local development where you might stop Emergent frequently:

```bash
# Aggressive reconnection for fast feedback
EMERGENT_MAX_RETRIES=-1
EMERGENT_LONG_OUTAGE_INTERVAL_MINS=1
EMERGENT_LONG_OUTAGE_THRESHOLD=5
```

### CI/CD Pipelines

For automated testing where fast failure is preferred:

```bash
# Limited retries for fast failure detection
EMERGENT_MAX_RETRIES=10
EMERGENT_LONG_OUTAGE_INTERVAL_MINS=1
EMERGENT_LONG_OUTAGE_THRESHOLD=5
```

### Monitoring and Alerting

Set up alerts based on log patterns:

1. **Warning**: `consecutive_failures >= 10` - Emergent might be struggling
2. **Critical**: `consecutive_failures >= 20` - Entered long outage mode
3. **Recovery**: `operation succeeded after retry` - Service recovered

## Troubleshooting

### Problem: Too many retry logs

**Symptom**: Logs are flooded with retry attempts.

**Solution**: Increase `EMERGENT_LONG_OUTAGE_THRESHOLD` to switch to long outage mode sooner:

```bash
EMERGENT_LONG_OUTAGE_THRESHOLD=10
```

### Problem: Recovery takes too long

**Symptom**: Takes a long time to reconnect after Emergent comes back.

**Solution**: Decrease `EMERGENT_LONG_OUTAGE_INTERVAL_MINS` for faster checks:

```bash
EMERGENT_LONG_OUTAGE_INTERVAL_MINS=2
```

### Problem: Giving up too early

**Symptom**: SpecMCP stops trying before Emergent comes back.

**Solution**: Enable infinite retries:

```bash
EMERGENT_MAX_RETRIES=-1
```

### Problem: Using too many resources during outage

**Symptom**: High CPU/memory usage while retrying.

**Solution**: Increase long outage interval to reduce retry frequency:

```bash
EMERGENT_LONG_OUTAGE_INTERVAL_MINS=10
EMERGENT_LONG_OUTAGE_THRESHOLD=15
```

## Related Documentation

- [Connection Improvements](CONNECTION_IMPROVEMENTS.md) - Technical details of all connection enhancements
- [Retry Strategy](RETRY_STRATEGY.md) - Original retry implementation details
- [Connection Resilience Summary](CONNECTION_RESILIENCE_SUMMARY.md) - Quick reference guide
