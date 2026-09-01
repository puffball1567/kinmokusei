# C FFI

OnsenTamago supports two explicit C boundaries.

## Exporting a C ABI

Mark only the functions that should become stable C symbols:

```ts
function add(left: int32, right: int32): int32 { return left + right; }
function subtract(left: int32, right: int32): int32 { return left - right; }

export c("ontama_add", "ontama_subtract") { add, subtract };
```

`ontama emit-c-abi` generates a Go gateway, C header, canonical manifest, and fingerprint. The gateway normalizes booleans, uses fixed-width values, isolates panics, and reports status explicitly.

## Calling C libraries

`ontama ffi generate` consumes a checked manifest and generates an isolated cgo package. The manifest makes ownership, release callbacks, handle lifetime, callback lifetime, and thread policy explicit.

Complex C or C++ APIs should expose a small C shim with stable structs, fixed signatures, and opaque handles. The shim is a library-specific adapter; it is not generated Go used as its own test oracle.

See the repository's detailed [C FFI design](https://github.com/puffball1567/onsentamago/blob/main/docs/c-ffi.md) for the schema and safety matrix.
