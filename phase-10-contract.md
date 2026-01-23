# Phase 10 Contract: Compliance Fixes (Error Returns + Idempotency)

**Created:** 2026-01-23

## Step 1 Checklist
- [x] 1a: Presented understanding
- [x] 1b: Asked clarifying questions (plan graded, improved)
- [x] 1b-answer: User approved plan
- [x] 1c: Contract created (this file)
- [ ] 1d: Approval received
- [ ] 1e: Plan + contract archived

## Objective

Fix two compliance issues identified during Phase 9 grading:
1. `emit()` panics on store conflict instead of returning error for retry (ES Guide §11.443)
2. Projection lost idempotency when map was removed for value semantics

User direction: "This is a reference implementation that can transition to distributed message queue for scaling. Always choose the general and correct solution, especially correct by construction."

## Success Criteria

- [ ] `emit()` returns `(Engine, error)` on store conflict
- [ ] Projection uses sorted slice for idempotency
- [ ] Idempotent Apply returns unchanged projection (no version bump)
- [ ] All Engine methods propagate errors correctly
- [ ] Handlers implement retry (3 attempts, 409 on exhaustion)
- [ ] All tests pass with `-race`
- [ ] Stress tests: no panics, proper retry, < 5% conflict rate

## Approach (TDD)

### Sub-phase A: Projection Idempotency

1. **Red**: Add test `TestProjection_Apply_IdempotentWithSortedSlice`
2. **Green**: Implement sorted slice in Projection
3. **Verify**: `go test ./internal/events/...`

### Sub-phase B: emit() Error Return

1. **Red**: Update emit() signature, watch compile errors
2. **Green**: Fix each method in dependency order
3. **Verify**: `go build ./internal/engine`

### Sub-phase C: Update Callers

1. **Registry**: Update `CreateSimulation` to handle errors
2. **Handlers**: Add retry logic (3 attempts, 409 on exhaustion)
3. **Tests**: Update all test files for new signatures

### Sub-phase D: Verify

```bash
go test -race ./internal/engine/...
go test -race ./internal/events/...
go test -race ./internal/api/ -run Concurrent -v
```

## Files to Modify

| File | Changes |
|------|---------|
| `internal/engine/engine.go` | emit() returns error, cascade to all callers |
| `internal/events/projection.go` | Add sorted slice idempotency |
| `internal/events/projection_test.go` | Restore idempotency tests |
| `internal/engine/*_test.go` | Update for new signatures |
| `internal/api/handlers.go` | Handle errors with retry logic |
| `internal/registry/registry.go` | Update CreateSimulation for errors |

## Token Budget

Estimated: 20-30K tokens
