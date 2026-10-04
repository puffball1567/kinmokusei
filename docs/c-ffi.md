# C ABI and FFI

For usage and exact keys, see the [public FFI guide](../website/guide/c-ffi.md)
and [manifest reference](../website/reference/c-ffi-manifest.md). The typed
borrowed/retained arrays, multiple handles/call-scoped callbacks, typed callback
arrays and `mainThread` policy in this document are available in v0.4.6. See
[release availability](../website/project/releases.md#v046-highlights).

## Purpose

The C ABI is a deliberate stable boundary for two directions:

1. **Outgoing export**: expose selected Kinmokusei functions to C, Nim, Rust, and other C ABI consumers.
2. **Incoming FFI**: call C/Nim libraries through generated ownership-aware, type-safe wrappers.

Incoming bindings are also a distribution feature: a binding project should
be able to present an idiomatic Kinmokusei API while keeping cgo, raw pointers,
platform link flags, ownership, and release rules private. This makes a real C
library binding a primary preview/demo target rather than treating FFI only as
compiler plumbing.

The internal program remains ordinary Go. C-compatible gateways are generated only for explicit declarations.

Generated Go `string` values imported from incoming bindings have type
`bstring` in Kinmokusei, including C string outputs and callback text. They
carry no UTF-8 guarantee: use checked `string(raw)` decoding and handle its
`Result<string>` when verified text is needed. Mutable buffers remain `byte[]`;
nullable string pointers are pointers to raw `bstring`, not verified text.
This text-contract split is available in v0.4.6; older releases use the previous
`string` boundary. The generated Go wrapper's ABI and manifest type names are unchanged.

## Implemented outgoing boundary

```ts
function add(left: int32, right: int32): int32 {
  return left + right;
}

const sub = (left: int32, right: int32): int32 => left - right;

export c("kinmokusei_add", "kinmokusei_sub") {add, sub};
```

Symbols and source names are paired by position: `"kinmokusei_add"` exports
`add`, and `"kinmokusei_sub"` exports `sub`. Their counts must match. The source
target must be a top-level function or a top-level `const` arrow with an
explicit result type. Lists may be split across lines and may use trailing
commas:

```ts
export c(
  "kinmokusei_add",
  "kinmokusei_sub",
) {
  add,
  sub,
};
```

The original single-function inline spelling remains supported:

```ts
export c("kinmokusei_add") function add(left: int32, right: int32): int32 {
  return left + right;
}
```

The compiler generates:

- Normal Go implementation code.
- A cgo gateway with stable exported symbols.
- A C header.
- A canonical ABI manifest.
- A SHA-256 ABI fingerprint.
- Compatibility diagnostics against a baseline manifest.

Generated C functions return an `int32_t` status. Non-void values are written through a final out parameter. Status values are:

- `0`: success.
- `1`: contained panic in Kinmokusei/Go code.
- `2`: invalid boundary argument such as a null out pointer.

Panics never cross the C boundary. This also covers `panic(nil)` when
`GODEBUG=panicnil=1` makes Go's recovered value nil: the gateway still returns
status `1`, not success. Null out pointers remain invalid arguments (status
`2`) and do not enter user code. An out value is unspecified on failure.

## Stable initial types

| Kinmokusei | C |
|---|---|
| `boolean` | `uint8_t` (`0` is false, nonzero input is true, output is normalized to `0` or `1`) |
| `byte` | `uint8_t` |
| `uint8` | `uint8_t` (alias of `byte`) |
| `int8` | `int8_t` |
| `int16` | `int16_t` |
| `int32` | `int32_t` |
| `int64` | `int64_t` |
| `uint16` | `uint16_t` |
| `uint32` | `uint32_t` |
| `uint64` | `uint64_t` |
| `float32` | `float` |
| `float` / `float64` / `number` | `double` |
| `enum Name: T` | the fixed-width C integer corresponding to `T` |
| `void` result | no out value |

A native enum is accepted when its ultimate underlying type is one of the
fixed-width integer types above. This includes an enum whose underlying type
passes through a non-generic alias or defined integer type. The gateway
converts the C integer to the named enum before calling user code and converts
the named result back to the same fixed-width transport. The header and ABI
manifest intentionally expose the transport integer rather than a
compiler-specific C enum layout. C callers must use the numeric values defined
by the Kinmokusei enum; unknown representable values are transported without
implicit validation, matching Go named-integer behavior.

Rejected from the initial stable boundary:

- Machine-width C `long`, Go/Kinmokusei `int`/`uint`, Nim `int`, and native
  enums whose ultimate underlying type is machine-width `int` or `uint`.
- Runtime-managed strings, slices, maps, interfaces, channels, classes, and errors.
- Nim `string`, `seq`, and `ref object` runtime layouts.
- Bit fields, flexible arrays, variadic C functions, and untagged or unresolved
  unions. Incoming schema 1 separately supports explicitly mapped tagged unions.
- Raw pointers without an ownership/lifetime contract.
- Complex structs passed by value.

## Why generated wrappers should be safer than raw cgo

- Hide `C.*` and `unsafe.Pointer` from public APIs.
- Reject width-ambiguous ABI types.
- Declare borrowed/owned/retained/static lifetimes and matching release functions.
- Prevent C from retaining Go memory by default.
- Generate string/byte conversion, copying, and release logic.
- Wrap opaque handles in safe classes and detect double release/use-after-release.
- Convert status/out conventions to explicit `Result<T>` wrapper APIs.
- Connect callbacks through generated gateways and integer handles.
- Test symbols, ABI versions, sizes, alignment, and platform link settings.
- Keep cgo isolated inside generated internal packages.

## Implemented incoming schema 1

`keika ffi generate --manifest <binding.json> -o <private-package>`
validates a schema-versioned manifest and emits `generated_ffi.go`, an isolated
cgo package intended to be consumed through ordinary Go interop and wrapped by
an idiomatic Kinmokusei module. Schema 1 supports:

- Fixed-width signed and unsigned integers, `float32`, `float64`, C `bool`,
  and explicitly checked 32-bit C `int`/`unsigned int`.
- Borrowed C string inputs. The wrapper rejects embedded NUL bytes, allocates C
  memory for the duration of the call, and always releases it afterward.
- Borrowed C string results. The wrapper copies the bytes into a Go string
  before returning and reports `ErrNullCString` for a null pointer. Owned or
  retained string results are not inferred by this type.
- Library-allocated C string results. An `ownedCString` status/out result
  requires a `resultRelease` C symbol and consumes a `char **` output. The
  wrapper copies the NUL-terminated bytes into an independent Go string and
  releases every non-null allocation, including one returned together with a
  failing status. A null success result reports `ErrNullOwnedCString`; an
  allocated empty string is valid and is still released.
- Borrowed byte-slice inputs. A `borrowedBytes` parameter becomes one Go
  `[]byte` parameter and one C `const uint8_t *` plus `size_t` pair. The wrapper
  copies into C-owned memory for the call, passes a null pointer for an empty
  slice, and always frees the temporary copy afterward.
- Borrowed typed-array inputs. A `borrowedArray` parameter with an `element`
  naming a scalar, enum, or POD struct becomes `[]T` in Go and a typed C
  pointer plus `size_t` pair. The wrapper checks the allocation size, makes a
  zero-initialized C-owned copy, converts each element, and frees the copy
  after the call. Empty arrays pass a null pointer and zero length. C may
  change its temporary copy, but those changes do not propagate to the Go
  slice. Direct-call wrappers return `(T, error)` (or `error` for `void`) so
  allocation failures are explicit. This parameter type is currently for
  ordinary function inputs, not callback or registration parameters.
- Library-allocated byte results. An `ownedBytes` status/out result requires a
  `resultRelease` C symbol and consumes `uint8_t **` plus `size_t *` outputs.
  The wrapper copies into an independent Go `[]byte` and calls the declared
  release function on success, status failure, null/length inconsistency, and
  an output length too large for the target Go address space whenever the C
  function returned a non-null allocation.
- Library-allocated typed arrays. An `ownedArray` result requires a scalar,
  enum, or POD `resultElement` plus `resultRelease`. The generated wrapper
  validates the byte products for both the C and Go element layouts against
  the target Go `int` before creating a slice or allocating Go storage. These
  layouts may differ for narrow C enums and enclosing PODs; an oversized
  product returns `ErrOwnedArrayTooLarge`, not an allocation panic. The wrapper
  copies and converts every element into an independent Go slice, and applies
  the same all-path release rules as `ownedBytes`. A non-null result is released
  once even if size validation fails; a native failure status takes precedence
  over output validation.
- Named C enums with generated Go named types and constants.
- POD structs passed and returned by value, including acyclic nested POD
  structs and enum fields. Conversion is field-by-field through the declared C
  type; pointer, string, array, union, and bit-field members are rejected.
- Tagged C union containers normalized into ordinary Go structs containing an
  integer or enum tag and one field per scalar, enum, or POD variant. Generated
  C helpers are the only code that reads or writes union members. Conversion
  reads only the active declared variant, supports multiple tag values for one
  variant, and preserves an unknown tag with all variant fields left at their
  zero values. The normal layout models an outer C struct whose tag coexists
  with its union payload and can therefore use scalar, enum, or POD variants.
  `"overlaidTag": true` models SDL-style unions where every variant overlays
  the tag at offset zero; it requires POD-struct variants and writes the active
  variant before restoring the tag. Tagged unions work as direct inputs/results
  and status/out results.
- Call-scoped C callbacks with scalar, enum, POD-struct, or normalized
  tagged-union value parameters and scalar, enum, or void results. POD and
  union arguments are converted into independent ordinary Go values before
  user code runs; unknown union tags are preserved with inactive fields zeroed.
  A callback manifest parameter expands to a C function pointer
  followed immediately by a `void *` context, while the generated public API
  exposes one typed Go function value. The wrapper uses an integer
  `runtime/cgo.Handle`, rejects nil with `ErrNilCallback`, deletes the handle on
  every return path, and never passes a Go pointer to C. Direct, status-only,
  and status/out functions are supported; callback-bearing functions always
  expose an error result.
- Copied callback inputs with explicit null contracts. `copiedCString` requires
  a non-null NUL-terminated `const char *` and exposes an independent Go `string`.
  `nullableCopiedCString` exposes `nil` for a null pointer or a pointer to an
  independent Go string otherwise. `copiedBytes` consumes `const uint8_t *`
  plus `size_t`, copies into an independent Go `[]byte`, and accepts null only with
  zero length; every zero-length value becomes a non-nil empty slice. A null
  required string, null/non-zero byte pair, or length exceeding the target Go
  `int` records `CallbackInputError`, skips user code, and returns the callback
  result type's zero value to C. The C pointer and claimed memory range must
  remain readable until the generated gateway finishes its copy.
- Transactional mutable callback buffers. `inoutBytes` consumes a writable
  `uint8_t *` plus `size_t`, performs the same checked copy-in as `copiedBytes`,
  and exposes only the Go copy to user code. If the callback returns normally,
  the gateway copies the final bytes back to C before returning—even when a
  boolean/numeric callback result is zero. A panic or input-contract failure
  performs no copy-back, so partial Go mutations do not leak into C memory.
  Zero length accepts null or non-null and exposes a non-nil empty slice. C must
  keep the claimed range writable until the gateway returns. Multiple
  `inoutBytes` parameters copy back in declaration order; overlapping ranges
  therefore use deterministic later-parameter-wins behavior and should usually
  be avoided.
- Typed callback arrays. `copiedArray` and `inoutArray` require an `element`
  naming a scalar, enum, or acyclic POD struct, and expose a Go `[]Element`.
  Each C argument expands to an element pointer followed by a `size_t` count.
  `copiedArray` takes a const pointer and never writes to it; `inoutArray` takes
  a writable pointer and copies back only after normal callback completion,
  including false, zero, and void results. Both lifetimes (`callScoped` and
  `registered`) are supported. Null is valid only with zero count; null and
  non-null empty inputs both produce a non-nil empty slice. Element counts and
  byte products are checked against the target Go `int` for both C and Go
  layouts before allocation or reading native memory. POD values are converted
  field-by-field instead of reinterpreting their layouts. The Go copy may be
  retained after the callback or registration ends. Panic or input-contract
  failure performs no copy-back. Multiple writable arguments are copied
  independently and written back in declaration order, so later arguments win
  when ranges overlap. The C library must keep the entire claimed input range
  readable (and writable for `inoutArray`) until the gateway returns. The count
  is fixed: appending or reslicing a callback's local slice does not resize the
  C buffer. These types are callback-input-only, not registration parameters,
  ordinary function parameters, or results.
- Registered callbacks declared with `"lifetime": "registered"` and an
  explicit `callbackRegistrations` entry naming status-returning register and
  unregister C symbols. The generated `Register<Name>` function returns a
  registration object with `Close` and `CallbackError`. Registration passes
  the same generated function/context pair to both C operations, rejects nil,
  and deletes the integer handle only after successful unregister and all
  admitted callback entries have finished. Registered callback declarations
  may use the same scalar, enum, POD-struct, and tagged-union arguments.
  Separately, each registration operation may
  declare fixed scalar, enum, POD-struct, or tagged-union value parameters.
  They precede the callback/context pair in both C calls and are copied into
  the registration object so unregister observes exactly the original values.
  A `retainedCString` or `retainedBytes` registration parameter explicitly
  transfers a copied registration-owned C allocation to the register call.
  The exact pointer (and byte length) is retained for unregister and is freed
  only after unregister succeeds and admitted callbacks drain. Embedded NUL in
  `retainedCString` is rejected; an empty `retainedBytes` value is represented
  by a null pointer and zero length. Register failure frees every temporary
  allocation, while unregister failure preserves them for retry.
- Registration-owned typed arrays. A `retainedArray` registration parameter
  requires a scalar, enum, or acyclic POD `element` and accepts `[]Element`.
  Register and unregister receive the same C element pointer and `size_t`
  count. The wrapper creates an independent C copy, converts POD fields rather
  than reinterpreting layouts, and retains the allocation until successful
  unregister and callback draining. Nil and empty inputs both use null/zero.
  C mutations never modify the caller's Go slice, and later Go mutations never
  modify the C copy; the retained count is fixed. C may access this storage
  while registered, but must neither free nor resize it and must stop all
  access before successful unregister returns. Concurrent native access is
  the library's synchronization responsibility. Size products are checked
  before allocation; `ErrRetainedArrayTooLarge` or
  `ErrRetainedArrayAllocation` is returned before native registration. A failure
  while preparing any retained input rolls back all earlier allocations;
  native register failure drains admitted callbacks before rollback.
  Unregister failure preserves every allocation for retry. This type is
  registration-input-only, not an ordinary call input or callback argument.
- C-owned byte results from registered callbacks. A callback result of
  `ownedBytes` exposes an ordinary Go `[]byte` callback and generates the C ABI
  `uint8_t *callback(..., size_t *output_length, void *context)`. Each non-empty
  result is copied into a new C allocation; an empty result is represented by a
  null pointer and zero length. Register and unregister both receive a paired
  generated release function, and C must call it exactly once for every
  non-null result after it has finished reading that allocation. A null
  `output_length` pointer records `CallbackInputError` for `$result` without
  invoking user code. A panic records `CallbackPanicError` and returns
  null/zero. This result contract is rejected for call-scoped callbacks because
  its allocation may outlive the callback entry.
- C-owned string results from registered callbacks. `ownedCString` exposes a Go
  `string` callback and generates `char *callback(..., void *context)` plus the
  same paired-release registration contract. Every valid result, including an
  empty string, is copied to a non-null NUL-terminated C allocation that C must
  release exactly once. An embedded NUL is rejected as `CallbackInputError` for
  `$result` instead of being silently truncated; a panic returns null. This
  result is likewise restricted to registered callbacks.
- C-owned typed-array results from registered callbacks. `ownedArray` requires
  a scalar, enum, or POD `resultElement` and exposes `[]T` in Go. Its C gateway
  returns `T *` plus `size_t *`, converts each element into a new contiguous C
  allocation, and supplies the paired release function to register and
  unregister. Empty arrays are null/zero. A null length pointer or an
  element-count/size product that exceeds the target address space becomes
  `CallbackInputError` for `$result`; panic and conversion failure transfer no
  allocation. Call-scoped arrays and pointer-bearing element types are rejected.
- Direct scalar, enum, POD-struct, tagged-union, string, and void calls.
- C `int32_t` status-only conversion to `error`, and status plus final
  out-parameter conversion to `(T, error)`.
- `threadSafe`, process-local `serialized`, `threadAffine`, and `mainThread`
  call policies.
  `threadAffine` lazily starts one dedicated goroutine, locks it to one OS
  thread, and synchronously routes every generated call and handle release
  through it.
- `mainThread` pins the Go startup goroutine during package initialization
  and rejects calls from other OS threads with `ErrWrongThread`, without
  dispatching them to another goroutine.
- Global and GOOS/GOARCH-specific cgo compiler and linker flags.
- Opaque pointer handles with an explicit C release symbol.
- Per-handle locking, nil/closed checks, single successful `Close`, and
  rejection of use after release. Ordinary functions may accept multiple
  handles: generated wrappers lock them in stable creation-ID order and lock
  the same object only once if it appears in multiple arguments.
- Native handles and callback registrations reject Go struct value copies
  with `ErrCopiedHandle` and `ErrCopiedCallbackRegistration`. Pointer aliases
  still share the original object, its lock, and its single close operation.

```json
{
  "schemaVersion": 1,
  "package": "imageffi",
  "header": "image.h",
  "threadPolicy": "serialized",
  "targets": [
    {"goos": "linux", "ldFlags": ["-limage"]},
    {"goos": "darwin", "ldFlags": ["-limage"]}
  ],
  "handles": [
    {"name": "Image", "cType": "image_handle", "release": "image_free"}
  ],
  "functions": [
    {
      "name": "ImageOpen",
      "symbol": "image_open",
      "parameters": [{"name": "id", "type": "int64"}],
      "result": "Image",
      "convention": "statusOut"
    }
  ]
}
```

Unknown fields, ambiguous machine-width types, unexported or malformed public
names, duplicate functions/parameters/types/fields/constants/handles, unsafe
header or flag text, recursive by-value structs, empty or ambiguous tagged
unions, unsupported union tags or variants, unsupported borrowed-array element
types, invalid callback lifetimes or
signatures, unsupported callback-array elements, unsupported conventions,
direct handle calls, and raw pointer/string/buffer callback parameters are
rejected before cgo generation. A generated
compile-time assertion rejects targets where a declared `cInt32` or `cUint32`
does not match a 32-bit C type. Ordinary functions and callback registrations
may accept multiple handles without a manifest lock-order declaration: the
wrapper assigns every created handle a unique ID and acquires multiple locks
in that order, locking repeated arguments only once.

Parameter names remain ordinary unique Go identifiers, not Go keywords.
Names such as `context`, `state`, `result`, `output`, `runtime`, and `int32`
are accepted: where they would shadow generated locals, imports, builtins, or
package declarations, the generated Go signature uses a deterministic internal
name instead. The generator also avoids collisions with user names resembling
its own helpers. Argument order and types do not change, and callback input
errors still report the parameter's original manifest name. Public type and
function names remain subject to the reserved-name checks above.

Keep opaque handles and callback registrations as pointers. Their constructors
record the original object's identity, and every use checks that identity
before locking or touching C state. A Go value copy such as `copy := *image`
is not another owner: `copy.Close()` and calls receiving `&copy` fail with
`ErrCopiedHandle`. Copied registrations likewise reject `Close` and
`CallbackError` with `ErrCopiedCallbackRegistration`, without unregistering or
deleting the original callback context. This protects against accidental
double release and copied mutexes; it does not make copying an object during
concurrent mutation safe or protect against arbitrary unsafe Go/C writes.
Mutable ownership and locks live in separate shared state, so even restoring
an earlier value snapshot to the original Go address cannot resurrect a closed
native handle or callback context.

### Libraries requiring the main thread

Use `"threadPolicy": "mainThread"` for libraries whose window or event APIs
must run on the executable's main OS thread. The generated package calls
`runtime.LockOSThread()` in `init`; Go guarantees that a normal executable's
`main` then runs on that startup thread. See the
[Go `LockOSThread` documentation](https://pkg.go.dev/runtime#LockOSThread).
Unlike `threadAffine`, this policy does not create a dedicated worker thread.

Each generated library call pins its goroutine before checking a C thread-local
marker and remains pinned until the native call and cleanup complete. A call
on another thread returns `ErrWrongThread` before allocating arguments,
registering callbacks, or touching native handles. Direct `void` functions
therefore return `error`, and direct value functions return `(T, error)`;
`status` and `statusOut` keep their existing error-returning signatures.
Handle and callback-registration `Close` use the same check, so a rejected
close leaves the resource available for subsequent use on the main thread.

Call these APIs from `main` or initialization, not a worker goroutine. The
generated code does not provide a main-thread event queue or dispatch callback
code to the main thread. Do not remove the initialization thread lock with
`runtime.UnlockOSThread`. Tests for main-thread APIs should run a separate
executable, because ordinary Go test functions execute in worker goroutines.
The C compiler must support C11 thread-local storage (or MSVC's thread-local
extension). Go shared-library/archive embedding is not covered by this policy:
its Go startup thread need not be the host application's main thread.

A `callScoped` callback is valid only until the C function returns. C must not
retain the function/context pair and must join every C thread using it before
returning. C may invoke it zero, one, many, or concurrently many times during
that interval, so concurrently invoked user callbacks must synchronize their
own captured mutable state. A Go panic is recovered in the generated gateway,
becomes the callback result type's zero value for C, prevents later callback entries from
calling user code, and is returned as `CallbackPanicError` after C unwinds. If
the C status also fails, callback failures take precedence. Schema 1 permits
multiple call-scoped callback parameters, including repeated callback types
and mixed signatures. Each argument has an independent context, failure state,
and runtime handle, even when the same Go function is passed in multiple slots.
All callback arguments are checked for nil before any runtime handles are
created, and every handle is deleted when the outer call returns. Each callback
parameter expands to its own adjacent C function-pointer/context pair, in
parameter declaration order; a different native ABI needs an adapting C shim.
Call-scoped callbacks are still not combined with owned, string, or handle
results; those lifetime combinations require a later explicit contract.

With one callback argument, the established direct `CallbackPanicError` or
`CallbackInputError` return is unchanged. With multiple arguments, each failed
slot contributes a `CallbackArgumentError` exposing `Function`, `Parameter`
(the original manifest argument name), and `Err`. It unwraps to that slot's
first panic/input error. The outer call returns `errors.Join` of those errors
in parameter declaration order, not C invocation order, so `errors.As` can
inspect both argument context and the underlying error. Failure in one slot
suppresses only that slot's later invocations; other callbacks keep running.
After any callback failure, the Go result is its zero value. A later outer call
starts with fresh callback state.

A registered callback remains valid until its registration object's successful
`Close`. Closing first rejects new user-code entries, calls the declared C
unregister function under the binding's thread policy, waits for already
admitted entries, and then deletes the integer handle. If C unregister returns
a failure status, the wrapper resumes the registration and permits a later
retry without freeing its context. C must guarantee that successful unregister
prevents every future callback and accounts for any callback already dispatched
before it returns. `CallbackError` preserves the first recovered panic and
later entries return the callback result's zero value without invoking user
code. Registrations require explicit `Close`; no finalizer guesses a safe C
thread or shutdown order.

Subscription-only manifests may omit `functions` or use `"functions": []`;
at least one function or callback registration is required. C may invoke a
callback while registering it, including an initial notification before the
registration is returned. The returned registration preserves any callback
failure through `CallbackError`. If C registration fails, C must not retain
the input pointers or callback context and must prevent future callbacks;
the wrapper drains admitted callbacks before freeing its temporary storage.

A registered `ownedBytes`, `ownedCString`, or `ownedArray` result has its own allocation
lifetime. Successful `Close` ends future callback entries but does not
invalidate or collect allocations that C received earlier. C may release them after
`Close`; the generated release bridge is package-lifetime C `free` code and
does not access the registration object, a cgo handle, or Go memory. The C
library must retain the paired release function together with each outstanding
allocation and release every non-null result exactly once. It must not release
a null empty-byte result; an empty owned string is non-null and must be
released. The binding does not track outstanding results or delay `Close` until
they are released.

For either callback lifetime, the first panic or copied-input contract failure
is retained. Call-scoped functions return it after the C call unwinds;
registered callbacks expose it through `CallbackError`, and later entries
return zero without invoking user code. `CallbackInputError` identifies the
outer function or registration, manifest parameter name, and rejected input
condition.

This includes `panic(nil)`, even when Go runs with `GODEBUG=panicnil=1` and
`recover` returns nil. Callback completion is tracked separately from the
recovered value: the wrapper still records `CallbackPanicError`, disables later
user-code entries, returns a zero C result, and does not copy back mutable
inputs. Owned byte/array results also have zero lengths on failure. Ordinary
input/result validation remains `CallbackInputError`, and a panic recovered
inside user code does not turn a successful callback into an error. The
`threadAffine` executor likewise preserves nil-valued panics rather than
mistaking them for normal completion.

A registration may take multiple opaque handle parameters, including handles
of different types. It holds one lifetime lease per distinct resource:
`Handle.Close` reports
`ErrHandleHasActiveRegistrations` until unregister succeeds and every admitted
callback has completed. Repeating the same handle argument does not add a
second lease. Failed registration leaves no leases; failed unregister preserves
the live callback and all leases for retry. Resource locks are released before
waiting for admitted Go callbacks to finish. The registration retains private
ownership state rather than mutable public wrappers, so replacing a wrapper
value does not change its retained C arguments. The registration does not own
the handles: callers still close each handle explicitly after closing all
coupled registrations.

`retainedCString` and `retainedBytes` are registration-parameter-only lifetime
types. They are not accepted as ordinary function parameters, callback
parameters, or results. C may read the retained allocation from successful
register until successful unregister returns, but must not access it after that
return. The registration owns these allocations; C must neither free nor
replace them. This makes the ownership boundary explicit instead of inferring
whether an ordinary borrowed string or slice might escape a call.

The generated package is the low-level private boundary. Public binding APIs
remain ordinary Kinmokusei code, so application code does not see `C.*`, raw
pointers, or release symbols.

## Distributing a C-backed Kinmokusei package

Use two versioned Go modules: a Kinmokusei source package and a private native
Go module. The native module contains the C headers and source files alongside
`generated_ffi.go` in the same Go package directory. The source package
declares the native module as a Go dependency and exposes only `.km` wrappers.
For example, a repository may have this layout:

```text
kinmokusei-raylib/
  go.mod                 # module example.com/kinmokusei-raylib
  kinmokusei.toml
  index.km
  native/
    go.mod               # module example.com/kinmokusei-raylib/native
    ffi/
      binding.json
      generated_ffi.go
      raylib_shim.h
      raylib_shim.c
```

The source package declares the native module and its cgo requirement:

```toml
[target]
cgo = "enabled"

[go.dependencies]
"example.com/kinmokusei-raylib/native" = "v0.1.0"
```

Its `.km` implementation can use `import go` privately and re-export a
Kinmokusei API. The consumer enables cgo for its target and adds only the
source package:

```toml
[target]
cgo = "enabled"
```

```sh
keika deps add example.com/kinmokusei-raylib@v0.1.0
keika check
keika run
```

Both modules must be available at their declared versions. The source package
dependency causes `keika deps add` to lock the native Go module; users do not
copy C sources, edit generated Go, or list the private Go module themselves.
After dependency resolution, check/build/run use the lock and cached sources
without updating dependencies. The target also needs a working C toolchain.

`keika ffi generate` is a package-author step: run it when preparing the native
module and distribute its output with the C files. Normal consumer builds do
not regenerate FFI code. For local development, ordinary `[go.replacements]`
are limited to directories inside the consuming project root; a sibling native
module cannot be referenced through `../` as a Go replacement. A separately
versioned Go module avoids that restriction for consumers.

## Proposed source declarations

```ts
ffi c library image {
  link linux dynamic "libimage.so";
  link darwin dynamic "libimage.dylib";

  opaque type Image release image_free;

  function image_load(
    data: borrowed bytes,
    length: uint64,
    out image: owned Image,
  ): status;
}
```

The exact source syntax remains a proposal. It will lower to the same checked
manifest model and private cgo package rather than introducing a second FFI
implementation.

## Ownership and lifetime

Proposed lifetime modes:

- `borrowed`: valid only during the call; the callee cannot retain it.
- `borrowedUntil(handle)`: valid while the named handle remains alive.
- `owned`: the receiver must release it with the declared release function.
- `retained`: the callee keeps it after return; use C memory or an explicit copy by default.
- `static`: valid for the library lifetime and never released by the caller.

Variable-size output should use one explicit pattern:

- Caller allocation after querying required size.
- Library allocation with a matching release function.
- A documented borrowed view valid until a specified event.

No generated wrapper may guess ownership from a pointer type or function name.

## Errors and panic

Incoming status codes and out parameters should convert into explicit result values. The FFI declaration must map success and known errors; unknown codes remain representable. C callbacks and wrappers must contain Go panics and never unwind through foreign frames.

## Callbacks and threads

- Never store raw Go function values or Go pointers in C.
- Use generated gateways and integer callback handles.
- Keep handles alive until explicit unregister completes.
- Define races between unregister and in-flight callbacks.
- Declare callback thread origin and reentrancy.
- Contain callback panic.

Generated call-scoped and registered gateways implement the applicable rules
with integer handles, typed scalar/enum/POD/tagged-union value calls, nil
rejection, copied string/byte/typed-array inputs with checked null/length
contracts, transactional mutable byte/typed-array buffers, panic containment, C-thread concurrency,
checked unregister, in-flight draining, and copied value parameters. Registered callbacks may
additionally couple their lifetime to multiple checked opaque handles and own copied
`retainedCString`/`retainedBytes`
parameters until successful unregister. Registered callbacks may also transfer
independent `ownedBytes` and `ownedCString` results to C through the paired
generated release function. `ownedArray` provides the equivalent contract for
scalar, enum, and POD slices.

Thread policies:

- `threadSafe`: calls may execute directly from multiple goroutines.
- `serialized`: a generated mutex permits one call at a time, without promising
  OS-thread identity.
- `threadAffine`: the implemented dedicated executor locks one goroutine to one
  OS thread and serializes calls and handle release there. It remains alive for
  the generated package lifetime.
- `mainThread`: calls and releases must originate on the startup/main thread;
  calls from any other thread fail with `ErrWrongThread` rather than being queued.

The schema 1 serialized and affine executors are deliberately non-reentrant. A
call-scoped callback can execute while either outer call is active, but it must
not synchronously call the same binding because that would wait on the already
held mutex or occupied executor and deadlock. Under `threadSafe` or `mainThread`,
callback code must still not reenter an operation using the same already-locked
opaque handle.
Additional reentrancy modes require a separate explicit contract.

A callback must not call `Close` on its own registration: a conforming C
unregister operation may wait for that callback to return, so self-close would
deadlock. With policies other than `mainThread`, close from another goroutine is
supported and waits for the callback to finish. Under `mainThread`, arrange to
close on the main thread after callback processing returns.

Blocking/cancellation semantics must also be declared; cancellation cannot be inferred for an arbitrary C function.

## ABI versioning

- Breaking changes increment ABI major and shared-library naming.
- Prefer adding symbols over changing/removing existing ones.
- Hide evolving layouts behind opaque handles.
- Add a new function rather than appending parameters to an existing symbol.
- Compare canonical ABI manifests in CI and classify symbol/type/gateway/status breaks.
- Declaration ordering and source aliases must not change the fingerprint.

## Implementation stages

### FFI 0: connection experiment

- Fixed-width integers.
- `const uint8_t*` plus length.
- Opaque handles and explicit release.
- Status plus out parameters.
- Linux dynamic linking.
- Library initialization/shutdown.
- Serialized call policy.

### FFI 1: practical wrappers

- String/byte/array conversion.
- Ownership validation.
- Safe class wrappers.
- Static and target-specific linking.
- ABI manifests and compatibility checks.

Borrowed strings and byte buffers, library-allocated owned strings, bytes, and
typed-array results, target-specific linking, enum/POD/tagged-union values,
status-only calls, safe opaque handle wrappers, and registration-owned retained
string/byte/typed-array inputs are implemented. General retained/static views and the full
ownership vocabulary remain in this stage.

### FFI 2: callbacks and concurrency

- Function-pointer gateways and callback handles.
- Unregister/in-flight behavior.
- Thread-safety policies.
- Concurrent goroutine calls.
- Blocking, cancellation, and reentrancy tests.

Typed call-scoped and explicitly registered callbacks with scalar/enum/POD/
tagged-union values, copied callback string/byte inputs, and
transactional `inoutBytes` buffers plus registration-owned string/byte
parameters, C-thread concurrent invocation, nil/input-contract/panic handling,
unregister failure/retry, in-flight Close, handle-coupled lifetime leases, and
thread-affine registration are implemented. Registered `ownedBytes`,
`ownedCString`, and scalar/enum/POD `ownedArray` callback results additionally
cover empty/non-empty transfer, panic, invalid length output, embedded NUL or
array-size overflow, paired release, and release after registration close. A
real Raylib-shaped load/unload shim verifies adaptation from a context-free
`int *bytesRead` callback pair to the checked context-bearing ABI. Explicit
reentrancy modes remain in this stage.

### FFI 3: extensions

- Restricted C header importer.
- Additional POD layout assertions and C-header-assisted discovery.
- ABI diff reports.
- Profile-guided copy reduction.
