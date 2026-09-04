---
title: Compatibility reference
description: Supported Go toolchains and platforms, direct Go type connectivity, generated-Go guarantees, and C boundary matrices.
---

# Compatibility reference

## Toolchains and platforms

| Area | v0.2 contract |
| --- | --- |
| Go source compatibility | Go 1.23 through Go 1.27 |
| Release compiler | Built with Go 1.27 |
| Full CI platforms | Linux, macOS, Windows on the current toolchain matrix |
| Release archives | Linux amd64/arm64, macOS amd64/arm64, Windows amd64 |
| Generated cross-builds | Linux amd64/arm64, macOS arm64, Windows amd64 with CGO disabled |

Direct package import reads versioned Go export data. Support for a newer Go minor release may require a matching Kinmokusei release.

## Go interop connectivity

<span class="status-label status-implemented">Implemented</span>

Constants, functions, variables, all Go basic types, named/alias/anonymous types, structs/fields/tags, pointers, nil, methods, multiple results, raw errors, conversions, variadics, callbacks, interfaces/assertions/type switches, generics, named collections, channels/select/goroutines/defer, target-aware loading, CGO loading, and explicit supported unsafe operations.

Package import success and symbol support are separate. Unused unsupported exports do not reject a package; used reachable shapes do. `interop audit` reports safe, unsafe-required, unsupported, and load-failure categories separately.

## Generated Go

<span class="status-label status-implemented">Implemented</span>

Generated source is deterministic, formatted, standalone, buildable, and free of machine-specific paths or an unnecessary compiler runtime. Where a source type has a stable representation, public packages expose ordinary Go APIs usable from an external Go module.

Exact generated helper names are compiler-owned except where the docs explicitly state a public API. Pre-1.0 source and generated API compatibility can change between minor releases with migration notes.

## Outgoing C ABI

| Supported | Rejected from stable boundary |
| --- | --- |
| `boolean`, fixed-width integers, `byte`/`uint8`, floats, fixed-width native enums, `void` | machine-width `int`/`uint`, strings, collections, objects/classes/interfaces, pointers, channels, errors |

Gateways use status/out conventions, panic containment, explicit ASCII symbols, canonical manifests, fingerprints, and compatibility checks.

## Incoming C FFI schema 1

Implemented groups include fixed/C-width scalars, borrowed/copying strings and byte buffers, released owned strings/bytes/typed arrays, enum/POD/tagged-union values, call-scoped and registered callbacks, retained registration inputs, opaque handles, status/out errors, target link flags, and thread-safe/serialized/thread-affine policies.

Unsupported or ambiguous machine-width/pointer/ownership shapes are rejected before cgo generation. Source-level `ffi c library` declarations remain proposed; the checked JSON manifest is the implemented input.

## Behavioral compatibility evidence

The runtime contract registry contains 82/82 implemented Go-equivalent groups, each connected to an isolated independently handwritten Go scenario. This is contract coverage for implemented behavior, not a claim that every Go or planned Kinmokusei feature exists.

See [Quality promise](../project/quality) for the oracle and generated-artifact gates.
