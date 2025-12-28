# P5400 - Minimal Settings UI

**Created**: 2025-12-28 (Split from P6400)  
**Priority**: P5400 (🟢 After Compartment Epic)  
**Status**: Backlog  
**Effort**: Small (1 day)  
**Depends on**: P3900 (Tests Compartment)

---

## User Story

**As a** fleet administrator  
**I want** basic settings UI for test configuration  
**So that** I can enable/disable test auto-run per host without editing env vars

---

## Scope

**Minimal settings for P5400 (Tests Compartment)**:

1. Per-host test configuration:
   - Enable/disable tests
   - Auto-run tests after switch (on/off)
   - Test timeout (seconds)

2. Global test settings:
   - Default auto-run behavior
   - Default timeout

---

## UI Design

### Settings Button

```
Dashboard → ⚙️ Settings (top-right)
```

### Settings Modal

```
┌─────────────────────────────────────────────────┐
│ Settings                                    [×] │
├─────────────────────────────────────────────────┤
│                                                 │
│ Test Configuration                              │
│                                                 │
│ ┌─────────────────────────────────────────────┐ │
│ │ Host          Tests   Auto-run   Timeout   │ │
│ ├─────────────────────────────────────────────┤ │
│ │ gpc0          ✓       ✓          60s       │ │
│ │ imac0         ✓       ✗          60s       │ │
│ │ hsb0          ✗       -          -         │ │
│ └─────────────────────────────────────────────┘ │
│                                                 │
│ Global Defaults:                                │
│   Auto-run tests: [✓] Enabled                  │
│   Default timeout: [60] seconds                │
│                                                 │
│                   [Cancel]  [Save]              │
└─────────────────────────────────────────────────┘
```

---

## API Endpoints

```go
// Get current settings
GET /api/settings

// Update settings
PUT /api/settings
{
  "hosts": {
    "gpc0": {
      "tests_enabled": true,
      "tests_auto_run": true,
      "tests_timeout": 60
    }
  },
  "global": {
    "tests_auto_run_default": true,
    "tests_timeout_default": 60
  }
}
```

---

## Storage

Settings stored in SQLite:

```sql
CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Per-host settings stored as JSON
INSERT INTO settings (key, value) VALUES
  ('host_gpc0_tests', '{"enabled":true,"auto_run":true,"timeout":60}');

-- Global settings
INSERT INTO settings (key, value) VALUES
  ('global_tests_auto_run', 'true'),
  ('global_tests_timeout', '60');
```

---

## Acceptance Criteria

- [ ] Settings button in dashboard header
- [ ] Modal shows per-host test configuration
- [ ] Can toggle test auto-run per host
- [ ] Can adjust timeout per host
- [ ] Global defaults apply to new hosts
- [ ] Settings persist across dashboard restart
- [ ] Settings sync to agents via State Sync

---

## Out of Scope

**Deferred to P6400b (Full Settings UI)**:

- GitHub token management
- Heartbeat interval configuration
- CSRF settings
- Theme/appearance settings
- User management
- Backup/restore settings

---

## Related

- **P3900**: Tests Compartment (uses these settings)
- **P7600**: Full Settings UI (future enhancement)
