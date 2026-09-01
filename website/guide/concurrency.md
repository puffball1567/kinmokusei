# Concurrency

Direct Go concurrency and structured tasks are both supported. Use the lowest-level form only when its lifetime is intentionally unmanaged.

## Raw goroutines and channels

`go call();` starts an ordinary goroutine. Directional channels, send/receive, checked receive, `close`, channel range, and `select` preserve Go behavior.

## Structured tasks

A `go` expression creates `Task<T>`. Every non-escaping task must be consumed exactly once with `await` or `detach`.

```ts
const task = go loadUser(id);
const user = await task;
```

Function and argument expressions are evaluated eagerly before the task starts. Panics are transported across the task boundary and re-raised by `await`; `Result` failures remain explicit results.

Pass `context.Context` through direct Go interop when work needs cancellation today. Automatic task-context inheritance remains outside the current release contract.
