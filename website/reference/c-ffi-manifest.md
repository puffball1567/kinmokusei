---
title: Incoming C FFI manifest
description: Schema 1 keys, conventions, boundary types, ownership, callbacks, handles, and thread policies.
---

# Incoming C FFI manifest

`keika ffi generate` accepts exactly one schema 1 JSON document. Unknown fields, trailing JSON, malformed identifiers, duplicate public names, unsupported type combinations, and ambiguous ownership are rejected before any cgo source is written.

This reference includes v0.4.6 extensions: `borrowedArray`,
`retainedArray`, `copiedArray`, `inoutArray`, multiple handle/call-scoped callback
arguments and `mainThread`. Use a v0.4.6 or newer compiler. See
[release compatibility](../project/releases#v046-highlights).

## Top-level keys

| Key | Required | Contract |
| --- | --- | --- |
| `schemaVersion` | Yes | Integer `1` |
| `package` | Yes | Valid generated Go package identifier |
| `header` | Yes | Non-empty, single-line C include path |
| `threadPolicy` | Yes | `threadSafe`, `serialized`, `threadAffine`, or `mainThread` |
| `cFlags`, `ldFlags` | No | Validated global cgo flag arrays |
| `targets` | No | Unique GOOS/optional GOARCH flag overrides |
| `functions` | No | C call declarations exposed as Go functions |
| `handles` | No | Opaque pointers with explicit release symbols |
| `structs`, `enums`, `taggedUnions` | No | Named by-value boundary types |
| `callbacks`, `callbackRegistrations` | No | Call-scoped or explicitly registered callbacks |

See the [minimal generated example](../examples/incoming-c-ffi) for a complete file.

At least one function or callback registration is required. Subscription-only
bindings may omit `functions` or set it to `[]`; callback and type declarations
alone do not provide an operation.

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

Parameter names that shadow generated locals, import aliases, builtins, or
package declarations are renamed only in generated Go signatures. Names such
as `context`, `result`, `output`, and `int32` are therefore accepted without
changing positional argument types or order. Callback input errors continue
to report the original manifest name. Go keywords and duplicate parameter
names are rejected; reserved public API names are not relaxed.

| Convention | C shape | Generated Go shape |
| --- | --- | --- |
| `direct` | Returns the declared result directly | `func Name(...) T`, or no result for `void` |
| `status` | Returns `int32_t` status | `func Name(...) error` |
| `statusOut` | Returns status and writes a final out parameter | `func Name(...) (T, error)` |

`ownedCString`, `ownedBytes`, and `ownedArray` results require `statusOut` plus `resultRelease`. `ownedArray` also requires `resultElement`.

Direct calls with checked inputs (strings, typed arrays, or callbacks), and all
`mainThread` direct calls, additionally return `error`: `(T, error)` for values
and `error` for `void`.

## Scalar and buffer types

Fixed-width types are `int8`, `int16`, `int32`, `int64`, `byte`, `uint16`, `uint32`, `uint64`, `float32`, `float64`, and `boolean`. `cInt32` and `cUint32` add a generated compile-time check that the platform C type is 32-bit. `void` is result-only.

| Type | Position | Ownership |
| --- | --- | --- |
| `cstring` | Function input/result | Input is copied for the call; result is borrowed and copied to Go |
| `borrowedBytes` | Function input | Go bytes are copied to temporary C memory for the call |
| `borrowedArray` | Function input | A Go typed slice is copied to temporary C memory; requires `element` |
| `ownedCString` | Function/callback result where allowed | Independent allocation copied to Go/C and released explicitly |
| `ownedBytes` | Function/registered callback result | Independent buffer with declared/paired release |
| `ownedArray` | Function/registered callback result | Independent typed array; requires scalar, enum, or POD element |
| `copiedCString`, `nullableCopiedCString` | Callback input | C memory is copied before user code runs |
| `copiedBytes`, `inoutBytes` | Callback input | Checked copy-in; `inoutBytes` copies back only after normal return |
| `copiedArray`, `inoutArray` | Callback input only | Requires scalar, enum, or POD `element`; checked copy-in; `inoutArray` copies back only after normal return |
| `retainedCString`, `retainedBytes` | Registration input only | Registration-owned C copy lives until successful unregister |
| `retainedArray` | Registration input only | Requires scalar, enum, or POD `element`; registration-owned typed C copy lives until successful unregister |

Embedded NUL, null/length disagreement, oversized counts, and missing release declarations are errors rather than guessed behavior.

An `ownedArray` function result checks both the C and Go element byte products
against the target Go `int` before reading native memory or allocating its Go
copy. Packed enums and enclosing POD structs can have different element sizes.
If either product is too large, the wrapper returns `ErrOwnedArrayTooLarge`
and still releases the non-null C result once. Zero-length results become
non-nil empty Go slices; a null pointer with a nonzero count returns
`ErrNullOwnedArray`. A nonzero native status remains `StatusError` and takes
precedence over these result checks.

For example, `{"name":"points","type":"borrowedArray","element":"Vector2"}`
accepts `[]Vector2` and passes a C pointer plus `size_t` count. Elements must be
scalars, enums, or POD structs. Empty inputs use null/zero; nonempty inputs are
converted into a zero-initialized C allocation that is freed after the call.
C must not retain this pointer. C mutations do not change the input Go slice.

For callbacks, `{"name":"points","type":"copiedArray","element":"Vector2"}`
accepts a C `const Vector2 *` plus `size_t` count and exposes a Go `[]Vector2`.
The copy is independent and may be retained after the callback returns.
`inoutArray` uses a writable C pointer and writes the copied elements back only
on normal completion, including false, zero, and void callback results. Panic
or input failure leaves C storage unchanged. Multiple writable inputs are
copied separately and written back in declaration order; later writes win if
their C ranges overlap.

Both callback lifetimes support these array inputs. Null requires zero count;
all empty inputs become non-nil empty Go slices. Counts and byte products for
both C and Go element layouts must fit the target Go `int`. PODs are converted
field-by-field, so their layouts need not match. The entire claimed C range
must remain readable, and writable for `inoutArray`, until the gateway returns.
Appending/reslicing a callback's local slice cannot resize the C buffer.
For `borrowedArray` function inputs, allocation failure and overflowing
count/size return an error before the library call. Invalid callback input is
recorded as a callback error without invoking user code.

## Named values

- `enums` declare a C type, fixed-width `underlying` type, and named C symbols; generation produces a Go named type and constants.
- `structs` declare `name`, `cType`, and fields with Go name, C member name, and type. They are acyclic POD values; pointer-bearing and recursive layouts are rejected.
- `taggedUnions` declare the outer C type, tag field, and variants with accepted tag symbols. Unknown runtime tags are preserved with variant fields zeroed. `overlaidTag` is available for layouts whose variants overlay the tag at offset zero.

## Handles

```json
{"name": "Image", "cType": "image_handle", "release": "image_free"}
```

The generated handle has explicit `Close`, nil/closed checks, one successful
release, and use-after-close rejection. Ordinary functions and callback
registrations may accept multiple handles, including different handle types;
wrappers lock them by stable creation ID, locking a repeated object only once.
A registration holds one lease per distinct resource, preventing early close
until unregister succeeds and admitted callbacks finish. Failed registration
leaves no leases; failed unregister retains all leases and permits retry.
The registration saves private ownership state, not mutable public wrappers.

Share pointers, not struct value copies. Copied handles fail with
`ErrCopiedHandle`; copied callback registrations fail with
`ErrCopiedCallbackRegistration`. A rejected operation does not release the
original resource, unregister its callback, or access its native pointer.

## Callbacks

A callback declares `name`, `lifetime`, parameters, and result. `callScoped` is valid only until the outer C function returns; C must join any thread still invoking it before return. The wrapper deletes its `runtime/cgo.Handle` on every outer return path.

A function may accept multiple call-scoped callbacks, including repeated types
and mixed signatures. Each parameter expands to a separate adjacent C callback
function-pointer/context pair. Every argument has independent state and lifetime,
even if the same Go function is passed twice. A nil callback in any slot rejects
the call before entering the native function. Owned, handle, and string results
remain unsupported with call-scoped callbacks.

For multiple callback arguments, all failed slots are returned via `errors.Join`
in declaration order. Each `CallbackArgumentError` identifies the outer
`Function`, the original argument `Parameter`, and an underlying `Err` (a
`CallbackPanicError` or `CallbackInputError`). `errors.As` follows its `Unwrap`.
A failed slot stops invoking user code without suppressing other slots. The Go
result is zero on failure; callback failures take precedence over C status
failure. Single-callback functions retain their existing direct error types.

`panic(nil)` is also retained as `CallbackPanicError`, including with Go's
`GODEBUG=panicnil=1` compatibility setting. Its `Value` may be nil; that does not
mean the callback succeeded. Later user-code entries are suppressed, mutable
inputs are not copied back, and C receives the result's zero value (including
zero owned-array/byte lengths). A panic recovered inside user code is not a
binding error.

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

Registration parameters may use `retainedArray`, for example
`{"name":"points","type":"retainedArray","element":"Vector2"}`. The Go
signature accepts `[]Vector2`; both C operations receive the same element
pointer and `size_t` count. Nil and empty slices use null/zero. Nonempty values
are converted into independent C storage, with POD fields converted individually.
Neither C mutations nor later Go mutations change the other copy. The count is
fixed; C must not free or resize the storage, and must end all access before
successful unregister returns. Native synchronization remains C's responsibility.
`ErrRetainedArrayTooLarge` and `ErrRetainedArrayAllocation` reject oversized or
failed allocations before native registration; all earlier retained inputs
are freed on preparation failure. Failed unregister retains the arrays for
retry; successful close drains callbacks before freeing them. `retainedArray`
is rejected in ordinary function parameters/results and callback inputs/results.

C may deliver an initial notification during registration. Callback failures
remain available through the returned registration's `CallbackError`. If C
registration returns a failure status, it must not retain the callback context
or input pointers, and must prevent future callbacks. The generated wrapper
drains admitted callbacks before releasing that failed registration's storage.

Callback panics never cross C. The first becomes `CallbackPanicError`; checked input-contract failures become `CallbackInputError`.

## Thread policies

| Policy | Generated behavior |
| --- | --- |
| `threadSafe` | Calls may overlap; the library owns synchronization |
| `serialized` | One generated process-local mutex serializes binding operations |
| `threadAffine` | One dedicated goroutine is locked to an OS thread and executes calls synchronously |
| `mainThread` | Initialization pins the startup/main thread; calls and closes from other threads fail with `ErrWrongThread` |

`serialized` and `threadAffine` bindings are non-reentrant in schema 1. Do not synchronously enter the same occupied binding from its callback or close that callback registration from inside itself.

`mainThread` is for normal Go executables, not embedded Go shared libraries or
archives. It requires C11 thread-local storage (or MSVC's equivalent). Calls
are not dispatched to an event queue: keep them on `main` or initialization,
and do not remove the generated initialization thread lock. Test these APIs
through a separate executable rather than an ordinary Go test goroutine.
Under `threadSafe` or `mainThread`, callbacks must not reenter an operation
using the same locked handle.

## Generation boundary

```sh
keika ffi generate --manifest binding.json -o internal/binding
```

The output is a private low-level Go package. The C header, compiler flags, linker inputs, target C toolchain, and native library remain build prerequisites. Wrap the package behind ordinary application types so raw C lifetime details do not spread through Kinmokusei code.
