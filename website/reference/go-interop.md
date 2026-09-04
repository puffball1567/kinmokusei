---
title: Go interoperability matrix
description: Detailed support matrix for Go declarations, types, calls, generics, interfaces, channels, targets, CGO, and unsafe boundaries.
---

# Go interoperability matrix

The compiler loads real export/type information for the selected Go version, module graph, target, tags, and CGO state. Support is determined lazily for referenced declarations and reachable shapes.

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
| Function values/callbacks | Implemented | Checked against the original Go signature |

## Type shapes

| Shape | Status | Notes |
| --- | --- | --- |
| All Go basic types | Implemented | Identity/width preserved; explicit conversions |
| Arrays, slices, maps | Implemented | Named collections and copy/alias behavior preserved |
| Anonymous structs | Implemented | Exported reachable fields/tags preserved |
| Interfaces | Implemented | Method sets, assignment, assertions, and type switches |
| Channels and directions | Implemented | Send/receive/close/range/select behavior preserved |
| Multiple results | Implemented | Local destructuring/reassignment; never first-class |
| Raw `error` | Implemented | Explicit split or postfix `?` bridge |
| Generic named types | Implemented | Explicit instantiation and identity |
| Generic functions/methods | Implemented | Inferred, partial, or full explicit arguments |
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

No implicit bridge turns `(T, error)` into a hidden wrapper, erases pointer identity for nullability, or converts a Go interface into a Kinmokusei class hierarchy.

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
