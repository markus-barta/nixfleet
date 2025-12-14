# Heartbeat & Communication Visualizer

**Created**: 2025-12-14
**Priority**: P6000 (Low)
**Status**: Backlog
**Depends on**: P4000-P4400 (Core rewrite)

---

## Overview

Visual animation showing real-time communication between hosts and dashboard.

---

## Design

```text
[INCOMING]        [CENTER]        [OUTGOING]
· · · · · · · ●    ⬡             ● · · · · · · ·
←←←←←←←←←←←←←←                    →→→→→→→→→→→→→→

15 dots per column, animating based on:
- Incoming: Time since last heartbeat
- Outgoing: Command lifecycle (queued → ack → exec → done)
```

---

## States

**Incoming (Heartbeat):**

- 0-10s: Green ripple
- 10-30s: Dots light up
- 30-90s: Yellow stale
- 90s+: Gray offline

**Outgoing (Commands):**

- Queued: Blue
- Executing: Cyan
- Complete: Green fade
- Error: Red

---

## Implementation

Pure CSS/JS animation, no backend changes needed.

---

## Acceptance Criteria

- [ ] Dot animation for heartbeats
- [ ] Dot animation for commands
- [ ] Mobile: Simplified view
- [ ] Accessible (reduced motion respected)

---

## Related

- Nice-to-have polish
- Depends on WebSocket for real-time state
