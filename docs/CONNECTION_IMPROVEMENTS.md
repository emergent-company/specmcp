# Connection Timeout and Keep-Alive Improvements

**Date**: 2026-02-17  
**Status**: Implemented

## Overview

This document describes comprehensive connection management improvements to ensure SpecMCP maintains persistent, reliable connections to the Emergent API with automatic reconnection on failures.

## Problem

SpecMCP was experiencing connection issues:
- Connection timeouts during long-running operations (sync, large graph queries)
- Lost connections during idle periods
- No automatic reconnection after network failures
- Network instability causing permanent failures

The original implementation had:
- 30-second HTTP client timeout (too short for long operations)
- 60-120 second server timeouts (insufficient for complex queries)
- No explicit keep-alive configuration
- No connection pooling optimization
- **No retry mechanism for failed requests**

## Solution

### 1. Automatic Retry with Exponential Backoff

**Retry Logic** (`internal/emergent/client.go`):
```go
// Configurable retry behavior
maxRetries: 5           // Default (configurable, -1 = infinite)
initialBackoff: 500ms   // Start fast
maxBackoff: 1m          // Cap backoff at 1 minute
backoffSequence: 500ms, 1s, 2s, 4s, 8s, 16s, 32s, 60s, 60s...
```

**Retryable Errors**:
- Network errors (`net.Error`)
- Timeout errors (`context.DeadlineExceeded`)
- Connection errors (`net.OpError`)
- EOF and connection reset errors
- Broken pipe errors

**All Operations Are Retried**:
- `CreateObject`, `GetObject`, `GetObjects`
- `UpdateObject`, `DeleteObject`
- `CreateRelationship`, `ListRelationships`
- `ExpandGraph`, `FTSSearch`
- `ListObjects`, `CountObjects`

### 2. HTTP Client Connection Pooling

**Connection Pooling**:
```go
MaxIdleConns:        100  // Total idle connections
MaxIdleConnsPerHost: 10   // Per-host idle connections
MaxConnsPerHost:     50   // Max connections per host
IdleConnTimeout:     90s  // Keep connections in pool for 90s
```

**Keep-Alive**:
```go
DialContext: (&net.Dialer{
    Timeout:   30s  // Connection establishment
    KeepAlive: 30s  // TCP keep-alive probes
}).DialContext
```

**Timeouts**:
```go
Timeout:                   5m   // Overall request timeout (increased from 30s)
ResponseHeaderTimeout:     60s  // Time to receive response headers
TLSHandshakeTimeout:       10s  // TLS handshake
```

**Connection Reuse**:
```go
DisableKeepAlives: false         // Enable keep-alive
ForceAttemptHTTP2: true          // Use HTTP/2 when available
```

### 3. HTTP Server Improvements (`cmd/specmcp/main.go`)

**Configurable Timeouts**:
```go
ReadTimeout:  requestTimeout  // Default 5m (configurable)
WriteTimeout: requestTimeout  // Default 5m (configurable)
IdleTimeout:  idleTimeout     // Default 5m (configurable)
```

**Session Activity Tracking**:
- Track last activity time for each session
- Update activity timestamp on each request
- Helps monitor connection health

### 4. Configuration Options

**New Environment Variables**:
- `EMERGENT_MAX_RETRIES` (default: 5, set to -1 for infinite retries)
- `SPECMCP_REQUEST_TIMEOUT_MINUTES` (default: 5)
- `SPECMCP_IDLE_TIMEOUT_MINUTES` (default: 5)

**Config File** (`specmcp.toml`):
```toml
[emergent]
max_retries = 5  # or -1 for infinite

[transport]
request_timeout_minutes = 5
idle_timeout_minutes = 5
```

### 5. Documentation Updates

Updated:
- `README.md` - Added timeout configuration to environment variables table
- `AGENTS.md` - Added "Connection Management" section with technical details
- `specmcp.example.toml` - Added timeout configuration examples

## Benefits

1. **Automatic Reconnection**: Failed requests automatically retry with exponential backoff
2. **Infinite Retry Mode**: Set `EMERGENT_MAX_RETRIES=-1` to never give up on reconnecting
3. **Long Operations**: 5-minute timeout supports sync and large queries
4. **Connection Reuse**: Pool of 100 connections reduces overhead
5. **Keep-Alive**: TCP probes every 30s maintain connection health
6. **Configurability**: Timeouts and retries adjustable per deployment needs
7. **HTTP/2**: Automatic upgrade when available for better multiplexing
8. **Monitoring**: Activity tracking and retry logging for visibility
9. **Smart Retry**: Only retries transient network errors, not application errors

## Deployment Recommendations

### Default Configuration (Most Users)
```bash
# Default 5 retries with exponential backoff works for most cases
# No configuration needed
```

### Never-Give-Up Mode (Maximum Reliability)
```bash
# Infinite retries - will keep trying to reconnect forever
EMERGENT_MAX_RETRIES=-1
```

### Very Long Operations
```bash
# For operations taking >5 minutes
SPECMCP_REQUEST_TIMEOUT_MINUTES=10
SPECMCP_IDLE_TIMEOUT_MINUTES=10
EMERGENT_MAX_RETRIES=10  # More attempts for flaky networks
```

### High-Throughput Environments
```toml
[emergent]
max_retries = 3  # Fail faster in high-load scenarios

[transport]
request_timeout_minutes = 5
idle_timeout_minutes = 2  # Shorter idle timeout frees resources faster
```

### Unstable Networks (Recommended)
```toml
[emergent]
max_retries = -1  # Infinite retries for maximum resilience

[transport]
request_timeout_minutes = 10  # More time for retries
idle_timeout_minutes = 10     # Keep connections alive longer
```

## Testing

Verify the improvements:

```bash
# 1. Start server with debug logging
SPECMCP_LOG_LEVEL=debug \
SPECMCP_TRANSPORT=http \
EMERGENT_URL=http://localhost:3002 \
EMERGENT_MAX_RETRIES=5 \
./dist/specmcp

# 2. Test with infinite retries (never gives up)
SPECMCP_LOG_LEVEL=debug \
EMERGENT_MAX_RETRIES=-1 \
./dist/specmcp

# 3. Simulate network failure
# Stop Emergent, watch SpecMCP retry
# Restart Emergent, watch SpecMCP recover

# 4. Check retry behavior in logs
# Look for "retrying operation after error" messages
# Verify exponential backoff: 500ms, 1s, 2s, 4s, 8s...

# 5. Monitor long operations
# Run sync or large queries and verify they complete

# 6. Check health endpoint
curl http://localhost:21452/health
```

## Retry Behavior Examples

### Example 1: Transient Network Error (Success After Retry)
```
WARN retrying operation after error operation=get_object attempt=1 backoff=500ms error="connection refused"
WARN retrying operation after error operation=get_object attempt=2 backoff=1s error="connection refused"
INFO operation succeeded after retry operation=get_object attempts=3
```

### Example 2: Infinite Retry Mode
```
WARN retrying operation after error operation=create_object attempt=1 backoff=500ms
WARN retrying operation after error operation=create_object attempt=2 backoff=1s
...
WARN still retrying operation in infinite mode operation=create_object attempts=10
...
WARN still retrying operation in infinite mode operation=create_object attempts=20
INFO operation succeeded after retry operation=create_object attempts=23
```

### Example 3: Non-Retryable Error (Fails Fast)
```
ERROR create Spec object: validation error: name required
# No retries - application error, not network error
```

## Backward Compatibility

All changes are backward compatible:
- Default values match or exceed previous behavior
- Existing deployments work without changes
- Optional configuration for tuning

## Future Improvements

Potential enhancements:
1. **Circuit Breaker**: Fail fast when Emergent is down for extended periods
2. **Connection Health Checks**: Proactive validation before use
3. **Metrics Export**: Prometheus-style metrics for connection pool and retry stats
4. **Adaptive Timeouts**: Adjust based on historical operation duration
5. **Jitter in Backoff**: Add randomness to prevent thundering herd
6. **Configurable Backoff Strategy**: Linear, exponential, or custom

## Related Files

- `internal/emergent/client.go` - HTTP client configuration
- `cmd/specmcp/main.go` - HTTP server configuration
- `internal/config/config.go` - Configuration schema
- `internal/mcp/http.go` - HTTP transport implementation
- `README.md` - User documentation
- `AGENTS.md` - Technical documentation
- `specmcp.example.toml` - Configuration examples
