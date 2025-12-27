# NixFleet 2.0 UX Flow — Control-Action-Command Reference

> **Purpose:** Map all UI controls, actions, and commands for flow analysis and harmonization.

---

## Legend

| Tag | Meaning         |
| --- | --------------- |
| ✅  | Working         |
| ⚠️  | Partial/Buggy   |
| ❌  | Broken          |
| 🚧  | Not Implemented |

**Scope:** `S` = Single host, `B` = Bulk (selected hosts), `G` = Global

---

## 1. Header Menu (☰ More)

| ID                 | Action               | Scope | Command(s)                                 | Outcome                                           | Status |
| ------------------ | -------------------- | ----- | ------------------------------------------ | ------------------------------------------------- | ------ |
| `CAC-MERGE-DEPLOY` | Merge & Deploy PR    | G     | API: `/api/flake-updates/merge-and-deploy` | Merges PR, triggers pull+switch on affected hosts | ✅     |
| `CAC-DO-ALL-BULK`  | Do All               | B     | `pull` → `switch` → `test` (sequential)    | Full update cycle on selected hosts               | ✅     |
| `CAC-PULL-BULK`    | Pull All             | B     | `pull`                                     | Git pull on all selected hosts                    | ✅     |
| `CAC-SWITCH-BULK`  | Switch All           | B     | `switch`                                   | nixos-rebuild on all selected hosts               | ✅     |
| `CAC-TEST-BULK`    | Test All             | B     | `test`                                     | Run host tests on all selected hosts              | ✅     |
| `CAC-RESTART-BULK` | Restart All Agents   | B     | `restart`                                  | Restart agent service on all selected hosts       | ✅     |
| `CAC-CACHE-CLEAR`  | Clear Cache & Reload | G     | JS: `forceRefresh()`                       | Clears browser cache, reloads page                | ✅     |
| `CAC-DEBUG-TOGGLE` | Toggle Debug Panel   | G     | JS: `toggleDebugPanel()`                   | Shows/hides debug info                            | ✅     |

---

## 2. Context Bar (when hosts selected)

| ID               | Action          | Scope | Command(s)                 | Outcome                     | Status |
| ---------------- | --------------- | ----- | -------------------------- | --------------------------- | ------ |
| `CAC-CTX-PULL`   | Pull            | B     | `pull`                     | Git pull on selected hosts  | ✅     |
| `CAC-CTX-SWITCH` | Switch          | B     | `switch`                   | Rebuild on selected hosts   | ✅     |
| `CAC-CTX-TEST`   | Test            | B     | `test`                     | Run tests on selected hosts | ✅     |
| `CAC-CTX-DO-ALL` | Do All          | B     | `pull` → `switch` → `test` | Sequential update cycle     | ✅     |
| `CAC-CTX-CLEAR`  | Clear Selection | B     | —                          | Deselects all hosts         | ✅     |

### Context Bar — Timeout Notifications

| ID                   | Action | Scope | Command(s)                            | Outcome                                  | Status |
| -------------------- | ------ | ----- | ------------------------------------- | ---------------------------------------- | ------ |
| `CAC-TIMEOUT-WAIT`   | Wait   | S     | API: `/api/hosts/{id}/timeout-action` | Extends timeout by 60s                   | ✅     |
| `CAC-TIMEOUT-KILL`   | Kill   | S     | API: `/api/hosts/{id}/kill`           | Sends SIGTERM/SIGKILL to command         | ✅     |
| `CAC-TIMEOUT-IGNORE` | Ignore | S     | API: `/api/hosts/{id}/timeout-action` | Dismisses warning, lets command continue | ✅     |

---

## 3. Host Row — Inline Action Buttons

| ID               | Action | Scope | Command(s) | Outcome                                        | Status |
| ---------------- | ------ | ----- | ---------- | ---------------------------------------------- | ------ |
| `CAC-ROW-PULL`   | Pull   | S     | `pull`     | `git fetch && git reset --hard`                | ✅     |
| `CAC-ROW-SWITCH` | Switch | S     | `switch`   | `nixos-rebuild switch` / `home-manager switch` | ✅     |
| `CAC-ROW-TEST`   | Test   | S     | `test`     | Runs host test scripts                         | ✅     |

---

## 4. Host Row — Ellipsis Menu (⋮)

| ID                       | Action          | Scope | Command(s)                         | Outcome                           | Status |
| ------------------------ | --------------- | ----- | ---------------------------------- | --------------------------------- | ------ |
| `CAC-MENU-PULL`          | Pull            | S     | `pull`                             | Git pull                          | ✅     |
| `CAC-MENU-SWITCH`        | Switch          | S     | `switch`                           | Rebuild                           | ✅     |
| `CAC-MENU-TEST`          | Test            | S     | `test`                             | Run tests                         | ✅     |
| `CAC-MENU-STOP`          | Stop Agent      | S     | `stop`                             | Graceful agent shutdown           | ✅     |
| `CAC-MENU-RESTART`       | Restart Agent   | S     | `restart`                          | Agent restart via systemd/launchd | ✅     |
| `CAC-MENU-REBOOT`        | Reboot Host     | S     | `reboot` (with TOTP)               | Full system reboot                | ✅     |
| `CAC-MENU-FORCE-UPDATE`  | Force Update    | S     | `force-update`                     | Currently: same as switch         | ⚠️     |
| `CAC-MENU-COLOR`         | Set Theme Color | S     | API: `/api/hosts/{id}/theme-color` | Sets host color in UI             | ✅     |
| `CAC-MENU-LOGS`          | View Logs       | S     | —                                  | Opens host log panel              | ✅     |
| `CAC-MENU-DOWNLOAD-LOGS` | Download Logs   | S     | API: `/api/hosts/{id}/logs`        | Downloads log file                | ✅     |
| `CAC-MENU-REMOVE`        | Remove Host     | S     | API: `DELETE /api/hosts/{id}`      | Removes host from dashboard       | ✅     |

---

## 5. Compartment Clicks (Status Icons)

| ID                | Compartment | Action         | Command(s)                         | Outcome                                     | Status |
| ----------------- | ----------- | -------------- | ---------------------------------- | ------------------------------------------- | ------ |
| `CAC-COMP-AGENT`  | Agent       | Check Version  | `check-version`                    | Compares running vs installed agent version | ✅     |
| `CAC-COMP-GIT`    | Git         | Refresh Status | API: `/api/hosts/{id}/refresh-git` | Fetches GitHub API for remote status        | ✅     |
| `CAC-COMP-LOCK`   | Lock        | Refresh Status | `refresh-lock`                     | Runs `git log` comparison on host           | ✅     |
| `CAC-COMP-SYSTEM` | System      | Refresh Status | `refresh-system` (with confirm)    | Runs `nix build --dry-run` (expensive!)     | ✅     |

**Behavior:** Click shows toast (TL;DR) + detailed log message.

---

## 6. Read-Only / Hover Interactions

| ID                      | Control         | Action | Outcome                                 | Status |
| ----------------------- | --------------- | ------ | --------------------------------------- | ------ |
| `CAC-HOVER-COMPARTMENT` | Any Compartment | Hover  | Shows status description in context bar | ✅     |
| `CAC-HOVER-HOSTNAME`    | Hostname        | Hover  | Shows copy icon                         | ✅     |
| `CAC-CLICK-HOSTNAME`    | Hostname        | Click  | Copies hostname to clipboard            | ✅     |
| `CAC-CLICK-CHECKBOX`    | Row Checkbox    | Click  | Toggles host selection                  | ✅     |
| `CAC-CLICK-SELECT-ALL`  | Header Checkbox | Click  | Selects/deselects all hosts             | ✅     |

---

## 7. Modals / Dialogs

| ID                         | Trigger                  | Action            | Outcome                                | Status |
| -------------------------- | ------------------------ | ----------------- | -------------------------------------- | ------ |
| `CAC-MODAL-REBOOT`         | Reboot menu item         | Confirm with TOTP | Executes reboot on host                | ✅     |
| `CAC-MODAL-REMOVE`         | Remove menu item         | Confirm           | Deletes host from DB                   | ✅     |
| `CAC-MODAL-ADD-HOST`       | Add Host button          | Submit form       | Registers new host                     | ✅     |
| `CAC-MODAL-COLOR`          | Color menu item          | Pick color        | Saves theme color                      | ✅     |
| `CAC-MODAL-PRECHECK`       | Failed pre-validation    | Force/Cancel/Alt  | Retry with force or alternative action | ✅     |
| `CAC-MODAL-SYSTEM-REFRESH` | System compartment click | Confirm           | Runs expensive nix build --dry-run     | ✅     |
| `CAC-MODAL-MULTI-ACTION`   | Multi-host lock outdated | Choose action     | Pull/Switch/Pull+Switch options        | ✅     |

---

## 8. Log Panel

| ID                  | Control         | Action | Outcome                 | Status |
| ------------------- | --------------- | ------ | ----------------------- | ------ |
| `CAC-LOG-TAB`       | Tab             | Click  | Switches to host log    | ✅     |
| `CAC-LOG-CLEAR`     | Clear button    | Click  | Clears current log tab  | ✅     |
| `CAC-LOG-COPY`      | Copy button     | Click  | Copies log to clipboard | ✅     |
| `CAC-LOG-CLOSE`     | Close button    | Click  | Closes tab              | ✅     |
| `CAC-LOG-FONT-UP`   | + button        | Click  | Increases font size     | ✅     |
| `CAC-LOG-FONT-DOWN` | − button        | Click  | Decreases font size     | ✅     |
| `CAC-LOG-COLLAPSE`  | Collapse button | Click  | Minimizes log panel     | ✅     |

---

## 9. Agent Commands (Backend)

All commands the agent can execute:

| Command          | Description                                     | Pre-validation     | Post-validation               | Status |
| ---------------- | ----------------------------------------------- | ------------------ | ----------------------------- | ------ |
| `pull`           | `git fetch && git reset --hard`                 | Checks git status  | Sets Lock=ok, System=outdated | ✅     |
| `switch`         | `nixos-rebuild switch` or `home-manager switch` | Checks Lock status | Sets System=ok                | ✅     |
| `pull-switch`    | Pull then Switch                                | Combined           | Combined                      | ✅     |
| `test`           | Run host test scripts                           | —                  | Validates test results        | ✅     |
| `restart`        | Restart agent service                           | —                  | Waits for reconnect           | ✅     |
| `stop`           | Stop agent service                              | —                  | —                             | ✅     |
| `reboot`         | System reboot                                   | TOTP required      | —                             | ✅     |
| `refresh-git`    | Refresh Git compartment                         | —                  | —                             | ✅     |
| `refresh-lock`   | Refresh Lock compartment                        | —                  | —                             | ✅     |
| `refresh-system` | `nix build --dry-run`                           | Confirm dialog     | —                             | ✅     |
| `refresh-all`    | Refresh all compartments                        | —                  | —                             | ✅     |
| `check-version`  | Compare running vs installed version            | —                  | —                             | ✅     |
| `force-update`   | Currently same as switch                        | —                  | —                             | ⚠️     |
| `update-agent`   | Bump flake + rebuild                            | —                  | —                             | 🚧     |
| `force-rebuild`  | Rebuild with cache bypass                       | —                  | —                             | 🚧     |

---

## 10. Compound Flows

### Flow: Do All (`CAC-DO-ALL`)

```
1. pull (all selected) → wait for all complete
2. switch (all that succeeded) → wait for all complete
3. test (all that succeeded) → wait for all complete
```

**Status:** ✅ Working (fixed race condition)

### Flow: Merge & Deploy (`CAC-MERGE-DEPLOY`)

```
1. Merge GitHub PR via API
2. pull (affected hosts)
3. switch (affected hosts)
4. test (affected hosts)
```

**Status:** ✅ Working

### Flow: Full Agent Update (Manual)

```
1. git fetch && git reset --hard
2. nix flake update nixfleet  ← MISSING from NixFleet!
3. nixos-rebuild switch
4. systemctl restart nixfleet-agent
```

**Status:** ⚠️ Step 2 missing — agents get stale versions

---

## 11. Missing / Needed Actions

| ID                  | Action             | Description                                             | Backlog  |
| ------------------- | ------------------ | ------------------------------------------------------- | -------- |
| `CAC-BUMP-AGENT`    | Bump Agent Version | Run `nix flake update nixfleet` + commit/push + rebuild | P7210 🚧 |
| `CAC-FORCE-REBUILD` | Force Rebuild      | Rebuild with `--option narinfo-cache-negative-ttl 0`    | P7220 🚧 |
| `CAC-UPDATE-AGENT`  | Update Agent       | Combined: bump + force rebuild + restart                | — 🚧     |

### The "Stale Agent" Problem

**Current pull+switch flow:**

```
pull:   git reset --hard origin/main  ← Uses REPO's flake.lock
switch: nixos-rebuild switch          ← Builds with REPO's lock
```

**Problem:** `flake.lock` in repo is old → agent stays old.

**Fix:** Need `CAC-BUMP-AGENT` to:

1. Run `nix flake update nixfleet` on one host
2. Commit updated `flake.lock` to repo
3. Push to origin
4. THEN `pull+switch` on all hosts gets new agent

---

## 12. API Endpoints

| Endpoint                              | Method | Purpose                          |
| ------------------------------------- | ------ | -------------------------------- |
| `/api/hosts`                          | GET    | List all hosts                   |
| `/api/hosts`                          | POST   | Add new host                     |
| `/api/hosts/{id}`                     | DELETE | Remove host                      |
| `/api/hosts/{id}/command`             | POST   | Send command to agent            |
| `/api/hosts/{id}/refresh`             | POST   | Refresh host status              |
| `/api/hosts/{id}/refresh-git`         | POST   | Refresh Git status via GitHub    |
| `/api/hosts/{id}/theme-color`         | POST   | Set host color                   |
| `/api/hosts/{id}/reboot`              | POST   | Reboot with TOTP                 |
| `/api/hosts/{id}/logs`                | GET    | Get host logs                    |
| `/api/hosts/{id}/kill`                | POST   | Kill running command             |
| `/api/hosts/{id}/timeout-action`      | POST   | Handle timeout action            |
| `/api/command-states`                 | GET    | Get command state machine status |
| `/api/system-log`                     | GET    | Get system log                   |
| `/api/flake-updates/status`           | GET    | Check for pending PRs            |
| `/api/flake-updates/check`            | POST   | Force check for updates          |
| `/api/flake-updates/merge-and-deploy` | POST   | Merge PR and deploy              |

---

## Summary

**Total CACs:** 45+ defined  
**Working:** ~40 ✅  
**Partial:** 1 ⚠️ (`force-update` is just switch)  
**Missing:** 3 🚧 (`update-agent`, `force-rebuild`, `bump-agent`)

**Critical Gap:** No way to update `flake.lock` via dashboard → agents stay stale.
