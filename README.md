# OnsenTamago

OnsenTamago is a programming language for writing web backends and Go libraries with TypeScript-inspired syntax and compiling them to readable Go source. The CLI is named `ontama`, and source files use the `.otm` extension.

OnsenTamago is a TypeScript-inspired source dialect for the Go ecosystem. It is
neither TypeScript-compatible nor Go-source-compatible: its source packages are
distributed as `.otm`, then compile into ordinary readable Go modules and use
the Go toolchain, module graph, ABI, runtime, and library ecosystem directly.

The project is currently a pre-1.0 public preview. See
[docs/index.md](docs/index.md) for the full design documentation.

## Core principles

- Use TypeScript-inspired static syntax while adding explicit language-specific constructs where needed.
- Treat `number` as an alias of `float`, while keeping `int` as a separate integer type.
- Emit human-readable, deterministic Go as an intermediate artifact.
- Keep emitted Go standalone and suitable for ordinary Go repository publication; authors may distribute `.otm`, generated Go, or both.
- Target web backends, Go libraries, and goroutine-based workloads first.
- Provide classes, encapsulation, interfaces, composition, and reference semantics explicitly.
- Lower object-oriented features to Go structs, methods, and interfaces.
- Allow OnsenTamago packages to import one another.
- Use `import go` as an explicit low-level escape hatch into the Go ecosystem.
- Build safer, idiomatic OnsenTamago libraries on top of direct Go interop.
- Keep the compiler and `ontama lsp --stdio` in the same versioned distribution.
- Preserve the Go ABI internally and generate a checked C ABI gateway only for explicitly exported functions.

## Currently implemented

- `boolean`, `string`, machine-width `int`/`uint`, fixed-width `int8`/`int16`/`int32`/`int64` and `byte`/`uint16`/`uint32`/`uint64`, the Go-compatible `uint8` alias, floating-point types, and `void`.
- Identical `number`, `float`, and `float64` types.
- Typed functions, top-level generic functions, native generic structs, interfaces, and defined types, arrow functions, function types, and local type inference. Native parameters support the explicit TypeScript-shaped `T extends comparable` constraint across functions and named generic types. TypeScript-shaped rest declarations such as `...values: int[]` work across functions, methods, interfaces, arrows, and constructors, with individual or spread calls and Go-compatible variadic output. Native generic calls support inference plus TypeScript-shaped `<T>` and Go-shaped `[T]` explicit or partial type arguments; generic named-type instantiation is explicit.
- Reference-type classes, including generic classes with explicit instantiation and `any` or `comparable` parameters, constructors, `this`, `new`, public/private/protected visibility, instance/static methods, generic static inference and explicit type arguments, idiomatic public static Go APIs without private/protected leakage, and explicit single inheritance with `extends`, `virtual`, `override`, `final`, `super`, multi-level dispatch, nil/identity-preserving implicit upcasts, checked/forced class downcasts, and public Go hierarchy conversion APIs. Generic classes currently use composition and generic interfaces rather than inheritance or virtual dispatch.
- Nominal value-type `struct` declarations, including generic structs with `any` or explicit `comparable` parameters, with complete named literals, Go-compatible assignment/argument/return copying, explicit pointer sharing, nested or external value/pointer receiver methods, method values, shallow reference-field copying, comparability, recursive indirection, and relative-module linking.
- Native nominal defined types with `type Name = distinct T`, including generic definitions such as `type Values<T> = distinct T[]` and finite recursive definitions through slice, map, pointer, function, or channel indirection, transparent non-generic aliases with `alias Name = T`, explicit Go-compatible conversions, inferred `comparable` constraints for map keys, untyped-constant assignment, collection/operator support, generic and non-generic value/pointer receiver methods, method values, and relative-module linking.
- Native integer enums with optional fixed-width underlying types, automatic and explicit constant values, namespaced members such as `Status.Pending`, explicit numeric conversion, switches, map keys, generics, relative-module linking, and ordinary generated-Go type/constant APIs.
- Interfaces, explicit `implements`, and interface-based polymorphism.
- `const`, `let`, simple and compound assignment, statement-only `++`/`--`, `return`, and `if`/`else`.
- `while`, C-style `for`, collection/channel `for-of` range, `break`, `continue`, labels, labeled branches, and `goto`.
- Slices `T[]`, fixed arrays `[N]T`, expected-type literals, indexing, two/three-index slicing, and `Map<K, V>`.
- `len`, `cap`, `append`, `copy`, `delete`, `clear`, `min`, `max`, `makeSlice`, `makeMap`, and explicit copy/view slice-to-array conversion.
- Explicit structural object types, object literals, member access, deterministic anonymous Go structs, and JSON tags.
- Arithmetic, comparison, logical, bitwise, shift, address, and pointer-dereference operators.
- Relative imports, cycle detection, deterministic module loading, file-scoped bindings, and selective imports.
- Namespaced Go interop for standard and external modules: constants, functions, variables, all Go basic types, named/alias/anonymous types, structs, fields/tags, pointers, `nil`, methods, multiple results, raw `error`, and explicit conversions.
- Go-compatible explicit conversion to `string` from byte slices, rune slices, named Go byte/rune slices, and integers.
- Go callbacks, function/method values, and explicit class conformance to Go interfaces.
- Go generic function inference, partial/full explicit type arguments, constraints, generic named types, and methods.
- Ordinary value switches with grouped cases, plus checked (`as?`) and unchecked (`as!`) Go interface assertions and explicit type-binding switches.
- `defer`, raw goroutine `go` statements, directional channels, send/receive, checked receive, close, range, and `select`.
- Structured `Task<T>` values from `go` expressions, single-consumption `await`/`detach`, `Task<Result<T>>` propagation, eager call evaluation, and task-boundary panic transport.
- Read-only existing Go module graphs, local `replace`, offline checks, strict `ontama.toml`, canonical `ontama.lock`, and dependency/license commands.
- Locked GOOS, GOARCH, CGO, and build tags; target-aware checking; cross-build; and preflight rejection of cross-run.
- Default-deny `unsafe` interop with explicit project policy and typed lowering for supported `unsafe` built-ins.
- Local destructuring of multiple results, blank bindings, multiple-result reassignment, and Go-compatible checked map lookups with `[value, present]`.
- Explicit `Result<T>`/`Result<void>` returns with `ok`, `fail`, postfix `?` propagation, and explicit split bindings across both OnsenTamago results and Go `(T, error)` APIs.
- A built-in extensible `Exception`, ordered typed `catch` clauses, and return-safe `try`/`catch`/`finally`, isolated from ordinary Go/runtime panics.
- Nil-backed `T | null` reference types with a dedicated `null` literal, separation from raw Go `nil`, checked nullable operations, assignment-sensitive local flow narrowing and joins, and definite constructor initialization for non-null reference fields.
- Source-positioned lexical, syntax, and type diagnostics.
- `version`, `check` with text or machine-readable JSON diagnostics, plus `build`, `run`, `emit-go`, `emit-c-abi`, checked incoming `ffi generate`, ABI compatibility checks, transactional `install --go-module` and dependency commands, and `interop audit`.
- An embedded source-written `ontama/http` kernel with bounded context-aware fetch, a Go `ServeMux`-compatible `App`, method routes, path/query/header/context/cookie access, direct `http.Handler` use, and structured-task compatibility.
- A 76/76 implemented Go-equivalent runtime contract registry backed by isolated handwritten-Go differential tests; new accepted runtime features must extend the registry and oracle together.
- Explicit fixed-width scalar and native-enum plus normalized-boolean C ABI exports with status/out parameters, panic isolation, headers, canonical manifests, and SHA-256 fingerprints; incoming C FFI generation supports fixed/C-width scalars, borrowed strings/byte buffers, copied library-owned strings, bytes, and typed-array results with mandatory release, enums, nested POD structs, normalized tagged unions, panic-contained call-scoped and explicitly registered callbacks carrying scalar/enum/POD/tagged-union values, checked copied string/byte inputs, transactional mutable byte buffers, and registered C-owned string/byte/scalar/enum/POD-array results with paired release callbacks, plus optional handle-coupled lifetime leases and registration-owned retained string/byte inputs, target link flags, status and status/out errors, serialized or OS-thread-affine calls, opaque handles, checked release, and a tested Raylib-shaped load/unload shim pattern.
- `gofmt`-formatted Go AST output, generated-Go validation, and `.ontama/gen/` modules.
- LSP lifecycle, transactional incremental document synchronization, stale-version suppression, asynchronous semantic requests, explicit request cancellation, content-modified result suppression, UTF-16 diagnostics, hover, semantic definition/references, scope-safe rename for values, types, enums and their members, classes, native structs and their literal fields, methods, interface implementation families, and import aliases, document symbols, lexical/import/Go package and value API completion, visibility-aware OnsenTamago enum/class/struct/interface member completion, and signature help for OnsenTamago callables, constructors, compiler built-ins, Go package functions, and Go value methods.
- An official thin Visual Studio Code client with `.otm` syntax highlighting, configurable `ontama` discovery, serialized restart behavior, visible retryable startup failures, real Extension Host end-to-end coverage, and reproducible local VSIX packaging.

Not yet implemented include generic aliases under the minimum Go target, native constraint type sets beyond `comparable`, generic class inheritance/virtual members, distinct definitions over native classes/structs/interfaces, general OnsenTamago package distribution, remaining general retained/static FFI data ownership policies, broader constructor cardinality proofs for unknown collections, and automatic task cancellation/context inheritance. Stable class-member nullable flow is implemented with conservative invalidation across aliases, writes, addresses, closures, and unknown calls. Boolean/integer/string constant expressions and local, `for`-initializer, same-file global, or explicitly imported `const` chains can prove guaranteed constructor-loop entry; provably nonempty array/string/fixed-array/`append`/`makeSlice` ranges participate in the same check. A JSON API using direct `net/http` and `encoding/json` interop already compiles and runs.

## Editor integration

The official Visual Studio Code client lives in
[`editors/vscode`](editors/vscode/README.md). It is intentionally a small
launcher around `ontama lsp --stdio`, keeping all semantic behavior in the same
versioned binary as the compiler. Other LSP-capable editors can invoke that
command directly for `.otm` files.

## Go interop

Go packages are imported through an explicit namespace:

```ts
import go strings from "strings";

function normalize(value: string): string {
  return strings.ToUpper(strings.TrimSpace(value));
}
```

The compiler uses the current standard library, a discoverable existing `go.mod`, or a module graph locked by `ontama.toml` and `ontama.lock`. Only explicit dependency operations may resolve or update dependencies; normal check/build/run/code-generation paths validate the locked graph read-only and offline.

`ontama interop audit --stdlib` inspects actual Go export data and classifies package-level declarations, direct fields, and value/pointer method sets under safe-default and unsafe-enabled policies. The result measures public API type connectivity, not coverage of the entire Go syntax. Use `--json` for the complete machine-readable report.

```ts
import go uuid from "github.com/google/uuid";

function newID(): string {
  return uuid.NewString();
}
```

Variadic calls accept individual values or explicit slice expansion with `values...`. Fixed array length is part of type identity, and fixed arrays never implicitly convert to slices. Slicing preserves Go storage aliasing and panic behavior. `copyArray[[N]T](slice)` produces an independent value; `viewArray[[N]T](slice)` produces a pointer view over the same backing storage.

Range syntax deliberately binds values in its single-binding form:

```ts
for (const value of values) { consume(value); }
for (const [index, value] of values) { consumeAt(index, value); }
for (const [key, value] of table) { consumeEntry(key, value); }
```

String range yields an `int` UTF-8 byte offset and an `int32` Unicode code point. Map iteration order is unspecified. Range sources are evaluated once, and range values are copies; mutate original elements explicitly through an index or key.

Integer bitwise and shift operators lower directly to Go and preserve Go operator groups and named-type identity. Unary address `&value` and binary `&` are distinct syntax contexts. Nested generic closers and `>>` are resolved by parser context. Negative constant shifts, excessive constant shifts, fixed-width constant overflow, and constant integer division by zero are diagnosed at the OnsenTamago source location. Dynamic negative shifts and zero divisors retain Go runtime panic behavior.

Compound assignment lowers directly to Go assignment operators rather than duplicating the target expression. Indexed and selected targets therefore execute once. `++` and `--` are statements and never produce values.

Unsupported APIs and operations are rejected at their OnsenTamago use site instead of being silently passed through to generated Go.

## Try it

Go 1.23 through Go 1.27 are supported. The released compiler is built with Go
1.27 so it can read package export data produced by every supported toolchain.

Install the released `ontama` binary and matching Visual Studio Code extension
using the [installation guide](docs/installation.md). Developers with Go already
installed can install the tagged command directly:

```sh
go install github.com/puffball1567/onsentamago/cmd/ontama@v0.1.0
ontama version
```

The following commands run from a source checkout:

```sh
go run ./cmd/ontama check examples/basic/main.otm
go run ./cmd/ontama check --json examples/basic/main.otm
go run ./cmd/ontama emit-go examples/basic/main.otm
go run ./cmd/ontama emit-c-abi -o ./generated-c-abi examples/c-abi/library.otm
go run ./cmd/ontama abi check --baseline ./previous/ontama_abi.json examples/c-abi/library.otm
go run ./cmd/ontama interop audit --stdlib
go test ./...
./scripts/coverage.sh
```

`check` performs lexical, syntax, import, and type/semantic validation without
generating Go or running the program. It exits with status 0 for a valid input,
1 for invalid source or a project/input error, and 2 for invalid command usage.
`--json` writes a stable report to standard output, including half-open source
ranges with one-based lines/columns and zero-based byte offsets, so editors and
AI coding tools do not need to parse human-readable diagnostics.

## Full-stack example

[`examples/react-web-frameworks`](examples/react-web-frameworks/README.md)
contains a React and TypeScript frontend with interchangeable Gin and Fiber
backends written in OnsenTamago. Both backends use the real external Go
framework packages, expose the same CRUD API, compile to ordinary Go, and are
checked against independently handwritten Go implementations under the race
detector. The React state transitions have their own automated coverage gate.

## Compatibility checks

OnsenTamago's [quality and Go compatibility policy](docs/quality-and-go-compatibility.md)
requires every implemented Go-equivalent runtime contract to match an
independently handwritten Go program. The compatibility workflow builds and
tests the compiler with Go 1.27 on Linux, macOS, and Windows; race detection
runs on Linux. Generated
direct-Go-interop programs are also cross-built with CGO disabled for
`linux/amd64`, `linux/arm64`, `darwin/arm64`, and `windows/amd64`.

The compiler reads Go package export data through the Go toolchain it was built
with. When installing from source, build `ontama` with the newest Go version it
will target. A future Go minor release may require a corresponding OnsenTamago
release before its packages can be imported.

`scripts/coverage.sh` measures repository-wide Go statement coverage with
cross-package instrumentation. It enforces both the current 87.0% repository
floor and an 80.0% floor for every implementation package, preventing a strong
area from hiding a weak one. This is separate from the 100% independent-Go
contract gate: statement coverage measures executed implementation statements,
while contract coverage proves that every registered Go-equivalent runtime
behavior has a handwritten oracle.

## License

OnsenTamago is available under the [Apache License 2.0](LICENSE).
