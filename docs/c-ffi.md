# C ABI and FFI

## Purpose

The C ABI is a deliberate stable boundary for two directions:

1. **Outgoing export**: expose selected OnsenTamago functions to C, Nim, Rust, and other C ABI consumers.
2. **Incoming FFI**: call C/Nim libraries through generated ownership-aware, type-safe wrappers.

Incoming bindings are also a distribution feature: a binding project should
be able to present an idiomatic OnsenTamago API while keeping cgo, raw pointers,
platform link flags, ownership, and release rules private. This makes a real C
library binding a primary preview/demo target rather than treating FFI only as
compiler plumbing.

The internal program remains ordinary Go. C-compatible gateways are generated only for explicit declarations.

## Implemented outgoing boundary

```ts
function add(left: int32, right: int32): int32 {
  return left + right;
}

const sub = (left: int32, right: int32): int32 => left - right;

export c("ontama_add", "ontama_sub") {add, sub};
```

Symbols and source names are paired by position: `"ontama_add"` exports
`add`, and `"ontama_sub"` exports `sub`. Their counts must match. The source
target must be a top-level function or a top-level `const` arrow with an
explicit result type. Lists may be split across lines and may use trailing
commas:

```ts
export c(
  "ontama_add",
  "ontama_sub",
) {
  add,
  sub,
};
```

The original single-function inline spelling remains supported:

```ts
export c("ontama_add") function add(left: int32, right: int32): int32 {
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
- `1`: contained panic in OnsenTamago/Go code.
- `2`: invalid boundary argument such as a null out pointer.

Panics never cross the C boundary. An out value is unspecified on failure.

## Stable initial types

| OnsenTamago | C |
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
by the OnsenTamago enum; unknown representable values are transported without
implicit validation, matching Go named-integer behavior.

Rejected from the initial stable boundary:

- Machine-width C `long`, Go/OnsenTamago `int`/`uint`, Nim `int`, and native
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

`ontama ffi generate --manifest <binding.json> -o <private-package>`
validates a schema-versioned manifest and emits `generated_ffi.go`, an isolated
cgo package intended to be consumed through ordinary Go interop and wrapped by
an idiomatic OnsenTamago module. Schema 1 supports:

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
- Library-allocated byte results. An `ownedBytes` status/out result requires a
  `resultRelease` C symbol and consumes `uint8_t **` plus `size_t *` outputs.
  The wrapper copies into an independent Go `[]byte` and calls the declared
  release function on success, status failure, null/length inconsistency, and
  an output length too large for the target Go address space whenever the C
  function returned a non-null allocation.
- Library-allocated typed arrays. An `ownedArray` result requires a scalar,
  enum, or POD `resultElement` plus `resultRelease`. The generated wrapper
  validates the element-count/address-space product, copies and converts every
  element into an independent Go slice, and applies the same all-path release
  rules as `ownedBytes`.
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
- `threadSafe`, process-local `serialized`, and `threadAffine` call policies.
  `threadAffine` lazily starts one dedicated goroutine, locks it to one OS
  thread, and synchronously routes every generated call and handle release
  through it.
- Global and GOOS/GOARCH-specific cgo compiler and linker flags.
- Opaque pointer handles with an explicit C release symbol.
- Per-handle locking, nil/closed checks, single successful `Close`, and
  rejection of use after release.

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
unions, unsupported union tags or variants, invalid callback lifetimes or
signatures, unsupported conventions, direct handle calls, and multiple handle
or pointer/string/buffer callback parameters are rejected before cgo
generation. A generated compile-time assertion rejects targets where a declared
`cInt32` or `cUint32` does not match a 32-bit C type. The one-handle restriction
gives schema 1 an unambiguous lock order; a later schema may add declared
multi-handle ordering.

A `callScoped` callback is valid only until the C function returns. C must not
retain the function/context pair and must join every C thread using it before
returning. C may invoke it zero, one, many, or concurrently many times during
that interval, so concurrently invoked user callbacks must synchronize their
own captured mutable state. A Go panic is recovered in the generated gateway,
becomes the callback result type's zero value for C, prevents later callback entries from
calling user code, and is returned as `CallbackPanicError` after C unwinds. If
the C status also fails, the callback panic takes precedence. Schema 1 permits
at most one callback parameter and does not combine it with owned, string, or
handle results; those lifetime combinations require a later explicit contract.

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

A registration may take at most one opaque handle parameter. The generated
registration holds a lifetime lease on that handle: `Handle.Close` reports
`ErrHandleHasActiveRegistrations` until unregister succeeds and every admitted
callback has completed. An unregister failure preserves both the live callback
and the lease for retry. The registration does not own the handle, so callers
still close the handle explicitly after closing all coupled registrations.

`retainedCString` and `retainedBytes` are registration-parameter-only lifetime
types. They are not accepted as ordinary function parameters, callback
parameters, or results. C may read the retained allocation from successful
register until successful unregister returns, but must not access it after that
return. The registration owns these allocations; C must neither free nor
replace them. This makes the ownership boundary explicit instead of inferring
whether an ordinary borrowed string or slice might escape a call.

The generated package is the low-level private boundary. Public binding APIs
remain ordinary OnsenTamago code, so application code does not see `C.*`, raw
pointers, or release symbols.

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
rejection, copied string/byte inputs with checked null/length contracts,
transactional mutable byte buffers, panic containment, C-thread concurrency,
checked unregister, in-flight draining, and copied value parameters. Registered callbacks may
additionally couple their lifetime to one checked opaque handle and own copied
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

The schema 1 serialized and affine executors are deliberately non-reentrant. A
call-scoped callback can execute while either outer call is active, but it must
not synchronously call the same binding because that would wait on the already
held mutex or occupied executor and deadlock. Under `threadSafe`, callback code
must still not reenter an operation using the same already-locked opaque handle.
Additional reentrancy modes require a separate explicit contract.

A callback must not call `Close` on its own registration: a conforming C
unregister operation may wait for that callback to return, so self-close would
deadlock. Close from another goroutine is supported and waits for the callback
to finish.

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
string/byte inputs are implemented. General retained/static views and the full
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
