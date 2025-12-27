# P3020: Pipeline Executor

**Priority**: P3020 (Critical - v3 Phase 1)  
**Status**: Backlog  
**Effort**: Medium (1-2 days)  
**Implements**: [CORE-002](../spec/CORE-002-pipeline-executor.md)  
**Depends on**: P3010 (Op Engine Foundation)

---

## User Story

**As a** user  
**I want** to run multi-step workflows (pull → switch → test)  
**So that** failed hosts are automatically excluded from subsequent steps

---

## Scope

### Define Pipeline Types

```go
type Pipeline struct {
    ID          string
    Ops         []string  // Op IDs in order
    Description string
}

type PipelineExecutor struct {
    registry *OpRegistry
    store    *StateStore
}

func (pe *PipelineExecutor) Execute(ctx context.Context, pipeline Pipeline, hosts []Host) error
```

### Register Built-in Pipelines

| Pipeline ID    | Ops                                   | Description                     |
| -------------- | ------------------------------------- | ------------------------------- |
| `do-all`       | `[pull, switch, test]`                | Full update cycle               |
| `merge-deploy` | `[merge-pr, pull, switch, test]`      | Merge PR then deploy            |
| `update-agent` | `[bump-flake, pull, switch, restart]` | Update agent to latest version  |
| `force-update` | `[force-rebuild, restart]`            | Force rebuild with cache bypass |

### Implement && Semantics

```go
func (pe *PipelineExecutor) Execute(ctx context.Context, pipeline Pipeline, hosts []Host) error {
    activeHosts := hosts

    for _, opID := range pipeline.Ops {
        // Execute op on all active hosts (parallel)
        results := pe.executeOpOnHosts(ctx, opID, activeHosts)

        // Filter to successful hosts only
        activeHosts = filterSuccessful(results)

        if len(activeHosts) == 0 {
            return ErrAllHostsFailed
        }
    }

    return nil
}
```

### Unit Tests

- Pipeline registration
- && semantics (failed hosts excluded)
- Partial completion tracking
- Cancellation mid-pipeline

---

## Acceptance Criteria

- [ ] Pipeline struct matches CORE-002 spec
- [ ] Built-in pipelines registered at startup
- [ ] && semantics work (failed hosts excluded)
- [ ] Pipeline status tracked (RUNNING, PARTIAL, COMPLETE, FAILED)
- [ ] Unit tests for pipeline execution
- [ ] Existing "Do All" button uses `do-all` pipeline

---

## Related

- **CORE-002**: Pipeline Executor spec
- **P3010**: Op Engine Foundation (prerequisite)
- **P7210**: Dashboard Bump Agent Version (uses `update-agent` pipeline)
- **P7220**: Dashboard Force Rebuild (uses `force-update` pipeline)
