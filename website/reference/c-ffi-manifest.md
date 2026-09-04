---
title: Incoming C FFI manifest
description: Schema 1 keys, conventions, boundary types, ownership, callbacks, handles, and thread policies.
---

# Incoming C FFI manifest

`keika ffi generate` accepts exactly one schema 1 JSON document. Unknown fields, trailing JSON, malformed identifiers, duplicate public names, unsupported type combinations, and ambiguous ownership are rejected before any cgo source is written.

## Top-level keys

| Key | Required | Contract |
| --- | --- | --- |
| `schemaVersion` | Yes | Integer `1` |
| `package` | Yes | Valid generated Go package identifier |
| `header` | Yes | Non-empty, single-line C include path |
| `threadPolicy` | Yes | `threadSafe`, `serialized`, or `threadAffine` |
| `cFlags`, `ldFlags` | No | Validated global cgo flag arrays |
| `targets` | No | Unique GOOS/optional GOARCH flag overrides |
| `functions` | No | C call declarations exposed as Go functions |
| `handles` | No | Opaque pointers with explicit release symbols |
| `structs`, `enums`, `taggedUnions` | No | Named by-value boundary types |
| `callbacks`, `callbackRegistrations` | No | Call-scoped or explicitly registered callbacks |

See the [minimal generated example](../examples/incoming-c-ffi) for a complete file.

## Function declaration

```json
{
  "name": "ImageOpen",
  "symbol": "image_open",
  "parameters": [{"name": "id", "type": "int64"}],
  "result": "Image",
  "convention": "statusOut"
}
```

`name` is the exported Go name; `symbol` is the C identifier. Each parameter has a unique Go name and boundary type.

| Convention | C shape | Generated Go shape |
| --- | --- | --- |
| `direct` | Returns the declared result directly | `func Name(...) T`, or no result for `void` |
| `status` | Returns `int32_t` status | `func Name(...) error` |
| `statusOut` | Returns status and writes a final out parameter | `func Name(...) (T, error)` |

`ownedCString`, `ownedBytes`, and `ownedArray` results require `statusOut` plus `resultRelease`. `ownedArray` also requires `resultElement`.

## Scalar and buffer types

Fixed-width types are `int8`, `int16`, `int32`, `int64`, `byte`, `uint16`, `uint32`, `uint64`, `float32`, `float64`, and `boolean`. `cInt32` and `cUint32` add a generated compile-time check that the platform C type is 32-bit. `void` is result-only.

| Type | Position | Ownership |
| --- | --- | --- |
| `cstring` | Function input/result | Input is copied for the call; result is borrowed and copied to Go |
| `borrowedBytes` | Function input | Go bytes are copied to temporary C memory for the call |
| `ownedCString` | Function/callback result where allowed | Independent allocation copied to Go/C and released explicitly |
| `ownedBytes` | Function/registered callback result | Independent buffer with declared/paired release |
| `ownedArray` | Function/registered callback result | Independent typed array; requires scalar, enum, or POD element |
| `copiedCString`, `nullableCopiedCString` | Callback input | C memory is copied before user code runs |
| `copiedBytes`, `inoutBytes` | Callback input | Checked copy-in; `inoutBytes` copies back only after normal return |
| `retainedCString`, `retainedBytes` | Registration input only | Registration-owned C copy lives until successful unregister |

Embedded NUL, null/length disagreement, oversized counts, and missing release declarations are errors rather than guessed behavior.

## Named values

- `enums` declare a C type, fixed-width `underlying` type, and named C symbols; generation produces a Go named type and constants.
- `structs` declare `name`, `cType`, and fields with Go name, C member name, and type. They are acyclic POD values; pointer-bearing and recursive layouts are rejected.
- `taggedUnions` declare the outer C type, tag field, and variants with accepted tag symbols. Unknown runtime tags are preserved with variant fields zeroed. `overlaidTag` is available for layouts whose variants overlay the tag at offset zero.

## Handles

```json
{"name": "Image", "cType": "image_handle", "release": "image_free"}
```

The generated handle has explicit `Close`, nil/closed checks, one successful release, and use-after-close rejection. A function may use at most one handle in schema 1, giving locking an unambiguous order. Active callback registrations lease the handle and prevent closing it early.

## Callbacks

A callback declares `name`, `lifetime`, parameters, and result. `callScoped` is valid only until the outer C function returns; C must join any thread still invoking it before return. The wrapper deletes its `runtime/cgo.Handle` on every outer return path.

`registered` callbacks require a matching registration:

```json
{
  "name": "Watch",
  "callback": "Visit",
  "parameters": [],
  "register": "watch_add",
  "unregister": "watch_remove"
}
```

Generation exposes `RegisterWatch`, a registration value with `Close` and `CallbackError`. Successful close prevents new entries, unregisters, waits for admitted calls, then releases the handle. A failed unregister keeps the registration live for retry. No finalizer guesses shutdown order.

Callback panics never cross C. The first becomes `CallbackPanicError`; checked input-contract failures become `CallbackInputError`.

## Thread policies

| Policy | Generated behavior |
| --- | --- |
| `threadSafe` | Calls may overlap; the library owns synchronization |
| `serialized` | One generated process-local mutex serializes binding operations |
| `threadAffine` | One dedicated goroutine is locked to an OS thread and executes calls synchronously |

`serialized` and `threadAffine` bindings are non-reentrant in schema 1. Do not synchronously enter the same occupied binding from its callback or close that callback registration from inside itself.

## Generation boundary

```sh
keika ffi generate --manifest binding.json -o internal/binding
```

The output is a private low-level Go package. The C header, compiler flags, linker inputs, target C toolchain, and native library remain build prerequisites. Wrap the package behind ordinary application types so raw C lifetime details do not spread through Kinmokusei code.
