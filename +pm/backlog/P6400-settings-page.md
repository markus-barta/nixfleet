# P6400 - Settings Page

**Created**: 2025-12-15  
**Priority**: P6400 (Low)  
**Status**: Backlog  
**Depends on**: Core dashboard (P4xxx)

---

## User Story

**As a** fleet administrator  
**I want** a settings page in the dashboard  
**So that** I can configure NixFleet behavior without editing environment variables

---

## Overview

Central settings page for dashboard configuration. Reduces need for env vars and restarts.

---

## Settings Categories

### Update Status (P5000)

| Setting               | Default | Description                               |
| --------------------- | ------- | ----------------------------------------- |
| System check interval | 5 min   | How often to check if system needs switch |
| Lock stale threshold  | 7 days  | When to glow Lock compartment             |
| Auto-refresh on load  | On      | Refresh status when dashboard loads       |

### Automated Updates (P4300)

| Setting            | Default    | Description                          |
| ------------------ | ---------- | ------------------------------------ |
| Auto-merge enabled | Off        | Automatically merge flake update PRs |
| Auto-merge delay   | 60 min     | Wait for CI before auto-merge        |
| Deploy strategy    | All online | Which hosts to deploy after merge    |

### Display

| Setting            | Default | Description                    |
| ------------------ | ------- | ------------------------------ |
| Theme              | Dark    | Dark / Light / System          |
| Refresh interval   | 30s     | Dashboard auto-refresh         |
| Show offline hosts | Yes     | Display offline hosts in table |

### Notifications (Future)

| Setting          | Default | Description             |
| ---------------- | ------- | ----------------------- |
| Email on failure | Off     | Email when deploy fails |
| Webhook URL      | -       | POST to URL on events   |

---

## Storage

Settings stored in SQLite database (not env vars):

```sql
CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT,
    updated_at DATETIME
);
```

Env vars override DB settings (for Docker/deployment flexibility).

---

## UI

Simple form page at `/settings`:

- Grouped by category
- Save button per section or global
- "Reset to defaults" option
- Changes take effect immediately (no restart)

---

## Acceptance Criteria

- [ ] Settings page accessible from header menu
- [ ] P5000 settings configurable (check interval, thresholds)
- [ ] Settings persist in database
- [ ] Env vars override DB values
- [ ] Changes apply without restart

---

## Related

- **P5000**: Update status (needs configurable intervals)
- **P4300**: Automated updates (needs enable/disable toggle)
