# Pickup: macOS Agent Switch Fix

**Date**: 2025-12-15  
**Context**: Fixed macOS switch command to survive agent restart via `Setsid`

---

## ✅ Completed This Session

1. **Setsid Fix**: macOS switch command now runs in a new session, survives when `launchctl bootout` kills the agent
2. **mba-mbp-work Cleanup**: v1 agent completely removed, garbage collected 2.5GB
3. **Location Fix**: `mba-mbp-work` location corrected from "home" → "work"
4. **Badge Jump Fix**: Status indicator layout fixed (no more jumping when command starts)

---

## 🔴 Open Points (Need Your Decision)

### 1. Pull Command Doesn't Update `flake.lock`

**Issue**: The `pull` command only does `git pull`. It does NOT run `nix flake update`. This caused the isolated repo to install an outdated v1 agent because its `flake.lock` pointed to an old nixfleet commit.

**Current Workflow** (requires manual step):

```
Pull → Update → Switch
```

**Options**:

- [ ] **A) Keep current behavior** - Users must remember to click "Update" before "Switch" when dependencies changed
- [ ] **B) Modify "Pull" to also run `nix flake update nixfleet`** - Auto-updates the agent dependency
- [ ] **C) Create "Pull-Update-Switch" combo command** - One-click full deployment
- [ ] **D) Add "Update" button to UI** - Currently no button, only CLI command exists

**Your preference?**

---

### 2. End-to-End Test of Setsid Fix

**Status**: Fix is deployed, but we haven't done a clean test where:

1. Dashboard sends Switch command
2. Agent starts switch in new session
3. Agent gets killed by `launchctl bootout`
4. Switch continues and completes
5. New agent auto-starts and reconnects
6. Dashboard shows online again

**Question**: Want me to run this test on `mba-mbp-work` now, or defer?

---

### 3. Other macOS Clients

**`mba-imac-work`** may have similar issues:

- [ ] Outdated `flake.lock` in isolated repo?
- [ ] Correct location setting? (currently shows "work" which seems correct)
- [ ] v1 agent remnants?

**Question**: Should I audit and fix `mba-imac-work` as well?

---

### 4. Backlog Item for Pull+Update

Should I create a backlog item for improving the Pull → Update workflow?

- [ ] Yes, create P-level backlog item
- [ ] No, current workflow is acceptable

---

## 📋 Related Commits

**nixfleet**:

- `ace61cb` fix(agent): macOS switch survives agent restart via Setsid

**nixcfg**:

- `b436a545` fix(mba-mbp-work): correct location to 'work'
- `999b43c9` flake: Update nixfleet to ace61cb

---

## 🔗 Related Files

- Agent commands: `nixfleet/v2/internal/agent/commands.go`
- Home Manager module: `nixfleet/modules/home-manager.nix`
- mba-mbp-work config: `nixcfg/hosts/mba-mbp-work/home.nix`
