---
title: Go interoperability matrix
description: Detailed support matrix for Go declarations, types, calls, generics, interfaces, channels, targets, CGO, and unsafe boundaries.
---

# Go interoperability matrix

The compiler loads real export/type information for the selected Go version, module graph, target, tags, and CGO state. Support is determined lazily for referenced declarations and reachable shapes.

Namespace and named `import go` forms select the same original exports. Named
imports can supply local aliases without changing Go type identity; manifest
source import aliases never rewrite Go package paths.

## Declarations and values

| Go surface | Status | Kinmokusei boundary |
| --- | --- | --- |
| Package constants | Implemented | Qualified namespace member; untyped constant rules preserved |
| Package variables | Implemented | Qualified value with Go mutability/addressability |
| Functions | Implemented | Ordinary call; multiple results remain multiple |
| Named and alias types | Implemented | Package identity and alias transparency preserved |
| Structs, fields, tags | Implemented | Go literal/field selector rules for exported fields |
| Pointers and `nil` | Implemented | Raw low-level Go pointer/nil semantics |
| Methods and method values | Implemented | Value/pointer method sets and addressability preserved |
| Imported Go method expressions | Implemented since v0.4.7 | `pkg.Type.Method` / `(*pkg.Type).Method`; explicit first receiver, interface/promoted methods, imported aliases and direct `<T>`/`[T]` receiver instantiation with checked type arguments |
| Function values/callbacks | Implemented | Checked against the original Go signature |

## Type shapes

| Shape | Status | Notes |
| --- | --- | --- |
| All Go basic types | Implemented | Identity/width preserved; explicit conversions |
| Go runtime `string` | Implemented as `bstring` | Arbitrary bytes; checked `string(raw)` returns `Result<string>` |
| Arrays, slices, maps | Implemented | Named collections and copy/alias behavior preserved |
| Anonymous structs | Implemented | Exported reachable fields/tags preserved |
| Interfaces | Implemented | Method sets, assignment, assertions, and type switches |
| Channels and directions | Implemented | Send/receive/close/range/select behavior preserved |
| Multiple results | Implemented | Destructuring/reassignment, matching return forwarding and sole-call argument expansion; never tuple values |
| Raw `error` | Implemented | Explicit split or postfix `?` bridge |
| Generic named types | Implemented | Explicit instantiation and identity |
| Generic functions/methods | Implemented | Inferred, partial, or full explicit arguments |
| Instantiated generic function values | Implemented since v0.4.7 | `function<T, U>` / `function[T, U]`; full explicit arguments only, including named Go imports; generic instance method values remain unsupported |
| Constraints | Implemented when reachable/representable | Checked using Go type information |
| `unsafe.Pointer` in public shape | Requires policy | `[go.interop].unsafe = "allow"` |

## Calls and operations

| Operation | Status |
| --- | --- |
| Variadic calls and final `slice...` expansion | Implemented |
| Explicit Go-compatible conversions | Implemented |
| Interface assignment and explicit class conformance | Implemented |
| Checked `as?` and forced `as!` assertion | Implemented |
| Explicit type-binding switch | Implemented |
| Channel send/receive/checked receive/close/range/select | Implemented |
| Raw goroutine and `defer` calls | Implemented |
| Supported `unsafe` compiler built-ins | Implemented behind allow policy |

With explicit unsafe permission, `unsafe.Slice` and `unsafe.SliceData` accept
constrained pointer/slice types with a common element contract, including native
class elements. Source nullability is preserved. Lengths and offsets support
integer-valued untyped constants and contextual shifts; negative constant lengths
and overflowing constants are rejected. These checks do not make pointer arithmetic,
lifetimes, aliasing, or `unsafe.String` backing storage safe automatically.

No implicit bridge turns `(T, error)` into a hidden wrapper, erases pointer identity for nullability, or converts a Go interface into a Kinmokusei class hierarchy.

Pointers to nominal native structs keep their source identity in matching source
signatures and pointer receiver methods. Passing shared verified-text storage
through unclassified Go `any`/interfaces is rejected: reflection could write unchecked
bytes or invoke a callback with raw input. This includes `string[]`, pointers to
text-bearing records, classes and hidden implementation fields; generic storage
must prove the same property for its complete bound. Go containers/callbacks use
`bstring`, not a no-copy view of verified text. Scalar text and scalar-only
by-value DTOs can be copied safely. Named generic interfaces are checked with
their instantiated owner arguments, including inherited interface contracts.
Source structural interfaces also inspect compatible class implementations;
method signatures alone do not prove the absence of hidden verified-text fields.

A union bound is checked term by term, not by whether all terms share a single
Go storage shape. For example, this wrapper accepts verified text as well as
numbers and booleans:

```ts
import go reflect from "reflect";

constraint Scalar = ~int | ~string | ~boolean;
function inspect<T extends Scalar>(value: T): reflect.Value {
  return reflect.ValueOf(value);
}
```

An unconstrained `T`, a `comparable`-only bound, or a union containing shared
verified-text storage such as `~string[] | ~int` is still rejected at this
boundary. Adding a safe alternative does not make an unsafe term safe.

The compiler recognizes an audited borrowed-input contract for direct calls to
Go's `reflect.DeepEqual`. It observes inputs without writing reachable storage,
retaining references after return, invoking input methods/callbacks, or returning
a mutation capability. This permits unconstrained native generic values,
verified-text containers, records and nullable class values. Comparison retains
Go semantics, including typed nil, nil versus empty slices, cycles, functions
and NaN; no data-copy adapter is inserted.

The contract requires the resolved package function and its exact signature,
not its source spelling. Namespace and named import aliases are supported.
Unknown function values, same-named source functions/methods, `reflect.ValueOf`
and unclassified APIs receive no exemption. Nested expressions still undergo
all ordinary checks. Use a source wrapper that directly calls `DeepEqual` rather
than storing it in an unclassified function value. See the
[comparison example](../guide/go-interop#read-only-comparison).

Since v0.4.10 the compiler separately recognizes direct `fmt.Sprintf(string, ...any)`
calls. Unlike `DeepEqual`, this audited operation invokes formatting callbacks;
ordinary effect invalidation remains enabled. It returns only raw text and
does not expose the arguments through its result. The exception does not extend
to `fmt.Errorf`, whose `%w` result can retain an error, or arbitrary writers.
Identity/signature checks and rejection of unsafe nested expressions remain
unchanged.

For generic reflection, compiler-owned `inspect*` operations expose only scalar
observations. For generic JSON encoding, `jsonSnapshot` returns bytes and a
text-only error instead of an error containing a writable reflection value.
Neither feature grants a general `any` escape or weakens decoding/assertion
checks. Open-ended error extraction uses concrete typed callbacks rather than
unproven `*T` assertions or writable Go calls. These v0.4.10 APIs and the
required library adaptations are documented together in
[safe generic Go adapters](../guide/safe-go-adapters).

For `encoding/json` decoder targets, use a raw DTO with `bstring` fields, then
validate text before constructing the domain model. Native struct pointers with
raw fields are supported too; see the
[checked class-input example](../guide/classes-and-structs#json).

Go assertions/type switches cannot establish UTF-8 validity. Use raw `bstring`
at that boundary, then a checked conversion. Valid Go string constants may be
verified at compile time. External Go code calling or mutating generated APIs
must respect their source contracts: Go `string` itself carries no UTF-8 proof.
See [text at the Go boundary](../guide/go-interop#text-at-the-go-boundary).

## Package and target behavior

| Boundary | Contract |
| --- | --- |
| Standard/external package | Same importer and type model |
| Existing module graph | Read-only during normal compilation |
| Locked project graph | Canonical, offline, target-aware validation |
| OS/architecture files | Selected by locked GOOS/GOARCH/tags |
| Package-internal reflection/unsafe/assembly | Allowed when the selected Go toolchain builds it |
| CGO package | Available only with selected target/toolchain/libraries |
| Go `internal` and semantic versions | Original Go rules apply |

## Classification

`keika interop audit` reports:

- `supported`: lossless under the default safe policy;
- `requires_unsafe`: representable only with explicit unsafe permission;
- `unsupported`: a reachable public shape cannot be represented;
- package-load failure: the selected environment could not load the package.

Package load success does not guarantee that every export is supported. Conversely, an unused unsupported export does not prevent using the rest of the package.

## Current explicit non-goals

- Parsing or translating arbitrary Go source syntax into `.km`.
- Reflection proxies that hide original package identity.
- Automatic approximation of unsupported reachable shapes.
- Treating public API connectivity as proof that every Go language feature has a Kinmokusei spelling.
- Silently making unsafe or CGO boundaries portable across targets.
