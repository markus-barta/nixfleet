# P4344 - Security: Cookie Secure Flag Configuration

**Priority**: Medium  
**Status**: Done  
**Effort**: Small  
**References**: `v2/internal/dashboard/auth.go`, NFR-3  
**Completed**: 2025-12-14

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

### Alternative: Auto-Detect (I THINK WE WANT TO USE THIS)

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

- [x] ~~Add `NIXFLEET_ENV` environment variable~~ (used auto-detect instead)
- [x] Set `Secure: true` when HTTPS detected (auto-detect from request)
- [x] Document in README/deployment docs (auto-detect, no config needed)
- [x] ~~Update Dockerfile to set `NIXFLEET_ENV=production`~~ (not needed)
- [ ] Test in both development and production (manual verification)

## Related

- NFR-3.4 (TLS) - Cookies require TLS in production
- P4400 (Deployment) - Environment configuration
