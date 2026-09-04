---
title: Coming from TypeScript
description: Understand what is familiar—and what is intentionally different—when moving from TypeScript to Kinmokusei.
---

# Coming from TypeScript

Kinmokusei deliberately feels familiar to TypeScript readers, but familiarity stops at the source surface. The runtime, package graph, type boundaries, and generated artifacts are Go.

## Quick comparison

| Topic | Familiar shape | Kinmokusei contract |
| --- | --- | --- |
| Annotations | `name: string` | Static types checked for predictable Go generation |
| Functions and arrows | `function` and `=>` | Explicit result types at public boundaries; Go-compatible callbacks |
| Classes and interfaces | `class`, `interface`, visibility | Reference classes lower to Go structs/methods/interfaces; no prototype model |
| Objects | `{ name: string }` | Structural value objects generate anonymous Go structs with JSON tags |
| Numbers | `number` spelling exists | `number` is `float64`; `int` and fixed-width integers remain distinct |
| Null | `T \| null` | Only nil-backed reference types; flow proof required before access |
| Errors | Promise rejection / thrown values | Explicit `Result<T>`, raw Go `error`, or typed exceptions |
| Async | `Promise<T>` / `await` | Non-escaping, exactly-once `Task<T>`; not a Promise runtime |
| Modules | npm/ES modules | Relative `.km` modules and explicit Go package namespaces |

## There is no JavaScript runtime

Kinmokusei does not provide JavaScript objects, prototypes, the DOM, Node.js globals, npm packages, dynamic property lookup, or TypeScript erasure semantics. A source construct is accepted only when the compiler can give it a defined Kinmokusei meaning and predictable Go representation.

This is valid because `strings` is a real Go namespace:

```ts
import go strings from "strings";

function normalize(value: string): string {
  return strings.ToUpper(strings.TrimSpace(value));
}
```

## Choose data by runtime behavior

TypeScript developers often use one object/class model for many jobs. Kinmokusei separates three shapes:

- A structural object is an anonymous data value with deterministic JSON tags.
- A native `struct` is a nominal value copied with Go struct rules.
- A `class` is a reference with identity, visibility, interfaces, and optional explicit inheritance.

Slices and maps carry shared backing storage even when held inside a copied struct, so outer value copying is shallow.

## `number` is not the universal numeric type

`number`, `float`, and `float64` are identical. `int`, `uint`, fixed-width signed and unsigned integers, `byte`, and `float32` are separate types. Numeric values do not silently widen or cross signedness; use an explicit conversion when Go conversion rules permit it.

## Rejected implicit behavior

Familiar surface syntax never enables JavaScript coercion. A string condition is rejected rather than converted by truthiness:

<<< ../snippets-invalid/truthiness-condition.km{ts}

Likewise, a runtime `int32` does not widen merely because the result expects `int64`:

<<< ../snippets-invalid/implicit-numeric-widening.km{ts}

Write `return int64(value);` when that conversion is intended. Fixed arrays and slices are also distinct storage contracts rather than interchangeable array-like values:

<<< ../snippets-invalid/fixed-array-as-slice.km{ts}

Use `copyArray` or `viewArray` for the supported slice-to-fixed-array direction. Choose a slice in the function contract when variable length is intended.

## `Result<T>` is not a tagged object

`Result<T>` can appear only as a function or method return effect. It lowers to Go `(T, error)`, while `Result<void>` lowers to `error`.

```ts
function parse(text: string): Result<int> {
  const value = strconv.Atoi(text)?;
  return ok(value);
}
```

It cannot be stored in a variable, field, collection, or nested result. This keeps the public Go API direct.

## `Task<T>` is not `Promise<T>`

A task is a local single-consumption capability. It cannot be copied, reassigned, captured, stored, or returned. Every continuing path must consume it exactly once with `await` or `detach`.

The callee and arguments execute synchronously once, before the worker goroutine starts. A worker panic is transported and re-panicked by `await`. Context propagation and cancellation remain explicit through ordinary Go `context.Context` values.

## Null facts can expire

A non-null check narrows a stable local or class-field path. Later writes, aliases, mutable captures, addresses, unknown calls, and loop joins can invalidate that proof. The compiler points to the invalidating boundary; a `const` snapshot creates stable local identity when needed.

## What to learn next

Start with [Types and data](../guide/types-and-data), then read [Errors and nullability](../guide/errors-and-nullability) and [Concurrency](../guide/concurrency). Those three pages cover the largest semantic differences from TypeScript.
