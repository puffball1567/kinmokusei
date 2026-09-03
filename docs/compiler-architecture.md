# Compiler architecture

## Semantic compatibility invariant

For accepted constructs with a Go equivalent, OnsenTamago must preserve the
observable behavior of an independently handwritten Go implementation. This
includes values and errors, mutation, evaluation order/count, copy versus
aliasing, identity, nil behavior, panics, channel behavior, and concurrency.
Readable generated Go and successful Go validation are necessary checks, but
generated declarations must never serve as the expected-value implementation
in differential tests.

## Pipeline

The first compiler is implemented in Go to simplify single-binary distribution and integration with the generated Go toolchain.

```text
OnsenTamago source
      |
lexer / parser
      |
syntax tree
      |
module resolution ----------------> Go module graph / build target
      |                                      |
name resolution / type checking <---- Go package export data (go/types)
      |
typed OnsenTamago representation
      |
Go lowering
      |
Go AST / source emission ----> outgoing C ABI gateway / checked incoming C FFI package
      |                                      |
      +------------------+-------------------+
                         |
                 generated Go module
                         |
                 gofmt / go test / go build
```

## Frontend

### Lexing and parsing

- Accept only syntax that OnsenTamago actually supports; do not parse all TypeScript and reject it later.
- Recover after syntax errors so one malformed statement does not suppress the rest of the file.
- Preserve source spans on every AST node.
- Resolve lexical ambiguities in context. For example, `>>` is a shift in expressions and two generic closers in nested type syntax.
- Treat `type`, `alias`, and `distinct` as contextual declaration words so ordinary identifier positions remain source-compatible.
- Keep mutations as statements: compound assignment and `++`/`--` are not expressions.

### Name resolution

- Distinguish files, modules, and packages.
- Reject relative import cycles.
- Resolve public/private names before mapping them to Go capitalization.
- Manage Go keywords, predeclared identifiers, and generated-name collisions through deterministic mangling.
- Place relative imports and Go package aliases in the same file scope.
- Do not expose transitive relative imports; every reference must resolve to a local declaration or explicit import.
- Resolve Go members from toolchain type information, never from spelling or documentation text.
- Separate package loading from symbol support so one advanced unused export cannot reject an entire package.
- Preserve package-path identity independently of source aliases and checkout paths.

### Type checking

- Limit implicit conversion to safe untyped literal contexts.
- Keep Go defined types, aliases, pointers, method sets, interfaces, multiple results, variadics, channels, and type parameters intact.
- Use `go/types` assignability and generic constraints as authoritative for Go values. Exported standard and external Go interface type sets are valid native constraints; operator checking intersects embedded sets and requires every remaining term to support the operation.
- Represent native generic defined types with `go/types.Named`, infer `comparable` for parameters used as map keys, and re-instantiate named results after native generic substitution.
- Predeclare incomplete `go/types.Named` values for distinct types, complete them after resolving their underlyings, and permit recursive re-entry only beyond slice, map, pointer, function, or channel indirection. Keep direct, fixed-array-only, and alias cycles as source diagnostics.
- Lower native generic classes to pointer-backed generic Go structs with generic constructors and receiver methods; preserve concrete type arguments through member substitution, interface conformance, module linking, public Go APIs, and source-name JSON tags. Decoding into a constructor-created class leaves its unexported hierarchy identity and virtual-dispatch state intact.
- Attach non-generic defined-type receiver signatures to the same `go/types.Named` method set used for source checking, generated Go validation, and external interface compatibility.
- Validate lowered Go conformance when OnsenTamago classes/functions cross Go interface or callback boundaries.
- Preserve Go bitwise/shift groups, named-type identity, and left-operand result types.
- Diagnose negative/excessive constant shifts, constant integer zero divisors, and fixed-width constant overflow before Go generation.
- Keep compound assignment and `++`/`--` as dedicated statement nodes and lower them directly to Go `AssignStmt`/`IncDecStmt`; never duplicate a selector/index/pointer target.
- Diagnose nonassignable string indexes, temporary array indexes, methods, constants, and nonaddressable fields at the OnsenTamago source location. Map indexes remain assignable as in Go.
- Lower native struct `function` declarations to value receivers and contextual `pointer function` declarations to pointer receivers. Preserve Go automatic address/dereference selector rules, method-value capture, receiver-before-argument evaluation, and nil-pointer receiver behavior.
- Normalize top-level `function name(this: Struct, ...)` and `function name(this: *Struct, ...)` declarations into the same native method set as nested struct methods. Remove the receiver from callable parameters, and reject duplicate mixed-form declarations and receiver types not declared in the same source module.
- Nullable references use explicit nil-backed types, callable-local and stable class-member mutation facts, and checked constructor initialization. `Task<T>` uses explicit single-consumption control-flow state, and `Result<T>` is a checked return effect; neither is inferred from wrapper-like shapes.

## Typed representation and lowering

The compiler currently keeps typed AST metadata close to syntax nodes. A more explicit small typed IR may be introduced when transformations need it, but it must serve semantic clarity rather than optimization complexity.

Required typed information includes:

- Class-to-struct/method expansion.
- Native nominal-struct identity, complete-literal field resolution, and direct named-Go-struct lowering.
- Native generic-struct parameter scopes, explicit instantiation identity,
  cycle-safe recursive field/method substitution, and direct Go `any` type
  parameter/instantiated receiver lowering.
- Native generic-interface parameter scopes, explicit instantiation identity,
  substituted method contracts and member signatures, explicit class
  conformance, and direct Go generic-interface lowering.
- Native generic-class parameter scopes, explicit construction identity,
  substituted fields, constructors, methods, and interface contracts, plus
  pointer-backed generic Go class/constructor/receiver lowering.
- Native defined-type identity versus transparent alias identity, explicit conversion targets, finite recursive named-type graphs, cycle-safe Go type conversion, direct Go `TypeSpec` lowering, and Go 1.23-compatible use-site expansion of generic aliases.
- Closure captures.
- Multiple-result and `Result<T>` lowering metadata, including explicit split bindings and postfix `?` propagation.
- Typed-exception boundaries, terminal-flow metadata, `finally` unwinding, and structural cross-package exception markers that leave ordinary Go panics untouched.
- Structured task result shape, single-consumption state, and `await`/`detach` lowering metadata.
- Deterministic anonymous struct shapes for object literals.
- Source-to-generated position mapping.
- C FFI ownership, lifetime, thread-safety, and link requirements.
- Imported Go package identity, method sets, addressability, multiple results, and type arguments.

### Nullable flow facts

The target nullable analysis is a forward control-flow dataflow pass. Declared
types remain symbol metadata; per-program-point facts form a separate lattice
with at least maybe-null, definitely-null, and non-null states. Conditions and
assignments are transfer functions, reachable predecessor facts are joined by
intersection of guarantees, and loop headers are iterated to a fixed point.

Facts are keyed by declaration identity rather than spelling so shadowed names
do not interfere. Writes carry resolved target identities. Address-taking,
escaping aliases, nested mutable captures, and goroutine-visible mutation add
conservative invalidation edges. Member facts additionally require stable
storage identity and getter/override guarantees; otherwise callers must bind a
local snapshot. Unknown calls invalidate only facts for storage they can
actually reach, but the compiler must remain conservative when escape analysis
cannot prove that boundary.

The current implementation operates directly on structured AST control flow
and has explicit switch/select joins, conservative loop backedges, closure
escape information, and source-positioned invalidation diagnostics. If richer
fixed-point and member rules become duplicated across semantic features,
introduce a small CFG/typed IR rather than extending structured state ad hoc.
Generated Go is unchanged by this pass.

Outgoing fixed-width C ABI export separates ordinary Go implementation functions, cgo gateways, C headers, and canonical manifests. Implemented incoming C FFI uses a strictly validated manifest IR to generate an isolated cgo package for scalars, borrowed strings, enums, nested POD values, C-helper-normalized tagged unions, integer-handle call-scoped and registered callbacks with checked in-flight draining, status/out calls, and opaque handles. Future source syntax must lower to that IR rather than bypassing normal checking.

## Go package loading

Direct interop does not translate Go source into the OnsenTamago AST. The compiler asks the selected Go toolchain to load export data for the locked Go version, module graph, OS, architecture, build tags, and CGO configuration.

```text
OnsenTamago manifest + lock + target
                    |
          Go package/module loader
                    |
       package identity + export data
                    |
       lazy symbol/type resolution
                    |
              typed Go nodes
```

- Model standard and external packages uniformly.
- Ignore package implementation details; retain only public reachable type information.
- Do not require every export to be representable at import time.
- Use the Go toolchain for build selection, generated source, CGO, and export data.
- Cache by Go version, module graph, target, tags, and CGO settings.
- Follow Go rules for `internal`, semantic import versions, vendor, and `replace`.
- Keep dependency acquisition outside normal loading. Only explicit dependency commands may access or mutate the module graph.

## Go generation guarantees

Generated Go is a user-inspectable artifact and must:

- Be formatted by `gofmt`.
- Pass Go syntax and type validation after OnsenTamago checking.
- Build under ordinary `go build` and `go test`.
- Avoid unnecessary reflection and dynamic boxing.
- Preserve module structure where practical.
- Use deterministic generated names and declaration ordering.
- Point diagnostics back to OnsenTamago files and spans.
- Absorb Go-only restrictions such as unused locals without changing OnsenTamago semantics.
- Preserve expression evaluation count and Go runtime behavior.

Project builds use `.ontama/gen/` for intermediate modules. Explicit `emit-go` writes a consumable artifact to a user-selected location.

## CLI

The primary commands are:

```text
ontama version
ontama check [--json] <sources...>
ontama build <sources...>
ontama run <sources...>
ontama emit-go <sources...>
ontama emit-c-abi <sources...>
ontama abi check --baseline <manifest> <sources...>
ontama interop audit --stdlib [--json] [--allow-incomplete]
ontama install --go-module <module>@<version> [project]
ontama deps lock|check|add|remove|update|licenses
ontama lsp --stdio
```

Commands that check, build, run, or emit code must not implicitly resolve dependencies or update lockfiles.
`install --go-module` is a transactional user-facing alias for adding an exact
Go module dependency; it preserves the manifest, lock, and internal module files
byte-for-byte when resolution fails.

`check` runs the shared lexer, parser, relative-import loader, Go importer, and
semantic checker without generating an artifact or invoking `go build`. Plain
mode is silent on success and writes source-positioned diagnostics to standard
error. `--json` always writes one object to standard output:

```json
{
  "valid": false,
  "diagnostics": [
    {
      "message": "cannot use string as int",
      "path": "src/main.otm",
      "start": { "line": 3, "column": 10, "offset": 52 },
      "end": { "line": 3, "column": 17, "offset": 59 }
    }
  ]
}
```

Lines and columns are one-based, offsets are zero-based bytes, and ranges are
half-open. A source diagnostic leaves the optional top-level `error` absent; an
input, locked-project, or loader failure sets `error` and leaves `diagnostics`
empty. The exit status is 0 for valid source, 1 for invalid source or a checking
failure, and 2 for command-line misuse. This contract lets AI and editor tooling
distinguish repairable source locations from environment or project failures
without parsing prose.

## Language Server

The LSP is part of the `ontama` binary and uses the same lexer, parser, resolver, type checker, target loader, and diagnostics as the CLI.

Implemented capabilities include lifecycle handling, transactional incremental synchronization, monotonic document versions, multiple open documents, unsaved overlays, UTF-16 and CRLF-aware positions, diagnostics, hover, semantic definition and references, scope-safe value-symbol rename with `prepareRename`, document symbols, lexical completion, selective-import completion, Go package and value API completion, visibility-aware OnsenTamago class/struct/interface/object member completion, and signature help. Source member completion follows parameters, scoped locals, catch bindings, constructor and callable result inference, `this`, inheritance, static/instance separation, concrete generic class/struct/interface arguments, native internal and external receiver methods, and selected relative imports. Checked binding-type metadata lets Go value completion follow explicit or inferred locals, globals, multiple results, range and select bindings, pointers, named/alias types, promoted fields and methods, and Go addressable-value method sets. Go value method signature help uses the same selector rules and recovers from incomplete calls and unclosed source braces without mutating the document. Compiler-provided declarations such as `Exception`, `message`, and `error()` expose completion, hover, and constructor signatures without inventing a source definition location. Semantic requests run asynchronously against immutable document snapshots. `$/cancelRequest` produces the standard request-cancelled error, and a document change that overtakes an in-flight request suppresses its result with a content-modified error. Shutdown drains already accepted requests before responding.

Signature help uses semantically resolved callable types for complete programs and declaration/import fallbacks for temporarily incomplete edits. It covers OnsenTamago functions including inferred and explicitly instantiated native generics, native rest parameters, substituted generic-class constructors and class/struct/interface methods, defined-type receiver methods, function values and arrows, compiler built-ins, generic and variadic Go package functions, nested calls, and active-parameter tracking. Native function/class/struct/interface/defined-type parameters and native defined/alias declarations also participate in hover, definition, references, rename, symbols, and completion. Optional compiler-built-in parameters are marked with `?` in the presentation only; `?` is not declaration syntax.

Incremental changes are applied in notification order against the updated snapshot. Invalid ranges, split UTF-16 surrogate pairs, mismatched `rangeLength`, and stale versions leave the text, version, and diagnostics unchanged; a notification never commits only a valid prefix of its edits.

References and rename use declaration identity produced by semantic checking rather than identifier spelling. Rename covers functions, variables, parameters, local bindings, types, classes, fields, methods, selected relative imports, and Go package aliases, including aliases used as Go type qualifiers. Renaming an interface method updates the connected implementation family across every explicitly implementing class. It validates the edited overlay by recompiling it and verifies that every edited use still resolves to the corresponding renamed declaration, rejecting capture, collisions, and invalid programs. External Go declarations are intentionally read-only.

The official Visual Studio Code client is a thin adapter in `editors/vscode`. It contributes the `.otm` language identity and syntax grammar, launches `ontama lsp --stdio`, selects only real file documents, supports a configurable executable path, and serializes configuration/manual restarts. Startup failures remain visible and retryable instead of leaving a partially active client. A real Extension Development Host test builds the current server and exercises language identification, hover, live diagnostic publication and recovery, and restart behavior.

Local VSIX packaging includes production dependencies, excludes development-only inputs, fixes archive timestamps, and compares two independently generated archives byte for byte. Tagged releases attach the matching VSIX beside the `ontama` command archives; marketplace publication remains future release work. Other LSP-capable clients can launch `ontama lsp --stdio` directly.

The LSP must never acquire dependencies, update manifests, or mutate external state implicitly.

## Diagnostics

Diagnostics should identify the failed rule, expected type/value, actual type/value, and original source span. Important categories include:

- Lexical and syntax errors with recovery.
- Name and scope errors.
- Type and assignment mismatches.
- Nonaddressable or nonassignable mutation targets.
- Unsafe boundary violations.
- Invalid or unhandled `Result`/`Task` states, including copying, escape, double consumption, and path-dependent non-consumption.
- Import cycles and unavailable target-specific packages.
- Generated-name collisions.
- Invalid public Go or C ABI boundaries.

Generated-Go validation is a final invariant check, not the first place ordinary source errors should appear.

## Reproducibility

- Record compiler and target Go versions in build information.
- Never depend on filesystem enumeration order.
- Restrict network access to explicit dependency resolution.
- Reconstruct the same graph and generated source from the same canonical lock and target.
- Keep aliases and checkout paths out of type identity.

## Implementation track

Completed foundations include:

1. Generated-Go syntax/type validation.
2. File-scoped modules, selective imports, and stable link names.
3. Standard-library constants/functions/basic collections.
4. Go named types, aliases, variables, structs, fields, pointers, `nil`, selectors, and method sets.
5. Multiple results, destructuring, blank bindings, reassignment, and raw `error`.
6. Variadics, expansion, callbacks, function/method values, fixed arrays, slicing, named collections, collection built-ins, explicit copy/view conversions, range, bitwise/shift, and update statements.
7. Go interfaces, explicit class conformance, assertions, and type switches.
8. Existing/read-only module graphs, local replacements, strict manifests, canonical locks, dependency commands, and license hashes.
9. Generic function inference/explicit arguments and generic named types/methods.
10. Directional channels, send/receive, checked receive, close, range, select, goroutines, and defer.
11. Locked target settings, cross-build, cross-run preflight, and CGO-aware loading diagnostics.
12. Default-deny unsafe policy and supported `unsafe` built-in lowering.
13. Minimum/current Go toolchain CI across Linux, macOS, and Windows, plus
    generated direct-interop cross-builds for Linux AMD64/ARM64, macOS ARM64,
    and Windows AMD64.
14. `Result<T>`/`Result<void>` return effects, explicit `ok`/`fail`, and postfix `?`
    propagation for OnsenTamago results and direct Go error-returning APIs.
15. Nil-backed `T | null`, a dedicated `null` literal, unsafe-use diagnostics,
    and immutable-local branch/guard narrowing composed with Result lowering.
16. Stable mutable-local/parameter nullable narrowing plus definite
    straight-line and `if/else` initialization of non-null reference fields.
17. Separate declared/observed local types, assignment-sensitive nullable
    transfer, reachable `if`/switch/select joins, conservative loop backedge
    merging, re-narrowing, and source-positioned assignment invalidation
    diagnostics.
18. Declaration-identity and program-point-sensitive local escape analysis for
    address-taking and nested mutable arrow captures, including shadow-safe
    capture propagation and nullable pointer-to-slot dereference typing.
19. Definite non-null constructor-field joins across nested blocks, `if/else`,
    value/type switches, and `select`, with explicit unmatched-switch paths and
    source-ordered `break`/terminal handling.
20. Definite constructor initialization through guaranteed loops:
    conditionless or compile-time-true boolean/integer/string conditions, and
    range over nonempty array/constant-string expressions, positive-length
    fixed arrays and pointers, provably nonempty `append`, or positive-length
    constant `makeSlice`, with source-ordered `break`/`continue` exits. The
    semantic pass resolves local, `for`-initializer, same-file global, and
    explicitly imported immutable `const` chains while declaration scopes and
    module visibility are available, then records the guaranteed-entry fact on
    the loop AST for constructor flow.
21. Non-escaping `Task<T>` bindings with exactly-once `await` or `detach`,
    path-sensitive branch/loop joins, eager callee/argument evaluation, ordinary
    and `Result` task shapes, and panic transport across the task boundary.

The next null-safety step deepens the implemented local dataflow with
call-in-place contracts, broader alias invalidation, and stable-field rules. The
current pass accepts safe accesses before a later direct write, address, or
mutable-capturing arrow expression, rejects accesses reachable after that
invalidation point, and iterates loop headers across fallthrough and `continue`
backedges while joining `break` exits separately.

Next work should deepen public external-module fixtures, generated-Go
publication fixtures, target-specific assembly, IDE refactoring features, task
cancellation/context propagation, and broader constructor cardinality proofs.
New functionality must extend parser,
semantic, generated-Go, independently handwritten-Go differential,
diagnostics, and fuzz matrices together. Public generated artifacts must also
be consumable from an external ordinary Go package without the OnsenTamago
compiler or machine-local build state.
