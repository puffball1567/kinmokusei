---
title: Go interoperability
description: Import standard-library and external Go packages while preserving types, methods, errors, generics, and channels.
---

# Go interoperability

`import go` connects a namespace to an ordinary package selected from the active or locked Go module graph. It is the primary ecosystem boundary, not a reflection shim.

```ts
import go strings from "strings";

function normalize(value: string): string {
  return strings.ToUpper(strings.TrimSpace(value));
}
```

## What the boundary preserves

The compiler reads actual Go export/type data and preserves:

- constants, variables, functions, and aliases;
- basic, named, alias, anonymous struct, pointer, array, slice, map, and interface types;
- public fields, tags, methods, addressability, and method sets;
- multiple results, raw `error`, variadics, function and method values, and callbacks;
- generic functions, named types, methods, constraints, and explicit/inferred type arguments;
- channels and direction, `select`, `defer`, and goroutine calls.

Imported type identity remains qualified:

```ts
import go http from "net/http";
import go time from "time";

const timeout: time.Duration = time.Second * 5;
let client: http.Client = http.Client { Timeout: timeout };
let pointer: *http.Client = &client;
```

`time.Duration` does not collapse into plain `int64`. Unsupported reachable shapes are diagnosed at the source use site; an unused advanced export does not reject an entire package.

## Multiple results and errors

Keep raw Go results when you need them:

```ts
const [value, err] = strconv.Atoi(text);
```

Use postfix `?` inside a Kinmokusei `Result` function when propagation is the intended bridge:

```ts
function parse(text: string): Result<int> {
  const value = strconv.Atoi(text)?;
  return ok(value);
}
```

No implicit conversion erases Go pointer identity, turns Go interfaces into class hierarchies, or wraps `(T, error)` as a hidden object.

## Variadics, generics, and interfaces

Variadic calls accept individual values or one final explicit slice expansion with `values...`. Generic functions infer type arguments where possible and accept partial/full `<T>` or `[T]` lists. Constraints are checked using Go type information.

A Kinmokusei class can explicitly implement a Go interface when its generated public method set matches. Checked `as?` and forced `as!` assertions expose Go interface assertions without changing failure behavior.

## External modules

Install an exact version in a project:

```sh
keika install --go-module github.com/google/uuid@v1.6.0
keika deps check
keika deps licenses
```

Local project-relative replacements are supported for declared dependencies. Normal compilation remains offline and read-only with respect to dependency resolution.

## Audit connectivity

```sh
keika interop audit --stdlib
keika interop audit --json net/http github.com/google/uuid
```

The audit classifies public declarations and reachable shapes as `supported`, `requires_unsafe`, or `unsupported`, and reports package-load failures separately. It measures type connectivity, not coverage of every Go syntax feature.

## Unsafe policy

Unsafe interop is denied by default. A project must set `[go.interop].unsafe = "allow"` before directly importing `unsafe` or using a public signature that exposes `unsafe.Pointer`, including through nested collections, callbacks, or generics.

Supported `unsafe` built-ins have dedicated type rules; enabling the capability does not make pointer arithmetic, lifetime, GC reachability, or panic behavior safe.

## Environment-dependent packages

Pure Go packages are the primary target. Packages may use reflection, unsafe, assembly, generated code, or CGO internally. CGO, OS/architecture files, and build tags must be available for the locked target. Public boundary types still need a representable Kinmokusei connection.

## Runnable examples

- [Encode JSON with a Go package](../examples/json) passes a structural object through the real `encoding/json` API.
- [Use Go standard-library values](../examples/go-standard-library) combines an imported struct, pointer-receiver methods, package calls, explicit multiple results, and raw errors.
- [Parse input with Result](../examples/result-parsing) bridges `strconv.Atoi` into an explicit result.
- [React with Gin and Fiber](../examples/web-backend) uses locked external Go framework modules.
- [Inspect a Go interface with a type switch](../examples/type-switch) narrows imported interfaces to concrete Go pointer types.

See the [Go interoperability matrix](../reference/go-interop) for the declaration/type/operation inventory.
