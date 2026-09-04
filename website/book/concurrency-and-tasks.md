---
title: Concurrency and tasks
description: Understand raw goroutines, structured tasks, channels, select, panic transport, cancellation, and shared-state discipline.
---

# Concurrency and tasks

Kinmokusei exposes Go's low-level concurrency and adds a local structured task capability for computations whose result must be consumed.

## Raw goroutine statement

```ts
go refreshCache();
```

The trailing semicolon makes this the raw statement form. It starts an ordinary Go goroutine, stores no result, enforces no join, and leaves panic/lifetime behavior unmanaged.

Use it only when the application deliberately owns the goroutine lifetime by another protocol.

## Structured task expression

```ts
const task: Task<User> = go loadUser(id);
const user: User = await task;
```

Without statement termination at the call, `go call()` is an expression producing `Task<T>`. The task binding must be consumed exactly once by `await` or `detach` on every continuing path.

The compiler rejects copying, reassigning, capturing, storing, escaping, returning, or multiply consuming a task.

Leaving a task unconsumed is a source error:

<<< ../snippets-invalid/task-unconsumed.km{ts}

## Eager evaluation and start

```ts
const first = go calculate(firstInput());
const second = go calculate(secondInput());
const left = await first;
const right = await second;
```

For each task, the call target and arguments evaluate synchronously once in source order before its worker goroutine starts. Both workers are started before the first await, allowing overlap.

## Awaiting failure

A worker panic is retained by the task and re-panicked in the goroutine that executes `await`. This keeps the failure attached to the required join point.

For an operation result:

```ts
const task: Task<Result<User>> = go loadChecked(id);
const user: User = await task?;
```

`await` first joins and transports panic; `?` then propagates the ordinary operation error through the enclosing `Result` function.

Without `?`, the `Result` layer has not been consumed:

<<< ../snippets-invalid/task-result-without-propagation.km{ts}

## Detach

```ts
const task = go refresh();
detach task;
```

Detach is an explicit consumption that discards the result. A detached panic is re-raised by its waiter goroutine and remains process-fatal. Detach does not mean “ignore all failure safely.”

## Channels

```ts
const messages = goChannel[string](1);
messages <- "ready";
const [message, open] = <-messages;
closeGoChannel(messages);
```

Channel types preserve direction:

- `GoChannel<T>` sends and receives;
- `GoSendChannel<T>` sends only;
- `GoReceiveChannel<T>` receives only.

Send/receive blocking, nil channels, closed-channel zero values, double close, and send-after-close follow Go behavior. Checked receive distinguishes a delivered zero value from a closed/drained channel.

## Range over a channel

```ts
for (const value of input) {
  consume(value);
}
```

Channel range accepts exactly one binding and continues until the channel is closed and drained. A send-only channel cannot be ranged.

## Select

```ts
select {
  case const value = <-input { consume(value); }
  case output <- pending { markSent(); }
  default { idle(); }
}
```

Select chooses a ready communication. Multiple ready cases are intentionally nondeterministic. Default makes the operation non-blocking; no default waits. Each case has its own scope.

## Cancellation and deadlines

Automatic task-context inheritance is not implemented. Pass `context.Context` explicitly:

```ts
function load(ctx: context.Context): Result<User> {
  // pass ctx into Go HTTP/database operations
}
```

Select on `ctx.Done()` when coordinating channels directly. The caller owns cancellation creation and must call its cancel function according to the Go API contract.

## Shared mutable state

Tasks and goroutines do not make shared state race-free. Classes, slices, maps, pointers, and captured variables may still be shared across workers. Use channels or Go synchronization primitives and verify concurrent programs with the race detector where applicable.

Use the [Go interoperability guide](../guide/go-interop) when concurrency crosses a package boundary, or browse the [runnable concurrency recipes](../examples/) to see task, channel, and select behavior together.
