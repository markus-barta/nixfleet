# Update Agent from UI ✅

**Completed**: 2025-12-13  
**Original**: 2025-12-12

## Summary

Added "Update Agent" action to the dashboard that updates the nixfleet flake input and rebuilds the host, bringing the agent to the latest version.

## Implementation

### Agent Changes (`agent/nixfleet-agent.sh`)

Added `do_update()` function that:
1. Runs `nix flake update nixfleet`
2. Commits `flake.lock` if changed
3. Pushes to remote (with fallback if push fails)
4. Calls `do_switch()` to rebuild

### Backend Changes (`app/main.py`)

- Added "update" to allowed commands in `CommandRequest` validation
- Added command labels/icons for status display

### Frontend Changes (`app/templates/dashboard.html`)

- Added "Update Agent" button to ellipsis dropdown menu
- Added "Updating agent..." status label with ⬆ icon

## Testing Results

Tested on `imac0` (macOS):
1. ✅ Dashboard shows "Update Agent" button
2. ✅ Clicking button queues "update" command
3. ✅ Agent receives command
4. ✅ Agent updates flake input
5. ✅ Agent commits and pushes flake.lock
6. ✅ Agent runs home-manager switch
7. ✅ Switch completes successfully
8. ✅ Dashboard shows "Command completed"

## Files Modified

- `agent/nixfleet-agent.sh` - Added `do_update()` and command handler
- `app/main.py` - Added "update" to allowed commands
- `app/templates/dashboard.html` - Added UI button and labels

