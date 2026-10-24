---
title: Go interoperability
description: Import standard-library and external Go packages while preserving types, methods, errors, generics, and channels.
---

# Go interoperability

`import go` connects explicit source bindings to an ordinary package selected
from the active or locked Go module graph. It is the primary ecosystem boundary,
not a reflection shim.

```ts
import go strings from "strings";

function normalize(value: string): Result<string> {
  return string(strings.ToUpper(strings.TrimSpace(value)));
}
```

Namespace imports keep the export names qualified. Named imports select exports
without that qualifier, with optional local aliases:

```ts
import go { ToUpper as upper, TrimSpace } from "strings"

function normalize(text: string): Result<string> {
  return string(upper(TrimSpace(text)))
}
```

Both forms preserve the original Go package/type identity. Manifest source
import aliases do not expand `import go` paths.

## Text at the Go boundary

Runtime Go `string` values are `bstring` in Kinmokusei. This also applies to
collection elements, struct fields and callback parameters/results. A scalar
`string` argument can be passed to Go without conversion; a returned raw value
must be validated with `string(raw)` before becoming verified text. That
conversion returns `Result<string>` and reports invalid UTF-8 without replacing
bytes. Known-valid Go string constants can be verified at compile time.

For an intentionally raw API, use `bstring` throughout:

```ts
function normalizeBytes(value: bstring): bstring {
  return strings.ToUpper(strings.TrimSpace(value));
}
```

Shared containers are invariant: Go `[]string` requires `bstring[]`, not
`string[]`. Likewise, a Go callback receiving a string must accept `bstring`,
even when the particular package usually supplies text. Build a separate raw
collection or validate callback inputs explicitly; do not reinterpret shared
storage or assume an imported API proves UTF-8 validity.
Imported generic types and inherited Go method contracts must likewise use
raw text arguments. A Go generic call returning ordinary strings returns
`bstring`; native verified-text types cannot be passed through a generic Go
signature that would erase their proof. Native Kinmokusei generics keep their
text contracts normally.

Go `any`/interface values can expose references to reflection. The compiler
rejects escaping verified-text storage, including pointers/containers/classes,
hidden implementing or subclass fields, and unconstrained generic storage
whose safety cannot be proved. Scalar text and by-value records containing only
scalar text are safe to copy. For JSON input, decode into a DTO with `bstring`
fields, then validate and construct the domain object; see the
[checked class-input example](./classes-and-structs#json).

This check also follows source structural interfaces and instantiated generic
interfaces: a method set returning only `bstring` does not prove that its
implementation contains no mutable `string` fields. For example, this is
rejected at the call to `reflect.ValueOf`:

<<< ../snippets-invalid/interface-text-erasure.km{ts}

Use a separate raw-data implementation with `bstring` fields for that boundary.
Unrelated classes with incompatible method sets or generic bounds do not make
an otherwise raw structural interface unsafe.

An interface assertion is a Go type check, not a UTF-8 check. Assert raw
`bstring`, then decode; asserting `string` or a text-bearing mutable type is
rejected. Generated Go represents both contracts with plain `string`, so
external Go callers must honor the declared Kinmokusei contract. Hand-written
Go mutation and explicitly unsafe memory operations are outside the checked
source guarantee.

## Read-only comparison

Use `reflect.DeepEqual` for generic assertions and comparisons of verified-text
collections. Its direct-call safety contract is recognized since v0.4.9:

<<< ../snippets/go-readonly-comparison.km{ts}

This prints `true false`. No unsafe permission or finite scalar constraint is
required. Namespace and named import aliases work too. Keep the direct call
inside your source wrapper: storing the Go function in an unclassified function
value does not carry this contract. Other APIs retain their ordinary checks
unless explicitly covered by a separate audited contract. Since v0.4.10 the
compiler also provides [safe observations, deferred formatting, JSON snapshots and typed
error adapters](./safe-go-adapters); see the [exact contracts](../reference/go-interop).

## What the boundary preserves

The compiler reads actual Go export/type data and preserves:

- constants, variables, functions, and aliases;
- basic, named, alias, anonymous struct, pointer, array, slice, map, and interface types;
- public fields, tags, methods, addressability, and method sets;
- multiple results, raw `error`, variadics, function/method values, method expressions, and callbacks;
- generic functions, named types, methods, constraints, and explicit/inferred type arguments;
- channels and direction, `select`, `defer`, and goroutine calls.

Fully instantiated Go generic function values are supported since v0.4.7;
see [instantiated function values](./functions-and-generics#instantiated-function-values).
These use the same Go type constraints and raw-text boundary as generic calls,
without adding runtime wrappers to those Go function values. Native generic
function values carry source-contract metadata for checked native downcasts.

Imported type identity remains qualified:

```ts
import go http from "net/http";
import go time from "time";

const timeout: time.Duration = time.Second * 5;
let client: http.Client = http.Client { Timeout: timeout };
let pointer: *http.Client = &client;
```

`time.Duration` does not collapse into plain `int64`. Unsupported reachable shapes are diagnosed at the source use site; an unused advanced export does not reject an entire package.

## Method values and method expressions

::: info Available since v0.4.7
Imported Go method expressions are released in v0.4.7.
Older v0.4.6 supports bound method values but not this type-based form.
:::

A bound method value such as `buffer.Len` captures its receiver. A method
expression such as `(*bytes.Buffer).Len` does not: its first argument is the
explicit receiver. The resulting function can be stored or passed as a callback.

<<< @/snippets/go-method-expressions.km

`time.Time.IsZero` accepts a `time.Time`; `(*bytes.Buffer).Len` accepts a
`*bytes.Buffer`. A named Go import alias can also serve as the receiver type,
for example `import go { Time as Instant } from "time"` followed by
`Instant.IsZero`. Interface types such as `io.Reader.Read` are supported and
accept the interface value as the first argument.

The receiver type's actual Go method set decides which methods are available.
`bytes.Buffer.Len` is rejected because `Len` has a pointer receiver; a method
expression never implicitly takes the address of its first argument. Promoted
methods retain the outer receiver type. Fields are not method expressions.
Variadics and multiple results keep their original Go signatures after the
additional receiver parameter.

Editor argument hints retain that first receiver parameter during incomplete
calls, including parenthesized pointer types. Whitespace and comments around
the dot do not prevent method completion or argument hints.

Go nil behavior is unchanged: a nil-aware pointer method can accept `nil`, but
calling a value method through a nil pointer or an interface method through a
nil interface panics as in Go. This feature does not turn raw Go pointers into
nullable-safe class references.

Generic receivers support direct `<T>`/`[T]` instantiation. For example,
`(*atomic.Pointer<int>).Load` from `import go atomic from "sync/atomic"` takes
an explicit `*atomic.Pointer<int>` and returns `*int`. Named imports also work:
`import go { Pointer as Ptr } from "sync/atomic"` permits `(*Ptr[int]).Load`.
All type arguments must satisfy the ordinary Go type constraints and storage
rules: `string` cannot establish a verified UTF-8 contract in a Go generic type;
use `bstring` for raw Go string storage. An exported Go alias of a concrete
instantiation also works; an uninstantiated generic type is rejected.
Native structs, defined types and
interfaces also support the [explicit-receiver form](./classes-and-structs#native-method-expressions).
Class static calls remain a separate feature.

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

Matching ordinary multiple-result calls can also be forwarded with
`return operation();` or expanded as the sole argument to another call. This
does not implicitly turn an ordinary result list into a `Result` effect; keep
the effect explicit when error handling must be checked.

## Runtime cost

An ordinary imported Go function lowers to a direct Go call. Imports do not add
a reflection, serialization or foreign-language ABI layer. `Result` checks
become ordinary error branches. Source classes, virtual dispatch, decorators,
Task starts and C FFI may emit their documented helpers, allocations or copies;
these costs come from the chosen feature, not merely from importing a Go module.
Checked raw-to-text decoding additionally scans bytes for UTF-8 validity;
byte-slice conversions copy a snapshot. Verified-text slicing adds boundary
checks, not a full UTF-8 rescan. Raw `bstring` operations retain Go behavior.

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

Go replacements are supported for declared dependencies at project-relative
paths inside the project root. Source-package sibling replacements use the
separate `[replace]` section. Normal compilation remains offline and read-only
with respect to dependency resolution.

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
