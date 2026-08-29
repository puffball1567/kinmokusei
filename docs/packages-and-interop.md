# Packages and interoperability

## Policy

OnsenTamago uses three explicit package paths:

1. Relative imports between OnsenTamago modules.
2. Versioned source-only OnsenTamago packages hosted in GitHub repositories
   (future installation work).
3. Namespaced direct Go imports through `import go`.

Direct Go interop is the low-level ecosystem boundary. Higher-level OnsenTamago wrappers should improve ergonomics without hiding the underlying Go package or changing values implicitly.

OnsenTamago packages distribute `.otm` source, manifests, licenses, and optional
C/C++ shim source rather than prebuilt library binaries. They compile into the
consumer's ordinary generated Go module. The initial package source is GitHub;
there is no dedicated binary registry or requirement for a central package
server. A future optional catalog may map short names to canonical GitHub
repositories without becoming the source of package contents.

This makes OnsenTamago a source dialect in the Go ecosystem rather than a
parallel runtime or binary ecosystem. It is not Go-source-compatible: the
compiler owns `.otm` parsing and semantics, while the completed dependency and
build graph is an ordinary Go module. Direct Go modules continue to be fetched
as source by Go tooling, exposed to OnsenTamago through export/type information,
compiled by the selected Go toolchain, and reused from Go's build cache.

## Project manifest and lock

The project files are `ontama.toml` and canonical JSON `ontama.lock`.

```toml
[project]
name = "example"
version = "0.1.0"
source = "."
go-module = "example.com/example"

[target]
goos = "linux"
goarch = "amd64"
cgo = "disabled"
tags = "production"

[go.interop]
unsafe = "deny"

[go.dependencies]
"github.com/google/uuid" = "v1.6.0"

[go.replacements]
"example.com/local/api" = "./local-api"
```

Rules:

- The four `[project]` keys are required; unknown/duplicate sections and keys are rejected.
- `[target]` accepts only `goos`, `goarch`, `cgo`, and `tags`. CGO is `auto`, `enabled`, or `disabled`; tags are unique and canonicalized in sorted order.
- Omitted GOOS/GOARCH use the compiler host, not ambient process overrides. Cross-target `auto` CGO becomes disabled.
- `[go.interop].unsafe` is `deny` by default and may be explicitly `allow` project-wide.
- Dependencies use complete versions, including complete pseudo-versions.
- Initial replacements are project-relative local paths inside the project root and must correspond to dependencies.
- The lock records manifest hash, Go version, resolved target/tags/CGO, module graph/checksums, project-relative replacements, generated `go.mod`/`go.sum` hashes, and license file hashes. It contains no machine-specific absolute paths.
- Dependency locking parses the project's `.otm` files, collects explicit
  `import go` paths, and lets the selected Go toolchain materialize the required
  indirect module graph. The resulting locked module builds with
  `-mod=readonly`; comments, ordinary strings, non-source files, and generated
  `.ontama` state cannot fabricate dependencies.

Only explicit dependency mutation commands (`install --go-module`, `deps add`,
`deps update`, `deps remove`, and `deps lock`) may resolve the graph and update
`.ontama/deps/` or the lock. `--offline` uses `GOPROXY=off` and existing caches.

Explicit commands:

- `ontama install --go-module <module>@<version> [project]`
- `ontama deps add <module>@<version> [project]`
- `ontama deps update <module>@<version> [project]`
- `ontama deps remove <module> [project]`
- `ontama deps lock [--offline] [project]`
- `ontama deps check [project]`
- `ontama deps licenses [--strict] [project]`
- `ontama target [project]`

`install --go-module` is the user-facing spelling for adding a direct Go module;
it uses the same transactional implementation as `deps add`. The latter remains
available as the lower-level dependency command. Both accept `--offline` and a
project-local `--replace` for development and reproducible fixtures.

Add/update/remove are transactional: failed resolution keeps the previous manifest, lock, and locked state. Normal check/build/run/emit/LSP paths only validate and use the locked graph read-only and offline.

The lock's target controls export loading, checking, and building. Cross-build is allowed; `run` rejects a non-host target before execution. Ambient GOOS/GOARCH/CGO variables cannot override locked values.

## Import syntax

```ts
import { User, findUser } from "./users";
import { Router } from "ontama/http";
import go http from "net/http";
```

- Each `.otm` file has an independent module scope.
- Only local declarations and explicitly imported names are visible.
- Transitive imports do not leak names.
- Import aliases, imported names, and local declarations share collision checks.
- Multiple root files never share names implicitly.
- Go symbols are always referenced through their alias namespace.
- `ontama/*` is reserved for standard/compiler-managed packages. The exact
  `ontama/http` package is implemented and embedded in the compiler; unknown,
  differently cased, traversal-like, or otherwise noncanonical `ontama/*`
  paths are rejected rather than normalized or fetched.

Go package internals may use reflection, unsafe, assembly, generated code, or CGO if the selected Go target can build them. Only public boundary types affect OnsenTamago source compatibility.

## Direct Go interop model

The compiler loads package export data from the selected Go toolchain and preserves:

- Basic and untyped constants.
- Defined types and aliases.
- Pointers, arrays, slices, maps, structs, field tags, interfaces.
- Function signatures, multiple results, variadics, callbacks.
- Channels and directions.
- Type parameters, constraints, and instantiated types.
- Package-path-based identity.
- Method sets and addressability.

Current connectivity includes package constants/functions/variables, all Go basic types, named/alias/anonymous structs, fields/tags, pointers, nil, methods, explicit conversions, multiple results, raw errors, variadics, function/method values, callbacks, interfaces, assertions, type switches, generics, named collections, Go-compatible `clear`/`min`/`max` and other collection built-ins, channels, select, defer, goroutines, target-aware packages, CGO loading, and explicit unsafe operations.

### Compatibility principles

- Package import success is separate from individual symbol support.
- Unsupported unused exports never reject an entire package.
- Check only referenced symbols and reachable call/type shapes lazily.
- Never collapse `time.Duration` into a plain `int64` or otherwise erase identity.
- Use `go/types` for assignability, method sets, addressability, constants, and generic constraints where possible.
- Never silently convert `(T, error)` to `Result<T>`, erase Go pointer identity when forming nullable types, or convert Go interfaces to class hierarchies. Postfix `?` is the explicit bridge from `(T, error)` or `error` into an enclosing `Result` function. `null` is the checked OnsenTamago literal; `nil` retains raw Go semantics.
- Keep generated code directly connected to the original Go package without reflection proxies.
- Record environmental availability in the locked target.

## Public API audit

`ontama interop audit --stdlib` inspects actual export data for package declarations, direct fields, and value/pointer methods.

Classifications:

- `supported`: lossless under the default safe policy.
- `requires_unsafe`: usable only when `unsafe = "allow"` because `unsafe.Pointer` appears publicly.
- `unsupported`: a reachable public type shape is not representable.
- package load failures: reported separately and never counted as support.

`--json` emits the full machine-readable inventory. The audit measures public API type connectivity, not implementation coverage of all Go syntax or runtime behavior.

## Low-level syntax correspondence

Examples:

```ts
import go strconv from "strconv";
import go strings from "strings";

const [value, err] = strconv.Atoi("42");
const reader = strings.NewReader("text") as! *strings.Reader;
const [checked, ok] = reader as? *strings.Reader;
```

Higher-level propagation remains explicit:

```ts
function parse(text: string): Result<int> {
  const value = strconv.Atoi(text)?;
  return ok(value);
}
```

The direct destructuring form keeps the raw `error` available to the caller;
the postfix `?` form checks it and returns it from the enclosing Result function.

Go interface type switches use explicit case-local types:

```ts
switch (value) {
  case const reader as *strings.Reader { use(reader); }
  case nil { handleNil(); }
  default { handleOther(); }
}
```

- The subject is a Go interface and executes once.
- Typed cases require `const`/`let`, one binding (or `_`), `as`, and one type.
- Duplicate types, `nil`, or `default` are errors.
- Case bindings are scoped to mandatory blocks.
- Typed nil enters its pointer/interface case; `case nil` matches a nil interface.
- Overlapping interfaces are tested in source order with no fallthrough.

Raw channel select preserves Go semantics:

```ts
select {
  case const value = <-input { consume(value); }
  case let [value, open] = <-channel { inspect(value, open); }
  case output <- value { sent(); }
  default { idle(); }
}
```

Receive cases may discard, declare one/checked bindings, or assign existing targets. Sends/receives evaluate operands with Go's count and order. Each case has its own block scope; there is no fallthrough. Nil/closed channels, send-on-closed panic, readiness selection, blocking, and empty-select behavior match Go.

## Unsafe boundary

`unsafe = "deny"` still permits packages that use unsafe internally without exposing `unsafe.Pointer`. Explicit allow is required for direct `unsafe` imports or any used public signature containing unsafe pointers, including nested collections/callbacks/generics.

Supported special built-ins have dedicated type rules rather than guessed function signatures:

- `Sizeof`, `Alignof`, `Offsetof` -> `uintptr`
- `Add` -> `unsafe.Pointer`
- `Slice` -> slice of pointer element
- `SliceData` -> element pointer
- `String` -> `string`
- `StringData` -> `*byte`

They are not first-class function values. `Offsetof` accepts a Go struct field selector and rejects invalid pointer-embedding paths. Named pointer/slice underlying types determine element types. Nil, alias, lifetime, GC reachability, pointer arithmetic, and panic behavior remain exactly as unsafe Go; enabling the capability does not make them safe.

## Environment-dependent packages

- Pure Go packages are the primary compatibility target.
- Reflection, unsafe, assembly, and code generation inside a package are not rejection reasons.
- CGO packages work when the selected target has the required C toolchain/libraries.
- OS/architecture packages and build tags are checked for the locked target.
- Go toolchain limitations such as plugins are inherited.
- Public CGO-specific/unsafe types require explicit boundary handling.

## Licensing

The lock records deterministic hashes for recognized root license files (`LICENSE`, `LICENCE`, `COPYING` families). `deps licenses` lists direct/transitive modules, multiple files, and unknowns; strict mode fails on unknown. `NOTICE` alone is not treated as a license, and the compiler does not infer SPDX identifiers from prose or claim legal compliance. Modified/missing files are lock mismatches.

## Diagnostics

Interop diagnostics include:

- Unresolvable package/module.
- Target/build-constraint/CGO unavailability.
- Missing or unexported symbol.
- Defined-type assignment mismatch.
- Pointer, nil, addressability, and assignability violations.
- Method-set/interface mismatch.
- Multi-value used as one value.
- Variadic expansion or generic constraint mismatch.
- Unsafe policy violation.
- Stale/tampered manifest, lock, module files, or licenses.

## Go-facing output and handwritten integration

Generated packages should expose clean Go APIs where their OnsenTamago types have stable Go representations. Advanced wrappers may be written in OnsenTamago on top of direct interop or in ordinary Go beside generated packages. Handwritten Go remains a supported integration path for optimizations, adapters, and APIs not yet expressible safely.

C ABI/cgo integration is a separate explicit boundary described in [c-ffi.md](c-ffi.md); direct Go interop must not disguise C ownership or lifetime concerns.
