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

Development builds require `open` before using a checked payload whose zero
would violate its source contract. A one-value reference binding is nullable;
handle it with a null guard, `??`, or `!`. Safe-zero elements keep Go behavior.

Receive assignment targets are evaluated only after selection. All target
locations are captured before writes, including field/index/pointer targets and
property receivers. For example, a setter cannot change the array slot chosen
for a later target:

```ts
select {
  case [holder.value, flags[index]] = <-channel {
    // Both target locations were chosen before holder.value's setter ran.
  }
}
```

Unsafe checked payloads require local payload/flag bindings so success can be
tracked; receive into locals, verify `open`, then update other storage.

<<< ../snippets-invalid/select-value-without-guard.km{ts}

Conversions into nullable base classes or interfaces preserve null. See
[zero values and absence](../reference/types#zero-values-and-absence).

See [Concurrency](../guide/concurrency#select).
