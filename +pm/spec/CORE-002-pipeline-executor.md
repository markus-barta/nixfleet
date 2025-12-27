# CORE-002: Pipeline Executor

> **Spec Type**: Core Building Block  
> **Status**: Stable  
> **Last Updated**: 2025-12-27

---

## Purpose

The **Pipeline Executor** orchestrates multi-op sequences with `&&` semantics. It ensures ops execute in order, tracks partial completion, and handles failures gracefully.

---

## Pipeline Definition

A **Pipeline** is an ordered sequence of Op IDs.

```go
type Pipeline struct {
    ID          string   // Unique identifier: "do-all", "merge-deploy"
    Ops         []string // Ordered Op IDs: ["pull", "switch", "test"]
    Description string   // Human-readable description
}
```

### Execution Semantics

Pipelines follow `&&` semantics (like bash):

1. Execute Op 1 on **all** target hosts
2. Wait for all hosts to complete Op 1
3. Hosts that **succeeded** proceed to Op 2
4. Hosts that **failed** are marked `SKIPPED` for remaining ops
5. Continue until all ops complete or all hosts fail

```
Hosts: [hsb0, hsb1, gpc0]
Pipeline: do-all [pull, switch, test]

Stage 1: pull
  hsb0: SUCCESS ────────┐
  hsb1: SUCCESS ────────┼──▶ proceed to switch
  gpc0: ERROR ──────────┴──▶ SKIPPED for switch, test

Stage 2: switch
  hsb0: SUCCESS ────────┐
  hsb1: TIMEOUT ────────┴──▶ SKIPPED for test

Stage 3: test
  hsb0: SUCCESS

Result: PARTIAL (1 of 3 hosts completed all stages)
```

---

## Pipeline Registry

All pipelines are defined at dashboard startup.

| Pipeline ID    | Ops                                   | Description                     |
| -------------- | ------------------------------------- | ------------------------------- |
| `do-all`       | `[pull, switch, test]`                | Full update cycle               |
| `merge-deploy` | `[merge-pr, pull, switch, test]`      | Merge PR then deploy            |
| `update-agent` | `[bump-flake, pull, switch, restart]` | Update agent to latest version  |
| `force-update` | `[force-rebuild, restart]`            | Force rebuild with cache bypass |

---

## Pipeline States

```
┌─────────┐     ┌─────────┐     ┌─────────┐     ┌─────────┐
│  IDLE   │────▶│ STAGE-1 │────▶│ STAGE-2 │────▶│ STAGE-N │────▶ COMPLETE
└─────────┘     │  (pull) │     │ (switch)│     │  (test) │
                └────┬────┘     └────┬────┘     └────┬────┘
                     │               │               │
                on failure      on failure      on failure
                 (some)          (some)          (some)
                     │               │               │
                     ▼               ▼               ▼
                PARTIAL         PARTIAL         PARTIAL
               (remaining      (remaining      (report only)
                hosts          hosts
                continue)      continue)
```

### Status Definitions

| Status      | Description                                 |
| ----------- | ------------------------------------------- |
| `IDLE`      | Not started                                 |
| `STAGE-N`   | Currently executing stage N                 |
| `COMPLETE`  | All hosts completed all stages successfully |
| `PARTIAL`   | Some hosts failed, remainder completed      |
| `FAILED`    | All hosts failed before completing pipeline |
| `CANCELLED` | User cancelled mid-execution                |

---

## Pipeline Record

Pipelines are journaled in the State Store (see CORE-003).

```sql
CREATE TABLE pipelines (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,          -- "do-all"
    hosts         TEXT NOT NULL,          -- JSON array of host IDs
    current_stage INTEGER DEFAULT 0,
    status        TEXT NOT NULL,          -- RUNNING, PARTIAL, COMPLETE, FAILED
    created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    finished_at   TIMESTAMP
);
```

---

## Execution Flow

```go
func (pe *PipelineExecutor) Execute(ctx context.Context, pipeline Pipeline, hosts []Host) error {
    // 1. Create pipeline record
    pipelineID := uuid.New().String()
    pe.store.CreatePipeline(pipelineID, pipeline.ID, hosts)

    activeHosts := hosts

    for stageIdx, opID := range pipeline.Ops {
        // 2. Update stage
        pe.store.UpdatePipelineStage(pipelineID, stageIdx)

        // 3. Execute op on all active hosts (parallel)
        results := pe.executeOpOnHosts(ctx, opID, pipelineID, activeHosts)

        // 4. Filter to successful hosts
        activeHosts = filterSuccessful(results)

        // 5. Check if any hosts remain
        if len(activeHosts) == 0 {
            pe.store.FinishPipeline(pipelineID, "FAILED")
            return fmt.Errorf("all hosts failed at stage %d", stageIdx)
        }
    }

    // 6. Determine final status
    status := "COMPLETE"
    if len(activeHosts) < len(hosts) {
        status = "PARTIAL"
    }
    pe.store.FinishPipeline(pipelineID, status)
    return nil
}
```

---

## Per-Host Op States in Pipeline

Each host's op within a pipeline has its own status:

| Status      | Description                                |
| ----------- | ------------------------------------------ |
| `PENDING`   | Waiting for this stage to start            |
| `EXECUTING` | Currently running this op                  |
| `SUCCESS`   | Op completed successfully, proceed to next |
| `ERROR`     | Op failed, skip remaining stages           |
| `SKIPPED`   | Not executed due to earlier failure        |
| `TIMEOUT`   | Op exceeded timeout                        |

---

## Cancellation

Users can cancel a running pipeline:

1. Running ops continue to completion (no mid-op abort)
2. Pending ops are marked `CANCELLED`
3. Pipeline status becomes `CANCELLED`

---

## Implementation Location

```
src/internal/ops/
├── pipeline.go       # Pipeline struct, registry, and executor
├── executor.go       # Op execution logic
└── ...
```

---

## Implementing Backlog Items

> Updated as backlog items are created/completed.

| Backlog Item | Description                      | Status  |
| ------------ | -------------------------------- | ------- |
| P7210        | Update Agent pipeline            | Backlog |
| P7220        | Force Rebuild pipeline           | Backlog |
| (pending)    | Create Pipeline Executor package | —       |

---

## Changelog

| Date       | Change                          |
| ---------- | ------------------------------- |
| 2025-12-27 | Initial spec extracted from PRD |
