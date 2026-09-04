---
title: C ABI and FFI
description: Export stable C symbols and generate checked incoming C bindings with explicit ownership and lifetime rules.
---

# C ABI and FFI

Kinmokusei provides two deliberately separate C boundaries:

1. **Outgoing C ABI** exposes selected Kinmokusei functions to C, Rust, Nim, and other C ABI consumers.
2. **Incoming C FFI** generates an isolated cgo package from a checked ownership-aware manifest.

Ordinary application code remains Go internally. A C gateway exists only when requested explicitly.

## Export a C ABI

```ts
function add(left: int32, right: int32): int32 { return left + right; }
function subtract(left: int32, right: int32): int32 { return left - right; }

export c("kinmokusei_add", "kinmokusei_subtract") { add, subtract };
```

Generate the gateway, header, canonical manifest, and fingerprint:

```sh
keika emit-c-abi -o ./generated-c-abi library.km
```

Generated C functions return an `int32_t` status. Non-void results are written through a final out parameter:

| Status | Meaning |
| --- | --- |
| `KINMOKUSEI_ABI_OK` (`0`) | Success |
| `KINMOKUSEI_ABI_PANIC` (`1`) | Contained panic in Kinmokusei/Go code |
| `KINMOKUSEI_ABI_INVALID_ARGUMENT` (`2`) | Invalid boundary argument, such as a null out pointer |

Panics never cross the C boundary. An out value is unspecified on failure.

The generated header also publishes `KINMOKUSEI_ABI_GATEWAY_VERSION_MAJOR`, `KINMOKUSEI_ABI_GATEWAY_VERSION_MINOR`, and `KINMOKUSEI_ABI_FINGERPRINT`. Generated ABI metadata therefore uses the `KINMOKUSEI_ABI_*` namespace; public example symbols use the lowercase `kinmokusei_*` prefix.

## Stable outgoing types

The boundary accepts `boolean`, fixed-width signed/unsigned integers, `byte`/`uint8`, `float32`, float aliases, `void` results, and native enums whose ultimate underlying type is fixed-width.

Boolean transport is `uint8_t`: zero is false, any nonzero input is true, and output is normalized to zero or one. Machine-width `int`/`uint`, strings, collections, classes, pointers, interfaces, channels, and errors are rejected from the stable outgoing boundary.

## Detect ABI breaks

```sh
keika abi check --baseline ./previous/kinmokusei_abi.json library.km
```

Declaration order and source aliases do not affect the canonical fingerprint. Prefer adding a symbol over changing a published signature.

## Generate an incoming binding

```sh
keika ffi generate --manifest ./binding.json -o ./internal/imageffi
```

Schema 1 validates the manifest before cgo generation. Its implemented surface includes fixed/C-width scalars, borrowed string/byte inputs, copied and released owned string/byte/typed-array results, enums, nested POD values, normalized tagged unions, callbacks, opaque handles, target flags, status/out conventions, and thread policies.

The generated package is a low-level private boundary. Wrap it with ordinary Kinmokusei code so application APIs do not expose `C.*`, raw pointers, release symbols, or cgo details.

## Ownership rules

No wrapper guesses ownership from a pointer type or function name.

- Borrowed input is copied to C-owned memory for the call when required and released afterward.
- Owned output requires a declared release function and is copied into independent Go storage before release.
- Opaque handles have explicit close behavior with nil, double-close, use-after-close, and active-registration checks.
- Registered callbacks stay alive until successful unregister and admitted calls drain.
- Retained string/byte registration inputs remain registration-owned until successful unregister.

Complex C or C++ APIs should expose a small stable C shim using fixed signatures, POD values, and opaque handles.

## Thread behavior

| Policy | Contract |
| --- | --- |
| `threadSafe` | Calls may execute concurrently |
| `serialized` | One generated mutex allows one call at a time |
| `threadAffine` | One executor locks a goroutine to a dedicated OS thread |

Serialized and affine executors are non-reentrant in schema 1. A callback must not synchronously reenter the same occupied binding or close its own registration.

For the complete public key, convention, type, callback, handle, and thread-policy inventory, use the [incoming C FFI manifest reference](../reference/c-ffi-manifest). The repository's [C ABI and FFI design](https://github.com/puffball1567/kinmokusei/blob/main/docs/c-ffi.md) records additional rationale.

Follow [Export a C ABI library](../examples/c-abi) for a complete outgoing source file and [Generate an incoming C binding](../examples/incoming-c-ffi) for a checked schema 1 manifest.
