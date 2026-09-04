---
title: Concurrency
description: Use Go goroutines, typed channels, select, and single-consumption structured tasks in Kinmokusei.
---

# Concurrency

Kinmokusei exposes direct Go concurrency and a structured local task form. Choose based on whether the result and lifetime should be tracked.

For the sequential model, read [Concurrency and tasks](../book/concurrency-and-tasks).

## Raw goroutines

```ts
go refreshCache();
```

A semicolon makes this the raw Go statement form. It starts an ordinary goroutine and does not retain a result or enforce a join. Panics behave like unhandled Go goroutine panics.

Use this form only when the lifetime is intentionally unmanaged.

## Channels

```ts
const channel = goChannel[int](1);
channel <- 42;
const [value, open] = <-channel;
closeGoChannel(channel);
```

`GoChannel<T>`, `GoSendChannel<T>`, and `GoReceiveChannel<T>` preserve Go direction rules. Send, receive, checked receive, close, and channel range keep Go blocking, nil, closed-channel, and panic behavior.

## Select

```ts
select {
  case const value = <-input { consume(value); }
  case let [value, open] = <-channel { inspect(value, open); }
  case output <- value { sent(); }
  default { idle(); }
}
```

Operands evaluate with Go's count and order. Every case has block scope, there is no fallthrough, and an empty select blocks forever. When multiple cases are ready, selection is intentionally nondeterministic.

## Structured tasks

A `go` expression retains the result in `Task<T>`:

```ts
const task: Task<User> = go loadUser(id);
const user: User = await task;

const checked: Task<Result<User>> = go loadChecked(id);
const checkedUser: User = await checked?;
```

A task is a non-escaping local capability. It cannot be copied, reassigned, captured, placed in a field or container, made global, or exposed through a function signature. Every task binding must be consumed exactly once with `await` or `detach` on every continuing path.

## Evaluation and failure

The callee and arguments evaluate synchronously, exactly once and in source order, before the worker goroutine starts. This avoids moving side effects into a scheduling race.

A worker panic is contained until `await`, then re-panicked in the awaiting goroutine. `detach` discards the value explicitly; a detached panic is re-raised by its waiter goroutine and remains process-fatal.

`Task<Result<T>>` keeps operation failure distinct from task transport. `await task?` propagates the result error through an enclosing `Result` function.

## Cancellation and shared state

Pass `context.Context` explicitly through direct Go interop when work needs cancellation today. Automatic task-context inheritance and cancellation propagation are planned, not part of the current runtime contract.

Kinmokusei does not promise implicit race prevention. Shared mutable state must use the same synchronization discipline as Go, and generated programs remain compatible with `go test -race`.

## Choosing a form

| Need | Use |
| --- | --- |
| Fire-and-forget Go behavior | `go call();` |
| One typed result with a required join | `Task<T>` plus `await` |
| Explicitly discard a task result | `detach` |
| Stream or coordinate values | Directional channels and `select` |
| Cancellation/deadline | Explicit Go `context.Context` |

## Runnable recipes

- [Run parallel work with tasks](../examples/tasks): eager start and exactly-once joins.
- [Send and receive through a channel](../examples/channels): buffered values, close, drain, and checked receive.
- [Coordinate channels with select](../examples/select): deterministic ready/default send and receive paths.
