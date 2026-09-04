---
title: Coordinate channels with select
description: Choose ready send/receive operations without blocking and handle the default path.
---

# Coordinate channels with `select`

This deterministic example prepares a buffered channel before each select so both the ready and default paths are visible.

## Source

<<< ../snippets/select.km{ts}

## Run

```sh
keika check select.km
keika run select.km
```

Expected output:

```text
value:42 idle
sent blocked
```

## Selection contract

- A receive case is ready when a value can be received immediately.
- A send case is ready when the send can proceed immediately.
- `default` runs only when no communication case is ready.
- Without `default`, select waits until a case becomes ready.
- If several cases are ready, Go chooses one pseudo-randomly.
- Case declarations have case-block scope and do not leak.

Checked receive is also accepted:

```ts
select {
  case const [value, open] = <-channel {
    if (!open) { return; }
    consume(value);
  }
}
```

Select preserves Go evaluation and channel panic/blocking behavior. It does not add cancellation automatically; select on an explicit `context.Context.Done()` channel when needed.

See [Concurrency](../guide/concurrency#select).
