# P4400 - Go Dashboard Deployment

**Priority**: High (blocks E2E testing)
**Depends on**: T04-T06 (dashboard tests) ✅ Complete
**Blocks**: T07-T08 (real E2E tests)

---

## Goal

Deploy the v2 Go dashboard to csb1, replacing the v1 Python dashboard.

---

## Server

| Item    | Value              |
| ------- | ------------------ |
| Host    | csb1               |
| Domain  | fleet.barta.cm     |
| Current | v1 Python (Docker) |
| Target  | v2 Go (Docker)     |

---

## Current v1 Architecture (csb1)

```
docker/nixfleet/
├── docker-compose.yml
├── .env              # Secrets
├── data/
│   └── nixfleet.db   # SQLite database
└── update.sh
```

**v1 Stack**:

- FastAPI (Python)
- Jinja2 templates
- SQLite database
- SSE for browser updates
- HTTP polling for agents

---

## Target v2 Architecture (csb1)

```
docker/nixfleet/
├── docker-compose.yml    # Updated for Go
├── .env                  # Secrets (mostly same)
├── data/
│   └── nixfleet.db       # SQLite (schema migration)
└── update.sh             # Updated for Go container
```

**v2 Stack**:

- Go (Chi router)
- Templ templates (future) / Simple HTML (current)
- SQLite database (same file, migrated schema)
- WebSocket for all communication

---

## Tasks

### 1. Create v2 Dockerfile

```dockerfile
# Dockerfile.v2
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY v2/ .
RUN go build -o nixfleet-dashboard ./cmd/nixfleet-dashboard

FROM alpine:3.19
COPY --from=builder /app/nixfleet-dashboard /usr/local/bin/
EXPOSE 8000
CMD ["nixfleet-dashboard"]
```

### 2. Update docker-compose.yml

```yaml
services:
  nixfleet:
    build:
      context: .
      dockerfile: Dockerfile.v2
    environment:
      - NIXFLEET_PASSWORD_HASH=${NIXFLEET_PASSWORD_HASH}
      - NIXFLEET_SESSION_SECRET=${NIXFLEET_SESSION_SECRET}
      - NIXFLEET_AGENT_TOKEN=${NIXFLEET_AGENT_TOKEN}
      - NIXFLEET_TOTP_SECRET=${NIXFLEET_TOTP_SECRET}
      - NIXFLEET_DB_PATH=/data/nixfleet.db
    volumes:
      - ./data:/data
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.nixfleet.rule=Host(`fleet.barta.cm`)"
```

### 3. Database Migration

v1 schema → v2 schema:

- Sessions table: same structure
- Hosts table: add new v2 fields
- Command logs: compatible
- Metrics: new table

```sql
-- v2 migration (auto-applied by InitDatabase)
ALTER TABLE hosts ADD COLUMN nixpkgs_version TEXT;
ALTER TABLE hosts ADD COLUMN heartbeat_interval INTEGER DEFAULT 30;
-- etc.
```

### 4. Deployment Steps

```bash
# On csb1
ssh mba@cs1.barta.cm -p 2222

# Backup current state
cd ~/docker/nixfleet
cp data/nixfleet.db data/nixfleet.db.v1-backup
docker compose down

# Update to v2
git pull
docker compose build
docker compose up -d

# Verify
docker logs nixfleet-nixfleet-1 -f
curl https://fleet.barta.cm/login
```

### 5. Verify Agent Connectivity

After dashboard is up:

1. Wait for agents to reconnect (they'll fail with v1 protocol)
2. Deploy v2 agents (P4100)
3. Agents connect via WebSocket

---

## Rollback Plan

```bash
# On csb1
cd ~/docker/nixfleet
docker compose down

# Restore v1
git checkout HEAD~1
cp data/nixfleet.db.v1-backup data/nixfleet.db
docker compose build
docker compose up -d
```

---

## Environment Variables

| Variable                | v1  | v2  | Notes                   |
| ----------------------- | --- | --- | ----------------------- |
| NIXFLEET_PASSWORD_HASH  | ✅  | ✅  | Same (bcrypt)           |
| NIXFLEET_SESSION_SECRET | ✅  | ✅  | Same                    |
| NIXFLEET_TOTP_SECRET    | ✅  | ✅  | Same                    |
| NIXFLEET_AGENT_TOKEN    | New | ✅  | Single token for agents |
| NIXFLEET_DB_PATH        | ✅  | ✅  | Same                    |

---

## Acceptance Criteria

- [ ] v2 dashboard builds as Docker image
- [ ] v2 dashboard running on csb1
- [ ] Login with password + TOTP works
- [ ] WebSocket endpoint accepting connections
- [ ] v2 agents can connect and register
- [ ] Browser receives real-time updates

---

## Decisions

1. **Brief downtime OK** - Simple swap, < 5 min
2. **Same domain** - Keep `fleet.barta.cm`
3. **Auto migration** - Go code handles it, backup first
4. **Reuse TOTP** - Same secret, no re-enrollment
