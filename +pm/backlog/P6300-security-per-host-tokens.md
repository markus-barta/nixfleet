# P6300 - Security: Per-Host Agent Tokens

**Created**: 2025-12-14  
**Updated**: 2025-12-28  
**Priority**: P7400 (⚪ Low Priority - Future)  
**Status**: Backlog  
**Effort**: Medium  
**References**: NFR-3.3, T01 (Agent Connection)

**Note**: Priority lowered - nice-to-have security feature

## Problem

Currently all agents share the same token (`NIXFLEET_API_TOKEN`). This means:

- Compromised agent can impersonate any host
- Cannot revoke access for single host
- No audit trail per host

**Decision**: Per-host tokens are **optional**. Shared token works for simple setups; per-host tokens recommended for production.

PRD NFR-3.3: "Agent tokens - Per-host, hashed in DB" - **Optional enhancement**

## Current State

```bash
# All hosts use same token
NIXFLEET_API_TOKEN="shared-secret-token"
```

## Solution

### 1. Generate Token Per Host

When adding a host (via modal or first connection):

```go
token := generateSecureToken()
hashedToken := bcrypt.GenerateFromPassword(token, bcrypt.DefaultCost)
// Store hashedToken in DB, give plaintext to user once
```

### 2. Store in Database

```sql
CREATE TABLE host_tokens (
    host_id TEXT PRIMARY KEY,
    token_hash TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMP
);
```

### 3. Validate on Connection

```go
func (h *Hub) validateAgentToken(hostname, token string) bool {
    var hashedToken string
    err := h.db.QueryRow(
        "SELECT token_hash FROM host_tokens WHERE host_id = ?",
        hostname,
    ).Scan(&hashedToken)
    if err != nil {
        return false
    }
    return bcrypt.CompareHashAndPassword(hashedToken, token) == nil
}
```

### 4. Token Generation UI

In Add Host modal:

- Generate token on creation
- Display once with "Copy" button
- Show warning: "Save this token, you won't see it again"

### 5. Token Rotation

In host dropdown menu:

- "Regenerate Token" option
- Confirms with modal
- Generates new token, invalidates old

### Migration Path

1. Keep supporting shared token for existing hosts
2. New hosts get per-host tokens
3. Gradual migration via "Regenerate Token"

### Requirements

- [ ] Create `host_tokens` table
- [ ] Generate unique token on host creation
- [ ] Hash token with bcrypt before storing
- [ ] Validate token on WebSocket connection
- [ ] Show token once in UI after creation
- [ ] Add "Regenerate Token" to dropdown
- [ ] Update agent module to support per-host config
- [ ] Document token management in README

## Related

- P4390 (Modals) - Add Host shows token
- P4380 (Dropdown) - Regenerate Token option
- T01 (Agent Connection) - Token validation
