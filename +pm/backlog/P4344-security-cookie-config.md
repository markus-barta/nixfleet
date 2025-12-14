# P4344 - Security: Cookie Secure Flag Configuration

**Priority**: Medium  
**Status**: Pending  
**Effort**: Small  
**References**: `v2/internal/dashboard/auth.go:186`, NFR-3

## Problem

Current code has:

```go
// auth.go:186
Secure: false, // TODO: make configurable for production
```

This means session cookies can be sent over HTTP, which is:

- Required for local development (no TLS)
- **Dangerous** in production (session hijacking)

## Solution

### Configuration via Environment

```go
func (a *Auth) SetSessionCookie(w http.ResponseWriter, sessionID string) {
    isProduction := os.Getenv("NIXFLEET_ENV") == "production"
    // Or infer from TLS: if request came via HTTPS

    http.SetCookie(w, &http.Cookie{
        Name:     "session",
        Value:    sessionID,
        Path:     "/",
        MaxAge:   86400 * 7,  // 7 days
        HttpOnly: true,
        Secure:   isProduction,  // Only HTTPS in production
        SameSite: http.SameSiteLaxMode,
    })
}
```

### Alternative: Auto-Detect

```go
func isSecureRequest(r *http.Request) bool {
    // Check X-Forwarded-Proto (behind reverse proxy)
    if r.Header.Get("X-Forwarded-Proto") == "https" {
        return true
    }
    // Check TLS directly
    if r.TLS != nil {
        return true
    }
    return false
}
```

### Requirements

- [ ] Add `NIXFLEET_ENV` environment variable
- [ ] Set `Secure: true` when `NIXFLEET_ENV=production`
- [ ] Document in README/deployment docs
- [ ] Update Dockerfile to set `NIXFLEET_ENV=production`
- [ ] Test in both development and production

## Related

- NFR-3.4 (TLS) - Cookies require TLS in production
- P4400 (Deployment) - Environment configuration
