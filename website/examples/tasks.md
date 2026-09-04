---
title: Run parallel work with tasks
description: Start eager structured tasks, consume each result exactly once, and join a Task<Result<T>> before propagating its error.
---

# Run parallel work with tasks

This recipe starts two computations before awaiting either result, then joins successful and failing `Result` tasks through one explicit propagation boundary.

## Project tree

```text
tasks/
└── main.km
```

## Source

<<< ../snippets/tasks.km{ts}

## Run

```sh
keika check main.km
keika run main.km
```

Expected output:

```text
16 25 14 true 0 true
```

## Lifecycle contract

- Each `go square(...)` expression starts a worker and returns `Task<int>`.
- The call target and argument evaluate synchronously once before that worker starts.
- Both tasks exist before the first `await`, allowing their work to overlap.
- Every binding is consumed exactly once.
- A worker panic is transported and re-panicked by its `await`.
- `Task<Result<int>>` has two completion layers: `await` joins the worker, then postfix `?` propagates its ordinary operation error.
- The successful result is doubled to `14`; the failed result returns the `int` zero value and a non-nil error.

The compiler rejects an unconsumed, copied, reassigned, captured, stored, escaped, or multiply consumed task. One such invalid program is part of the documentation test suite.

The `await task?` spelling is valid inside a compatible `Result` function. Awaiting without `?` is rejected because `Result` is an effect, not a value that can be stored after the join.

Pass `context.Context` explicitly when work requires cancellation; automatic context inheritance is not implemented.
