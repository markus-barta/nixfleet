# Show NixOS Generation in Dashboard

**Created**: 2025-12-13  
**Priority**: MEDIUM  
**Status**: Backlog

---

## Problem

Currently, the dashboard doesn't show the NixOS generation number for each host. This makes it hard to:

- See if a host is up-to-date with the latest configuration
- Identify which hosts need a switch after a git push
- Compare generations across hosts to verify deployment consistency

---

## Requirements

### 1. Agent Reports Generation

- [ ] Agent sends current generation number in heartbeat
- [ ] Include generation timestamp if available
- [ ] Include nixpkgs commit/version for context

### 2. Dashboard Displays Generation

- [ ] Show generation number in host list/table
- [ ] Show generation timestamp (when it was activated)
- [ ] Visual indicator for outdated hosts (different generation than others)

### 3. Compare Across Hosts

- [ ] Highlight hosts with different generations
- [ ] Optional: Show "latest" generation based on most common or newest
- [ ] Optional: Group by generation for easy identification

---

## Implementation

### Agent Changes

The agent already collects some of this. Enhance heartbeat payload:

```bash
# Get current generation info
GENERATION=$(readlink /nix/var/nix/profiles/system | grep -oP 'system-\K\d+')
GEN_TIME=$(stat -c %Y /nix/var/nix/profiles/system)
NIXPKGS_REV=$(nixos-version --revision 2>/dev/null || echo "unknown")

# In heartbeat JSON
{
  "hostname": "hsb0",
  "generation": 42,
  "generation_time": "2025-12-13T18:03:09Z",
  "nixpkgs_rev": "2fad6eac6077",
  ...
}
```

### Dashboard Changes

**API:** Already receives generation in status updates, ensure it's stored and returned.

**UI:** Add column to host table:

| Host | Status   | Generation   | Nixpkgs | Last Seen |
| ---- | -------- | ------------ | ------- | --------- |
| hsb0 | ✅ ok    | 42 (2h ago)  | 2fad6ea | 10s ago   |
| hsb1 | ✅ ok    | 42 (2h ago)  | 2fad6ea | 10s ago   |
| gpc0 | ⚠️ stale | 30 (13d ago) | 2fad6ea | 2h ago    |

**Visual indicators:**

- 🟢 Same generation as majority of hosts
- 🟡 Older generation than others
- 🔴 Very old generation (>7 days)

---

## Data Model

Extend host data in dashboard:

```python
# In app/main.py host state
host_data = {
    "hostname": "hsb0",
    "status": "ok",
    "generation": 42,
    "generation_time": datetime(...),
    "nixpkgs_rev": "2fad6eac6077",
    "os_version": "NixOS 26.05.20251127.2fad6ea (Yarara)",
    ...
}
```

---

## Acceptance Criteria

- [ ] Agent sends generation number in every heartbeat
- [ ] Dashboard shows generation column in host list
- [ ] Generation age shown (e.g., "2h ago", "13d ago")
- [ ] Outdated hosts visually highlighted
- [ ] Works for NixOS hosts; macOS shows "N/A" or home-manager generation

---

## Related

- Agent script: `agent/nixfleet-agent.sh` (heartbeat function)
- Dashboard: `app/main.py`, `app/templates/dashboard.html`
- Existing generation tracking in agent (partial implementation)

---

## Notes

- macOS doesn't have NixOS generations; could show home-manager generation instead
- Consider adding git commit hash of nixcfg repo for more precise tracking
- Future: Add "Deploy to all" button that triggers switch on outdated hosts
