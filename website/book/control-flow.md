---
title: Control flow
description: Learn boolean conditions, loops, ranges, value and type switches, channel select, labels, defer, and try/catch/finally.
---

# Control flow

Control-flow syntax is explicit: conditions use parentheses, bodies use braces, and conditions must be `boolean`. There is no truthiness and no implicit fallthrough.

## `if` and `else`

```ts
if (temperature >= 40) {
  return "hot";
} else if (temperature >= 20) {
  return "warm";
} else {
  return "cool";
}
```

Every branch is a lexical block. A non-`void` function must return on every continuing path; the checker accounts for terminating branches.

Null checks also refine flow facts:

```ts
if (user === null) { return "guest"; }
return user.name;
```

The declared type remains `User | null`; only the reachable path carries a non-null proof.

## `while`

```ts
while (pending()) {
  poll();
}
```

The condition is evaluated before each iteration. `break` leaves the loop; `continue` begins the next condition check.

## Three-clause `for`

```ts
for (let index = 0; index < len(values); index++) {
  consume(values[index]);
}
```

The header contains initializer, condition, and post statement separated by semicolons. Any clause may be empty:

```ts
for (; ready(); ) {
  work();
}
```

The initializer has loop scope. The post clause accepts a simple assignment/update/call form without a trailing semicolon. Loop backedges conservatively join nullable and task-flow state.

## Range loops

One binding receives values:

```ts
for (const value of values) {
  consume(value);
}
```

Two bindings receive index/key and value:

```ts
for (const [index, value] of values) { }
for (const [key, value] of lookup) { }
for (const [offset, rune] of text) { }
```

Supported sources and bindings:

| Source | One binding | Two bindings |
| --- | --- | --- |
| Slice, fixed array, pointer to array | value | `int` index, value |
| Map | value | key, value |
| String | `int32` rune | `int` UTF-8 byte offset, `int32` rune |
| Receive-capable channel | value | Not supported |

The source evaluates once. Bindings may be `const` or `let` and may carry explicit annotations. Map order is unspecified; channel range continues until close and drain.

## Value switch

```ts
switch (method) {
  case "GET", "HEAD" { return "read"; }
  case "POST" { return "write"; }
  default { return "unsupported"; }
}
```

The subject evaluates once. Case expressions are tested in source order and stop after the first match. A case body does not enter the next case automatically.

Explicit `fallthrough;` must be the final direct statement of a non-final value-switch case:

```ts
switch (value) {
  case 0 {
    initialize();
    fallthrough;
  }
  case 1 { run(); }
  default { stop(); }
}
```

Fallthrough does not re-test the next case expressions and does not carry a nullable proof into the next case.

## Go interface type switch

```ts
switch (input) {
  case const reader as *strings.Reader {
    return reader.Len();
  }
  case let writer as io.Writer {
    writer = wrap(writer);
    return 1;
  }
  case nil { return 0; }
  default { return -1; }
}
```

A type switch requires a Go interface subject. Typed cases use `const` or `let`, and each narrowed binding exists only in its case. `_` may ignore the value. Duplicate or impossible types, mixed value/type cases, and duplicate `nil`/`default` are rejected. Type switches do not support fallthrough.

## Channel `select`

```ts
select {
  case const value = <-input { consume(value); }
  case let [value, open] = <-other { inspect(value, open); }
  case output <- pending { sent(); }
  default { idle(); }
}
```

Each communication's operands evaluate according to Go select rules. If multiple communications are ready, one is selected pseudo-randomly. `default` runs only when none is ready. Without default, select blocks; `select {}` blocks forever.

Receive cases may discard, declare one value, declare checked `[value, open]`, or assign to existing targets. Case bodies have independent lexical scopes and never fall through.

## Labeled branches

Labels give `break` and `continue` an explicit enclosing target:

```ts
outer: for (let row = 0; row < 3; row++) {
  for (let column = 0; column < 3; column++) {
    if (column === 1) { continue outer; }
    if (row === 2) { break outer; }
  }
}
```

`continue label;` must target an enclosing loop. `break label;` must target an enclosing loop, switch, or select.

## `goto`

`goto label;` supports controlled transfer within one function:

```ts
goto dispatch;
dispatch: if (ready) { run(); }
```

The checker rejects undefined/unused/duplicate labels, jumps into nested blocks, jumps over declarations, and jumps across `try`, `catch`, or `finally` boundaries. Task consumption and definite initialization must remain valid along every jump path.

## `defer`

```ts
defer resource.Close();
```

`defer` requires a function or method call. Its target and arguments evaluate when the defer statement executes; the call runs when the surrounding function exits, using Go-compatible last-in-first-out order. A deferred call does not replace structured `finally` when cleanup belongs to a specific try/catch region.

## `try`, `catch`, and `finally`

```ts
try {
  load();
} catch (err: NotFoundException) {
  recoverMissing(err.message);
} catch (err: error) {
  recoverGeneral(err);
} finally {
  cleanup();
}
```

Catch clauses are tested in source order. The checker rejects a specific catch already covered by an earlier base or `error` catch. `throw value;` requires an error-compatible value; bare `throw;` rethrows the current catch value.

`finally` executes for normal completion, handled/rethrown language exceptions, return, and ordinary Go/runtime panic. A return or throw from finally replaces the earlier completion. Ordinary panics run finally but do not become catchable typed exceptions.

Control flow determines which names and facts remain valid at each program point. [Modules and imports](./modules-and-imports) explains how those checked regions combine across files and package boundaries.
