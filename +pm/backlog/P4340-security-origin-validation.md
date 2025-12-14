# P4340 - Security: WebSocket Origin Validation

**Priority**: Critical  
**Status**: Pending  
**Effort**: Small  
**References**: `v2/internal/dashboard/handlers.go:18`, NFR-3

## Problem

Current code has:

```go
// handlers.go:18
return true // TODO: validate origin in production
```

This means **any origin** can connect WebSockets to the dashboard, enabling:

- Cross-site WebSocket hijacking
- Malicious sites controlling fleet via logged-in user's session
- Data exfiltration from command output

## Solution

### Validate Origin Header

```go
func (s *Server) checkOrigin(r *http.Request) bool {
    origin := r.Header.Get("Origin")
    if origin == "" {
        // No origin = same-origin request (OK)
        return true
    }

    // Parse origin
    originURL, err := url.Parse(origin)
    if err != nil {
        return false
    }

    // Check against allowed origins
    host := r.Host
    if strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.0.0.1") {
        // Development: allow localhost
        return originURL.Host == host
    }

    // Production: strict match
    expectedOrigin := fmt.Sprintf("https://%s", host)
    return origin == expectedOrigin
}
```

### Configuration Option

Allow override via environment variable for development:

```go
allowedOrigins := os.Getenv("NIXFLEET_ALLOWED_ORIGINS")
if allowedOrigins != "" {
    origins := strings.Split(allowedOrigins, ",")
    for _, allowed := range origins {
        if origin == strings.TrimSpace(allowed) {
            return true
        }
    }
}
```

### Requirements

- [ ] Implement `checkOrigin` function
- [ ] Validate against request Host header
- [ ] Allow localhost in development
- [ ] Add `NIXFLEET_ALLOWED_ORIGINS` env var
- [ ] Log rejected origins for debugging
- [ ] Test with browser console

## Related

- NFR-3.4 (TLS) - Production requires HTTPS
- T05 (Dashboard WebSocket) - Origin validation in spec
