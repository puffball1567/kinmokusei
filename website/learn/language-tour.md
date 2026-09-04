---
title: Language tour
description: Learn Kinmokusei declarations, values, errors, types, packages, and concurrency through a small program.
---

# Language tour

This tour follows a small visit record rather than presenting disconnected syntax. The complete source is checked and run by the documentation test.

<<< ../snippets/language-tour.km{ts}

It prints:

```text
Aki · 42°C
```

## Imports are explicit boundaries

```ts
import go errors from "errors";
import go fmt from "fmt";
import go strings from "strings";
```

Each `import go` declaration creates a namespace backed by the selected Go toolchain's real package export data. Go symbols are never injected into the local scope, and unsupported reachable shapes fail at their Kinmokusei use site.

Relative Kinmokusei modules use selective imports:

```ts
import { Visit, prepareVisit } from "./visit";
```

Files have independent scopes; transitive imports do not leak names.

## Bindings and inference

Use `const` when a binding will not be reassigned and `let` when it will. Local initializers usually provide the type:

```ts
const guest = strings.TrimSpace(name); // string
let attempts: int = 0;
attempts += 1;
```

Public boundaries—parameters, results, fields—use explicit types. Integer literals default to `int` when no other expected type is present. `number`, `float`, and `float64` are the same floating-point type; `int` remains separate.

## Data has an ownership shape

The example declares both an enum and a value struct:

```ts
enum BathStatus { Open, Resting }

struct Visit {
  public guest: string;
  public temperature: int;
}
```

An enum is a distinct integer type. A `struct` is a nominal Go-style value: assignment, argument passing, and return copy the outer value. A `class`, by contrast, is a reference with identity. Structural object types are convenient anonymous data shapes with deterministic JSON tags.

Choose the form based on value versus reference behavior, not only syntax. [Types and data](../guide/types-and-data) gives the full matrix.

## Functions state their effects

```ts
function prepareVisit(name: string, temperature: int): Result<Visit> {
  // ...
}
```

Parameters and results are explicit. `Result<Visit>` is a return effect that lowers directly to `(Visit, error)`; it is not a storable wrapper object.

Functions can be generic, and arrow functions can satisfy ordinary Kinmokusei or imported Go callback signatures:

```ts
function identity<T>(value: T): T { return value; }
const double = (value: int): int => value * 2;
```

## Failure paths stay visible

```ts
if (guest === "") {
  return fail(errors.New("guest name is required"));
}
return ok(Visit { guest: guest, temperature: temperature });
```

Use `Result<T>` for expected operation failure. Postfix `?` propagates a failed Kinmokusei result or a Go `(T, error)` operation from another `Result` function. Explicit `[value, err]` splitting keeps the raw Go result available when you want to inspect it.

Typed `throw`/`catch`/`finally` is a separate exceptional path. Ordinary Go/runtime panics are not silently converted into application exceptions.

## Control flow follows Go-shaped behavior

Kinmokusei provides `if`, value and type `switch`, `while`, C-style `for`, `for-of`, labels, `goto`, and `defer`. Range sources execute once. A single `for-of` binding receives the value; use `[index, value]` or `[key, value]` for both positions.

```ts
for (const [index, value] of values) {
  if (index === 0) { continue; }
  consume(value);
}
```

Map iteration order remains unspecified, and range values are copies.

## Null must be proven away

`T | null` is a checked nil-backed reference. Access requires a non-null proof:

```ts
const user = findUser(id);
if (user === null) { return "missing"; }
return user.name;
```

Assignments, aliasing, address-taking, mutable closures, unknown calls, and control-flow joins invalidate a proof when the value may have changed. Bind a fresh `const` snapshot when storage is not stable.

## Concurrency has a low and high level

Raw `go call();`, channels, `select`, and `defer` preserve Go behavior. A `go` expression creates a tracked `Task<T>`:

```ts
const task = go loadUser(id);
const user = await task;
```

Every task must be consumed exactly once by `await` or `detach` on every continuing path. Calls and arguments evaluate eagerly before the goroutine starts. Context and cancellation are passed explicitly today.

## The result is ordinary Go

Generation preserves identity, copy versus alias behavior, evaluation order and count, nil behavior, errors, panics, and concurrency wherever a direct Go equivalent exists. Continue with the [Language Manual](../book/) for a connected treatment, use the [language guide](../guide/language-basics) for focused tasks, or jump to the [reference](../reference/).
