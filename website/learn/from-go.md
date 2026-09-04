---
title: Coming from Go
description: Map Go values, packages, errors, concurrency, and public APIs to Kinmokusei source.
---

# Coming from Go

Kinmokusei aims to preserve Go behavior where the two languages share a construct. The compiler adds higher-level source forms, but generated Go remains a first-class artifact and direct package interoperability stays explicit.

## Concept map

| Go | Kinmokusei |
| --- | --- |
| `bool` | `boolean` |
| `[]T`, `[N]T`, `map[K]V` | `T[]`, `[N]T`, `Map<K, V>` |
| Named value type | `struct` or `type Name = distinct T` |
| Pointer-backed object pattern | `class` |
| `(T, error)` | direct Go multiple result or `Result<T>` return effect |
| Package import | namespaced `import go alias from "path"` |
| Goroutine statement | `go call();` |
| Managed result goroutine | `const task = go call()` plus `await` or `detach` |
| `go:generate`-style artifact | explicit `keika emit-go` |

## Values preserve Go rules

Fixed arrays and native structs copy on assignment, argument passing, and return. Slices alias backing storage; maps and channels are reference-bearing values; classes are pointer-backed references with identity. Named imported Go types keep package identity and do not collapse to their underlying scalar or collection.

Range sources evaluate once. The single-binding `for-of` form intentionally yields the value, not Go's first range variable:

```ts
for (const value of values) { consume(value); }
for (const [index, value] of values) { consumeAt(index, value); }
```

## Go packages stay Go packages

```ts
import go http from "net/http";
import go time from "time";

const timeout: time.Duration = time.Second * 5;
let client: http.Client = http.Client { Timeout: timeout };
```

The compiler loads export data from the selected toolchain and preserves named types, aliases, fields, tags, pointers, interfaces, methods, multiple results, variadics, generics, channels, and constraints. It does not translate Go source into Kinmokusei syntax or wrap calls in reflection proxies.

## Errors have explicit bridges

Raw Go multiple results remain available:

```ts
const [value, err] = strconv.Atoi(text);
```

Within a function returning `Result<int>`, postfix `?` explicitly checks and propagates the same Go error:

```ts
const value = strconv.Atoi(text)?;
return ok(value);
```

No implicit conversion turns an arbitrary `(T, error)` into a wrapper. Typed exceptions are a separate source-level mechanism with an explicit structural marker; ordinary panics continue unwinding after `finally`.

## Classes add explicit reference OOP

Classes provide constructors, visibility, interfaces, static methods, and optional single inheritance with `virtual`, `override`, `final`, and `super`. Generated APIs use Go structs, methods, interfaces, and conversion helpers. Native `struct` remains available when Go value behavior is the desired contract.

## Generated Go is part of the product

`keika emit-go` writes deterministic, `gofmt`-formatted source that builds without Kinmokusei or a compiler runtime. Public generated APIs are tested from ordinary external Go packages. For performance-sensitive or not-yet-expressible boundaries, handwritten Go can live beside generated packages.

## When ordinary Go is the better source language

Use Go directly when the code is primarily low-level runtime work, depends heavily on syntax Kinmokusei does not implement, or benefits more from Go's native tooling than from Kinmokusei's class/null/result abstractions. The boundary is designed to support that choice rather than hide it.

Continue with [Go interoperability](../guide/go-interop) and [Generated Go](../guide/generated-go).
