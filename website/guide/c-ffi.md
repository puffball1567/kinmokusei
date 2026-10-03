---
title: C ABI and FFI
description: Export stable C symbols and generate checked incoming C bindings with explicit ownership and lifetime rules.
---

# C ABI and FFI

Kinmokusei provides two deliberately separate C boundaries:

1. **Outgoing C ABI** exposes selected Kinmokusei functions to C, Rust, Nim, and other C ABI consumers.
2. **Incoming C FFI** generates an isolated cgo package from a checked ownership-aware manifest.

Ordinary application code remains Go internally. A C gateway exists only when requested explicitly.

The typed borrowed/retained arrays, multiple-handle and multiple call-scoped
callback support, typed callback arrays and `mainThread` policy below are
available in v0.4.6. See
[release availability](../project/releases#v046-highlights).

Since v0.4.6, strings received from generated Go
bindings are raw `bstring`, including C string results and callback arguments.
Use checked `string(raw)` decoding when valid UTF-8 is required. The C manifest
names and generated Go signatures remain unchanged; see
[text at the Go boundary](./go-interop#text-at-the-go-boundary).

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

Schema 1 validates the manifest before cgo generation. Its implemented surface includes fixed/C-width scalars, borrowed string/byte/typed-array inputs, copied and released owned string/byte/typed-array results, enums, nested POD values, normalized tagged unions, callbacks, opaque handles, target flags, status/out conventions, and thread policies.

The generated package is a low-level private boundary. Wrap it with ordinary Kinmokusei code so application APIs do not expose `C.*`, raw pointers, release symbols, or cgo details.

## Ownership rules

No wrapper guesses ownership from a pointer type or function name.

- Borrowed input is copied to C-owned memory for the call when required and released afterward.
- Owned output requires a declared release function and is copied into independent Go storage before release.
- Opaque handles have explicit close behavior with nil, double-close, use-after-close, value-copy, and active-registration checks. Ordinary functions and callback registrations can safely lock multiple handles in a stable order. Registrations keep every distinct resource alive until successful unregister and callback completion; failed unregister preserves the leases for retry.
- Registered callbacks stay alive until successful unregister and admitted calls drain.
- Call-scoped callbacks can occupy multiple independent argument slots; failures from different slots are preserved together, with their original argument names.
- Callbacks can receive copied scalar, enum, or POD arrays without exposing C memory to application code. Writable `inoutArray` inputs are copied back only after normal callback completion; panic or invalid input leaves C storage unchanged.
- Keep handles and registrations as pointers: copying their struct values is rejected without closing the original resource.
- Retained string/byte/typed-array registration inputs remain registration-owned until successful unregister. Preparation failure frees all earlier copies; unregister failure keeps them for retry. `retainedArray` requires a scalar, enum, or POD `element`, so no raw Go pointer is retained by C.

Complex C or C++ APIs should expose a small stable C shim using fixed signatures, POD values, and opaque handles.

## Thread behavior

| Policy | Contract |
| --- | --- |
| `threadSafe` | Calls may execute concurrently |
| `serialized` | One generated mutex allows one call at a time |
| `threadAffine` | One executor locks a goroutine to a dedicated OS thread |
| `mainThread` | Package initialization pins the startup/main thread; another thread receives `ErrWrongThread` |

Serialized and affine executors are non-reentrant in schema 1. A callback must not synchronously reenter the same occupied binding or close its own registration.

Use `mainThread` for native window/event APIs that require the main OS thread
of a normal Go executable. It does not dispatch worker calls to an event queue.
Direct calls return errors under this policy, including `void` calls; handle
and callback-registration closes must also run on the main thread.

## Distribute a C-backed package

Publish the native C sources, headers, generated FFI wrappers and an ordinary
Go file importing `C` in a separately versioned cgo Go module. Declare that
module in the `.km` library's `[go.dependencies]`; application code imports the
library through an ordinary source import. Native files beside `.km` sources
are not compiled automatically, and `ffi generate` does not fetch or bundle the
C library.

With the native module cached, `keika check`, `emit-go`, `build` and `run` use
the locked graph offline without editing manifests or locks. Native compilation
requires cgo, a C compiler and the declared platform link dependencies. Include
the C library's license and required notices in the distributed package. See
[external packages](./external-packages) and the
[C-backed packaging example](https://github.com/puffball1567/kinmokusei/blob/main/docs/c-ffi.md#distributing-a-c-backed-kinmokusei-package)
for manifests and layout.

For the complete public key, convention, type, callback, handle, and thread-policy inventory, use the [incoming C FFI manifest reference](../reference/c-ffi-manifest).

Follow [Export a C ABI library](../examples/c-abi) for a complete outgoing source file and [Generate an incoming C binding](../examples/incoming-c-ffi) for a checked schema 1 manifest.
