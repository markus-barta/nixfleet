# P4800 - Webhook-Triggered Deployment

**Status**: ❌ CANCELLED - Not worth the complexity  
**Closed**: 2025-12-19  
**Reason**: Item analysis concluded "Don't implement unless deploy frequency increases significantly." Current 30-second SSH deploy is sufficient.

---

## Problem

Dashboard deployment still requires SSH to csb1:

```bash
ssh csb1 "cd ~/docker && docker compose pull nixfleet && docker compose up -d"
```

This is only ~30 seconds but requires manual intervention.

---

## Solution

Dashboard exposes a webhook endpoint that GitHub calls on push, triggering self-update.

### Workflow

```
Push to nixfleet master
      │
      ▼
GitHub webhook POST to https://fleet.barta.cm/webhook/deploy
      │
      ▼
Dashboard verifies signature
      │
      ▼
Dashboard runs: docker compose pull && up -d (via Docker socket)
      │
      ▼
New container starts with updated code
```

---

## Implementation

### 1. Webhook Handler

`v2/internal/dashboard/webhook.go`:

```go
package dashboard

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "net/http"
    "os/exec"
)

func (s *Server) handleWebhookDeploy(w http.ResponseWriter, r *http.Request) {
    // Verify GitHub signature
    signature := r.Header.Get("X-Hub-Signature-256")
    if !s.verifySignature(r, signature) {
        http.Error(w, "Invalid signature", http.StatusUnauthorized)
        return
    }

    // Verify event type
    event := r.Header.Get("X-GitHub-Event")
    if event != "push" {
        w.WriteHeader(http.StatusOK)
        return
    }

    // Trigger update in background
    go s.triggerSelfUpdate()

    w.WriteHeader(http.StatusAccepted)
    w.Write([]byte("Deployment triggered"))
}

func (s *Server) verifySignature(r *http.Request, signature string) bool {
    secret := os.Getenv("WEBHOOK_SECRET")
    if secret == "" {
        return false
    }

    body, _ := io.ReadAll(r.Body)
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(body)
    expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))

    return hmac.Equal([]byte(signature), []byte(expected))
}

func (s *Server) triggerSelfUpdate() {
    s.log.Info().Msg("webhook triggered self-update")

    // Pull new image
    exec.Command("docker", "compose", "pull", "nixfleet").Run()

    // Restart with new image (this will kill current process)
    exec.Command("docker", "compose", "up", "-d", "nixfleet").Run()
}
```

### 2. Register Route

```go
r.Post("/webhook/deploy", s.handleWebhookDeploy)
```

### 3. GitHub Webhook Setup

1. GitHub repo → Settings → Webhooks → Add webhook
2. Payload URL: `https://fleet.barta.cm/webhook/deploy`
3. Content type: `application/json`
4. Secret: Generate random secret
5. Events: Just "push"

### 4. Docker Socket Access

Container needs Docker socket mounted:

```yaml
services:
  nixfleet:
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
```

---

## Why Low Priority?

| Factor           | Reality                                         |
| ---------------- | ----------------------------------------------- |
| Deploy frequency | Weekly at most                                  |
| Manual effort    | 30 seconds via SSH                              |
| Complexity added | Webhook security, Docker socket, error handling |
| Risk             | Self-update bugs could break dashboard          |

The effort/value ratio doesn't justify this yet.

---

## Risks & Mitigations

| Risk                            | Mitigation                                      |
| ------------------------------- | ----------------------------------------------- |
| Invalid webhook triggers deploy | HMAC signature verification                     |
| Update fails mid-way            | Docker handles graceful restart                 |
| New version is broken           | Need manual rollback via SSH anyway             |
| Docker socket = root access     | Container runs as non-root, socket is read-only |

---

## Acceptance Criteria

- [ ] Webhook endpoint exists at `/webhook/deploy`
- [ ] HMAC signature verification works
- [ ] Only push events trigger deploy
- [ ] Container restarts with new image
- [ ] Unauthorized requests are rejected

---

## Alternative: Keep It Simple

Instead of webhooks, create a simple deploy script:

```bash
#!/bin/bash
# deploy-dashboard.sh (run from your laptop)
ssh csb1 "cd ~/docker && docker compose pull nixfleet && docker compose up -d"
```

This is:

- Zero new code
- No security concerns
- Works today
- 30 seconds max

**Recommendation**: Don't implement this unless deploy frequency increases significantly.

---

## Related

- P4600 - GitHub Actions Docker Build (prerequisite)
- P4700 - Automated flake.lock Updates
