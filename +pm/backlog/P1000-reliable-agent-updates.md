# P1000 - Reliable Agent Updates

**Created**: 2025-12-19  
**Updated**: 2025-12-19  
**Priority**: P1000 (Critical - Blocking)  
**Status**: Root Cause Confirmed - UX Issue  
**Estimated Effort**: 2-3 hours  
**Depends on**: None

---

## Executive Summary

~~The agent update flow is broken on macOS.~~ **CORRECTION**: The agent update flow works correctly! The issue is **UX** — users click "Pull" but don't click "Switch", thinking the update is complete.

**Verified**: Running `home-manager switch` on macOS correctly restarts the agent with the new binary via launchd's bootout/bootstrap mechanism.

---

## Root Cause (Confirmed via Manual Testing)

### What We Tested (2025-12-19 on imac0)

1. **Before switch**: Agent running 2.0.0
2. **After `home-manager switch`**: Agent running 2.1.0 ✅

```
1:50PM INF received signal signal=terminated
1:50PM INF shutting down
1:50PM INF NixFleet Agent starting ... version=2.1.0
```

home-manager's `setupLaunchAgents` correctly:

- Sends SIGTERM to old agent
- Runs `launchctl bootout`
- Runs `launchctl bootstrap` with new plist
- New agent starts with new binary

### The ACTUAL Problem

Timeline on imac0:

| Event                    | Date/Time     | Result                          |
| ------------------------ | ------------- | ------------------------------- |
| Gen 121 created (switch) | Dec 17, 19:24 | Agent 2.0.0 installed           |
| Commit with 2.1.0 pushed | Dec 18, 14:07 | flake.lock updated in repo      |
| Pull via dashboard       | Dec 19, 10:58 | Agent fetched new flake.lock    |
| **Switch NOT triggered** | -             | Gen 121 still active!           |
| Manual switch            | Dec 19, 13:49 | Gen 122 created, Agent 2.1.0 ✅ |

**The bug**: Users clicked "Pull" and assumed they were done. They didn't click "Switch".

---

## The UX Problem

Current UI:

```
[Pull]  [Switch]  [Test]   ← Three separate buttons
```

User mental model:

- "I clicked Pull, my host is updated!" ❌
- Reality: Pull just fetches code, Switch applies it

### Evidence

The UI has no combined action:

```go
// dashboard.templ lines 1279-1280
@CommandButton(host.ID, "pull", "Pull", "btn", ...)
@CommandButton(host.ID, "switch", "Switch", "btn", ...)  // Separate button!
```

But the agent DOES support `pull-switch`:

```go
// commands.go line 100
case "pull-switch":
    // Pull first, then switch
```

This command exists but isn't exposed in the UI!

---

## Solution: Add "Update" Button

### Option A: Add "Update" Button (Recommended)

Add a combined button that runs `pull-switch`:

```
[Update ▾]  [Test]   ← Single button with optional dropdown
   └── Pull only
   └── Switch only
```

Or simpler:

```
[Update]  [Pull]  [Switch]  [Test]   ← "Update" does both
```

### Option B: Auto-Switch After Pull

When Pull completes successfully, automatically trigger Switch.

**Risk**: User might want to review changes before applying. Less control.

### Option C: Better Visual Feedback

After Pull succeeds, show prominent message:

```
✓ Pull complete. Click [Switch] to apply changes.
```

### Recommendation

**Option A** — Add "Update" button that runs `pull-switch`. Keep Pull/Switch for advanced users who want granular control.

---

## Implementation

### Step 1: Add "Update" button to UI (1 hour)

File: `v2/internal/templates/dashboard.templ`

```go
// In host card actions section
@CommandButton(host.ID, "pull-switch", "Update", "btn btn-primary", host.Online && host.PendingCommand == "")
@CommandButton(host.ID, "pull", "Pull", "btn btn-secondary", host.Online && host.PendingCommand == "")
@CommandButton(host.ID, "switch", "Switch", "btn btn-secondary", host.Online && host.PendingCommand == "")
```

### Step 2: Update "Pull All" / "Switch All" (30 min)

Add "Update All" button in header:

```go
<button class="btn btn-primary" @click="sendBroadcast('pull-switch')">
    <svg class="icon"><use href="#icon-refresh"></use></svg>
    Update All
</button>
```

### Step 3: Fix button styling (30 min)

Make "Update" visually prominent, Pull/Switch secondary.

---

## What We DON'T Need to Fix

Based on testing, these are NOT broken:

- ~~launchd restart~~ → Works correctly
- ~~launchctl kickstart~~ → Not needed
- ~~Agent self-restart on macOS~~ → Not needed
- ~~Activation hooks~~ → Not needed

The existing code in `modules/home-manager.nix` is correct:

```nix
# NOTE: No custom activation hook needed - home-manager's setupLaunchAgents
# already handles agent lifecycle (bootout → bootstrap) correctly.
```

This comment is ACCURATE. We just need better UX.

---

## Acceptance Criteria

- [ ] "Update" button visible in host card (runs pull-switch)
- [ ] "Update All" button in header (runs pull-switch on all hosts)
- [ ] Pull/Switch buttons still available for granular control
- [ ] After Update, agent version updates within 60 seconds

---

## Testing

### Verify the fix works:

1. Update flake.lock in nixcfg with new agent version
2. Push to GitHub
3. In dashboard, click "Update" on imac0
4. Verify: Pull runs, Switch runs, Agent restarts with new version

### Regression test:

1. Click "Pull" only → Agent version should NOT change
2. Click "Switch" only → Agent should update if code was already pulled
3. Click "Update" → Agent should update in one action

---

## Files to Modify

| File                                    | Change                                |
| --------------------------------------- | ------------------------------------- |
| `v2/internal/templates/dashboard.templ` | Add "Update" and "Update All" buttons |
| `v2/internal/templates/styles.css`      | Style primary vs secondary buttons    |

---

## Why Previous Analysis Was Wrong

The P1000 code analysis looked at `commands.go:146`:

```go
if ... && runtime.GOOS != "darwin" {
    os.Exit(101)  // Only for NixOS
}
```

And concluded "macOS doesn't restart". But this code is for the AGENT to restart ITSELF after switch. It's not needed on macOS because **home-manager already handles the restart**.

The confusion: We thought switch was being run and failing. Actually, **switch wasn't being run at all** — only pull was.

---

## Related

- [UPDATE-ARCHITECTURE.md](../../docs/UPDATE-ARCHITECTURE.md) — Documents the 5-step update flow
- The `pull-switch` command already exists in agent code, just not exposed in UI
