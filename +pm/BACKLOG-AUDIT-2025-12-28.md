# NixFleet Backlog Audit - 2025-12-28

**Context**: After defining the 5-compartment system (P5000 epic), reviewing all backlog items for relevance, conflicts, and priority.

---

## Summary

| Total Items | Keep As-Is | Update | Re-prioritize | Cancel | New (P5xxx) |
| ----------- | ---------- | ------ | ------------- | ------ | ----------- |
| 22          | TBD        | TBD    | TBD           | TBD    | 7           |

---

## Item-by-Item Analysis

### P3300 - Logs on Page Load ⭐ HIGH PRIORITY

**Status**: Backlog  
**Current Priority**: P3300 (High - v3 Phase 4)  
**Effort**: 1-2 days

**What it does:**

- Restore system logs on page refresh
- Populate host output tabs from server history
- Include recent events in init payload

**Analysis:**

- ✅ **Relevant**: Critical for v3 State Sync
- ✅ **No conflicts** with P5000 compartment system
- ✅ **Complements** compartment work (logs explain why compartments changed)
- ⚠️ **Depends on**: CORE-003 (State Store), CORE-004 (State Sync) - already done

**Recommendation**: ✅ **KEEP - HIGH PRIORITY**  
**Action**: Implement after P5200-P5400 (compartment core)  
**Notes**: Logs will show compartment state changes, very valuable

---

### P3400 - Frontend Simplification

**Status**: Backlog  
**Current Priority**: P3400 (Medium)  
**Effort**: 2-3 days

**What it does:**

- Remove business logic from frontend
- Make UI a thin dispatcher
- Consolidate duplicate action handlers

**Analysis:**

- ✅ **Relevant**: Part of v3 "Thin Frontend" principle
- ✅ **No conflicts** with P5000
- ✅ **Complements** compartment work (cleaner UI code)
- ⚠️ **Some overlap**: Compartment work will simplify UI naturally

**Recommendation**: ✅ **KEEP - MEDIUM PRIORITY**  
**Action**: Do AFTER P5000 (compartment changes will reduce scope)  
**Notes**: Some simplification happens naturally with new compartments

---

### P4200 - Declarative Secrets

**Status**: Backlog  
**Current Priority**: P4200 (Medium)  
**Effort**: 3-4 days

**What it does:**

- Store secrets in nixcfg repository (encrypted)
- Inject secrets into host configs
- UI for secret management

**Analysis:**

- ✅ **Relevant**: Valuable feature, orthogonal to compartments
- ✅ **No conflicts** with P5000
- ⚠️ **Low urgency**: Nice-to-have, not blocking anything

**Recommendation**: ✅ **KEEP - LOW PRIORITY**  
**Action**: Move to P7xxx range (future enhancements)  
**Notes**: Good idea, but not critical path

---

### P4301 - Flake Updates E2E Tests

**Status**: Backlog  
**Current Priority**: P4301 (Medium)  
**Effort**: 2-3 days

**What it does:**

- Comprehensive E2E tests for flake update workflow
- Mock GitHub API, test PR detection/merge
- Test deployment flow

**Analysis:**

- ✅ **Relevant**: Testing is important
- ⚠️ **Conflicts**: Workflow is changing (P5700 - Merge PR)
- ⚠️ **Needs update**: Current spec assumes old "Merge & Deploy" flow

**Recommendation**: ⚠️ **UPDATE REQUIRED**  
**Action**: Update after P5700 (new Merge PR workflow)  
**Notes**: Tests should reflect new manual deployment strategy

---

### P4310 - Flake Update Rollback

**Status**: Backlog  
**Current Priority**: P4310 (Medium)  
**Effort**: 2-3 hours

**What it does:**

- Rollback after failed flake update
- Revert PR on GitHub
- Rollback hosts to previous generation

**Analysis:**

- ❌ **SUPERSEDED**: Completely replaced by P5600 (Rollback Operations)
- P5600 is more comprehensive (per-host + fleet-wide)
- P5600 includes generation visibility

**Recommendation**: ❌ **CANCEL - SUPERSEDED BY P5600**  
**Action**: Move to cancelled, reference P5600  
**Notes**: P5600 is the better design

---

### P5000-P5800 - Compartment System (NEW)

**Status**: Backlog (newly created)  
**Current Priority**: P5000 (Epic - High)  
**Total Effort**: 18-24 hours

**What it does:**

- P5000: Epic overview
- P5200: Lock version tracking
- P5300: System inference
- P5400: Tests compartment
- P5500: Generation tracking
- P5600: Rollback operations
- P5700: Merge PR workflow
- P5800: Documentation

**Analysis:**

- ✅ **High value**: Fixes critical UX issues
- ✅ **Well-scoped**: Clear dependencies
- ✅ **No conflicts**: Orthogonal to other work

**Recommendation**: ✅ **KEEP - TOP PRIORITY**  
**Action**: Start with P5200, P5300, P5400 (must-haves)  
**Notes**: This is the main work for next sprint

---

### P6000 - Heartbeat Visualizer

**Status**: Backlog  
**Current Priority**: P6000 (Low)  
**Effort**: 1-2 days

**What it does:**

- Visual indicator of heartbeat activity
- Pulse animation on successful heartbeat
- Alert on missed heartbeats

**Analysis:**

- ✅ **Relevant**: Nice UX enhancement
- ✅ **No conflicts** with P5000
- ⚠️ **Low urgency**: Cosmetic, not critical

**Recommendation**: ✅ **KEEP - LOW PRIORITY**  
**Action**: P7xxx or later (polish phase)  
**Notes**: Good idea for future

---

### P6200 - Security Hardening

**Status**: Backlog  
**Current Priority**: P6200 (Medium)  
**Effort**: 3-5 days

**What it does:**

- Rate limiting
- Input validation
- HTTPS enforcement
- Security headers

**Analysis:**

- ✅ **Relevant**: Important for production
- ✅ **No conflicts** with P5000
- ⚠️ **Can wait**: Not blocking features

**Recommendation**: ✅ **KEEP - MEDIUM PRIORITY**  
**Action**: After P5000 (before production deployment)  
**Notes**: Do before any public exposure

---

### P6300 - Per-Host Security Tokens

**Status**: Backlog  
**Current Priority**: P6300 (Medium)  
**Effort**: 2-3 days

**What it does:**

- Unique token per host (not shared)
- Token rotation
- Revocation

**Analysis:**

- ✅ **Relevant**: Good security practice
- ✅ **No conflicts** with P5000
- ⚠️ **Can wait**: Current auth is acceptable for personal fleet

**Recommendation**: ✅ **KEEP - LOW PRIORITY**  
**Action**: P7xxx (future enhancement)  
**Notes**: Nice-to-have, not urgent

---

### P6400 - Settings Page

**Status**: Backlog  
**Current Priority**: P6400 (Low)  
**Effort**: 2-3 days

**What it does:**

- UI for configuration (currently env vars)
- GitHub token management
- Heartbeat interval, timeout settings
- Test configuration per-host

**Analysis:**

- ✅ **Relevant**: Needed for test auto-run config (P5400)
- ⚠️ **Partially needed**: Some settings needed for P5400
- ⚠️ **Scope creep**: Full settings page is large

**Recommendation**: ⚠️ **SPLIT INTO TWO**  
**Action**:

- **P6400a** (High): Minimal settings for P5400 (test auto-run toggle)
- **P6400b** (Low): Full settings UI (future)

**Notes**: We need basic settings for P5400, but not the full UI

---

### P6700 - Queue Offline Commands

**Status**: Backlog  
**Current Priority**: P6700 (Low)  
**Effort**: 2-3 days

**What it does:**

- Queue commands when host is offline
- Execute when host reconnects
- Show queued commands in UI

**Analysis:**

- ✅ **Relevant**: Useful feature
- ✅ **No conflicts** with P5000
- ⚠️ **Low urgency**: Edge case (hosts are usually online)

**Recommendation**: ✅ **KEEP - LOW PRIORITY**  
**Action**: P8xxx (nice-to-have features)  
**Notes**: Good idea, but not critical

---

### P6800 - Mobile Card View

**Status**: Backlog  
**Current Priority**: P6800 (Low)  
**Effort**: 2-3 days

**What it does:**

- Responsive mobile layout
- Card-based host view
- Touch-friendly controls

**Analysis:**

- ✅ **Relevant**: Mobile UX is useful
- ⚠️ **Low urgency**: Desktop is primary use case
- ⚠️ **Large scope**: Requires significant UI work

**Recommendation**: ✅ **KEEP - VERY LOW PRIORITY**  
**Action**: P9xxx (future, after v3 stabilizes)  
**Notes**: Desktop works on mobile (just not optimized)

---

### P7220 - Dashboard Force Uncached Rebuild

**Status**: Backlog  
**Current Priority**: P7220 (Low)  
**Effort**: 1-2 hours

**What it does:**

- UI button to force uncached rebuild on dashboard
- Useful for testing/debugging

**Analysis:**

- ✅ **Relevant**: Development tool
- ✅ **No conflicts** with P5000
- ⚠️ **Very niche**: Rarely needed

**Recommendation**: ✅ **KEEP - VERY LOW PRIORITY**  
**Action**: P9xxx or cancel  
**Notes**: Workaround exists (docker rebuild)

---

### P8500 - Accessibility Audit

**Status**: Backlog  
**Current Priority**: P8500 (Low)  
**Effort**: 2-3 days

**What it does:**

- WCAG compliance check
- Keyboard navigation
- Screen reader support
- Color contrast

**Analysis:**

- ✅ **Relevant**: Good practice
- ✅ **No conflicts** with P5000
- ⚠️ **Low urgency**: Personal tool (not public product)

**Recommendation**: ✅ **KEEP - LOW PRIORITY**  
**Action**: After v3 stabilizes (P9xxx)  
**Notes**: Good to do eventually, but not urgent

---

### P8600 - Devenv Release Pin Update

**Status**: Backlog  
**Current Priority**: P8600 (Low)  
**Effort**: 1 hour

**What it does:**

- Update devenv.yaml to use stable release pin
- Document pinning strategy

**Analysis:**

- ✅ **Relevant**: Development environment stability
- ✅ **No conflicts** with P5000
- ✅ **Quick win**: 1 hour effort

**Recommendation**: ✅ **KEEP - DO SOON**  
**Action**: Quick task, do anytime  
**Notes**: Easy maintenance task

---

### P4400 - Nix Darwin Support (file: P9400-nix-darwin-support.md)

**Status**: Backlog  
**Current Priority**: P4400 (Medium)  
**Effort**: 1-2 days

**What it does:**

- Support nix-darwin (system-level macOS) alongside home-manager
- Detect which tool is in use
- Use correct commands (`darwin-rebuild` vs `home-manager`)
- Use correct paths for system checks

**Analysis:**

- ✅ **Relevant**: Useful for system-level macOS management
- ✅ **Integrates with P5300**: System inference needs to handle both
- ⚠️ **File misnamed**: Should be P4400, not P9400
- ⚠️ **Medium urgency**: Only needed if user has nix-darwin Macs

**Recommendation**: ✅ **KEEP - MEDIUM PRIORITY**  
**Action**:

1. Rename file to P4400 (correct priority number)
2. Integrate with P5300 (System compartment)
3. Implement after P5300 if nix-darwin Macs exist

**Notes**: Good addition, especially if you use nix-darwin. System inference will need both paths.

---

## Priority Matrix (After Audit)

### 🔴 Critical Path (Do First)

| Priority | Item  | Effort | Reason                         |
| -------- | ----- | ------ | ------------------------------ |
| 1        | P5200 | 3-4h   | Lock tracking foundation       |
| 2        | P5300 | 2-3h   | System inference               |
| 3        | P5400 | 4-5h   | Tests compartment              |
| 4        | P3300 | 1-2d   | Logs (completes v3 State Sync) |

**Total**: ~2-3 days

---

### 🟡 High Priority (Do Next)

| Priority | Item  | Effort | Reason                  |
| -------- | ----- | ------ | ----------------------- |
| 5        | P5500 | 2-3h   | Generation tracking     |
| 6        | P5600 | 3-4h   | Rollback operations     |
| 7        | P5700 | 2-3h   | Merge PR workflow       |
| 8        | P5800 | 1-2h   | Documentation           |
| 9        | P3400 | 2-3d   | Frontend simplification |

**Total**: ~3-4 days

---

### 🟢 Medium Priority (After v3 Core)

| Priority | Item   | Effort | Reason                         |
| -------- | ------ | ------ | ------------------------------ |
| 10       | P6200  | 3-5d   | Security hardening             |
| 11       | P4301  | 2-3d   | E2E tests (update after P5700) |
| 12       | P6400a | 1d     | Minimal settings UI            |
| 13       | P4400  | 1-2d   | nix-darwin support (if needed) |

**Total**: ~6-9 days

---

### ⚪ Low Priority (Future / Polish)

| Item   | Effort | Notes                  |
| ------ | ------ | ---------------------- |
| P4200  | 3-4d   | Declarative secrets    |
| P6000  | 1-2d   | Heartbeat visualizer   |
| P6300  | 2-3d   | Per-host tokens        |
| P6400b | 2-3d   | Full settings UI       |
| P6700  | 2-3d   | Queue offline commands |
| P6800  | 2-3d   | Mobile view            |
| P7220  | 1-2h   | Force rebuild button   |
| P8500  | 2-3d   | Accessibility audit    |

---

### ✅ Quick Wins (Do Anytime)

| Item  | Effort | Notes             |
| ----- | ------ | ----------------- |
| P8600 | 1h     | Devenv pin update |

---

### ❌ Cancel / Superseded

| Item  | Reason              |
| ----- | ------------------- |
| P4310 | Superseded by P5600 |

---

## Recommended Action Plan

### Sprint 1: Compartment Core (1 week)

1. P5200 - Lock tracking (3-4h)
2. P5300 - System inference (2-3h)
3. P5400 - Tests compartment (4-5h)
4. P3300 - Logs on page load (1-2d)

### Sprint 2: Compartment Features (1 week)

5. P5500 - Generation tracking (2-3h)
6. P5600 - Rollback operations (3-4h)
7. P5700 - Merge PR workflow (2-3h)
8. P5800 - Documentation (1-2h)
9. P3400 - Frontend simplification (2-3d)

### Sprint 3: Polish & Security (1-2 weeks)

10. P6200 - Security hardening (3-5d)
11. P4301 - E2E tests (2-3d)
12. P6400a - Minimal settings (1d)
13. P8600 - Devenv pin (1h)

### Backlog (Future)

- All other items

---

## Files to Update

- [ ] Move P4310 to `+pm/cancelled/`
- [ ] Rename P9400-nix-darwin-support.md to P4400-nix-darwin-support.md
- [ ] Update P4301 (after P5700 implemented)
- [ ] Split P6400 into P6400a and P6400b
- [ ] Update PRD with new priorities
- [ ] Create sprint plan based on recommendations

---

## Next Steps

1. User reviews this audit
2. User confirms/adjusts priorities
3. Move items as needed
4. Start with P5200 (Lock tracking)
