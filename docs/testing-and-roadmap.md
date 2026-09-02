# Testing policy and roadmap

## Testing is part of the specification

OnsenTamago is developed with generated code and AI-assisted implementation, so automated tests are a required correctness boundary, not a final cleanup task. A feature is incomplete until its accepted behavior, rejected behavior, generated form, and runtime behavior are covered at the appropriate layers.

Every new feature should consider these axes:

| Axis | Examples |
|---|---|
| Normal | minimum case, ordinary case, feature combinations |
| Boundary | empty, zero, one, fixed-width limits, large input, nil |
| Failure | syntax, type, generated validation, runtime panic, external failure |
| State | initial, active, completed, cancelled, closed |
| Concurrency | isolated, simultaneous, reversed ordering, race, timeout |
| Public boundary | OnsenTamago API, generated Go, HTTP, C ABI |

Expected results must be explicit enough for a reviewer to validate independently. A generated test list is not itself a specification.

## Independent Go differential oracle

For every accepted feature that has a direct Go semantic equivalent, the
runtime oracle is an independently handwritten Go implementation. The expected
side must not call generated functions, construct generated classes, reuse
generated types, or derive expected values by inspecting generated output.
Generated-source assertions remain useful structural checks, but they are not
runtime oracles.

Compiler differential tests place the handwritten implementation in a separate
Go package and reject imports from the generated module. Each matrix compares
all relevant observable behavior: returned values and errors, mutation and
side-effect order/count, copy versus alias behavior, reference identity,
nil/null behavior, panic presence, and channel/concurrency results. When Go
semantics intentionally permit more than one result, such as a `select` with
multiple ready cases, compare the independently defined outcome set or
invariant instead of requiring two nondeterministic executions to make the same
choice.

Language-specific rejection diagnostics have no executable Go equivalent;
those remain explicit source/diagnostic matrices. Hard-coded constants may
supplement a differential test for readability, but do not replace the
independent Go oracle when an equivalent Go program exists.

The handwritten Go reference expresses the specified OnsenTamago semantics,
not merely the shortest visually similar Go syntax. If a Go compiler intrinsic
has toolchain-dependent operand evaluation, the reference must store operands
in the specified order before invoking it. This remains independent because it
is authored from the language rule and never copied from or linked to generated
output.

## Go-equivalent contract coverage

The compiler maintains an explicit registry of every implemented, accepted
runtime contract that has a direct Go equivalent. Coverage is complete only
when each registered contract is connected to an isolated handwritten-Go
differential scenario. The current registry covers 75 of 75 contract groups
(100%), including core language behavior, collections, nullability, results,
control flow, concurrency, standard/external Go interop, locked targets, CGO,
unsafe operations, string conversion, native generic functions, classes, structs, and interfaces,
native defined types and aliases, native integer enums, the HTTP/JSON
dogfood application, and the bounded fetch adapter.

An automated gate discovers every differential scenario in the compiler tests,
requires it to be classified in the registry, rejects a registered scenario
without a test, and rejects direct generated `go test` runners outside the
isolating harness. The harness requires the comparison test to import the
separate reference package and rejects any reference package import from the
generated module. Existing locked modules, build tags, target environments,
CGO, and offline module graphs use the same isolation rule.

This 100% figure is contract coverage for the implemented Go-equivalent runtime
surface, not line coverage and not a claim that all future Go or OnsenTamago
features exist. Language-specific rejection diagnostics, CLI/LSP behavior, C
ABI behavior, and other intentionally non-equivalent boundaries retain their
own matrices. Adding an accepted runtime feature requires adding its contract,
independent reference, normal/boundary/failure cases, and observable-behavior
comparisons in the same change; otherwise the feature is incomplete.

The race gate runs every race-capable differential module. A deliberately
locked `CGO_ENABLED=0` target cannot use Go's race runtime and therefore keeps
its ordinary differential execution while the outer compiler test remains
race-enabled. Likewise, the documented `unsafe.Slice(nil, nonzero)` and
`unsafe.String(nil, nonzero)` panic cases run in the normal and compatibility
toolchain matrices because race/checkptr converts them from recoverable panics
into unrecoverable process failures; the rest of the unsafe matrix remains
race-enabled.

## Test layers

1. **Token/lexer**: every token, longest-match boundaries, Unicode positions, malformed input, EOF/span invariants, fuzzing.
2. **Parser/AST**: valid shapes, precedence, contextual ambiguity, recovery, missing delimiters/expressions/types, fuzzing.
3. **Semantic checking**: type identity, assignability, scope, mutability, addressability, constants, diagnostics and spans.
4. **Lowering/code generation**: deterministic Go AST, names, imports, direct constructs, `gofmt`, Go syntax/type validation.
5. **Compile-and-run**: independent handwritten-Go differential results for observable values, alias/copy behavior, evaluation count/order, panic behavior, and real Go APIs.
6. **Project integration**: modules, targets, locks, CGO, unsafe policy, CLI, LSP, C ABI, HTTP dogfood.
7. **Global gates**: full normal suite, full race suite, `go vet`, lexer fuzz, parser fuzz.
8. **Publishable artifact**: standalone external-package consumption,
   deterministic formatting, portable module metadata, no machine-local paths,
   ordinary Go tests/vet/builds, and public API shape checks.

Repository-wide Go statement coverage is measured separately with
`scripts/coverage.sh`. It instruments every project package, prints the exact
total, rejects repository-wide regressions below 87.0%, and requires every
implementation package to remain at or above 80.0%. The full-stack React
component has its own statement, branch, function, and line thresholds; none of
these percentages replaces the independent-Go runtime contract gate.

## Current regression matrix

| Area | Required normal coverage | Required boundary/failure coverage | Required integration |
|---|---|---|---|
| Source/token/AST | spans, all keywords/operators, every node category | empty/different paths, near-keywords, invalid characters | positions flow into parser and diagnostics |
| Lexer | identifiers, numbers, strings, comments, Unicode, all operators | longest-match groups such as `&`/`&&`/`&=`/`&^=`, shifts/assignments, escapes, unterminated input | arbitrary input never panics and returns valid EOF/spans |
| Parser | declarations, types, expressions, arrows, control, OOP, collections, interop | missing tokens, invalid targets, nested generic `>>`, statement-only updates, recovery | arbitrary input never panics and later declarations recover |
| Predictability | value/reference, copy/alias, mutation, evaluation order/count, failure path | hidden alias, duplicate evaluation, unsupported fallback | generated form and runtime result tested together |
| Types/operators | all built-ins including `uint`, `int8`/`int16`, `uint8`/`byte`, and `uint16`/`uint32`/`uint64`, native nominal `type Name = distinct T`, generic and finite recursive defined types, transparent non-generic `alias Name = T`, conversions, defined-type value/pointer receiver methods, arithmetic/comparison/logical/bitwise/shift, all compound updates, `++`/`--`, Go defined numbers | implicit nominal/signed/unsigned/width conversion, mixed types, infinite-size or alias declaration cycles, invalid underlyings/operands/targets, alias/generic/pointer-underlying receiver rejection, pointer-method addressability, generic arity/constraint failures, conversion arity, const/nonaddressable mutation, fixed-width constant overflow, negative unsigned constant, zero divisor, negative/excessive shift | independent-Go exact results across scalars/slices/maps/fixed arrays/generic and recursive definitions/method values/nil receivers/Results/linked modules/external APIs, copy/alias behavior, machine/fixed-width overflow behavior, alias identity, target-once evaluation, Go flags/API, dynamic panic |
| Names/functions | globals/locals, inference, shadowing, forward references, parameters/results, top-level generic function, class, struct, interface, and defined-type parameters, explicit `extends comparable`, inferred/explicit/partial generic calls, explicit generic named-type instantiation | duplicate/reserved/unscoped/uninferred type parameters, inconsistent inference, invalid/unsatisfied constraints and explicit types, uninstantiated/wrong-arity generic types and values, incompatible instantiations, const mutation, arity, missing return | top-level, linked-module, constrained/unconstrained generic calls and generic class/struct/interface/defined-type external APIs compile and match independent Go |
| Control flow | conditionals, loops, all `for` clauses, range forms, value/type switches, branches, labels, labeled break/continue, forward/backward `goto` | nonboolean conditions, invalid range, incompatible/non-comparable/duplicate cases, scope/binding errors, duplicate/unused/undefined/non-enclosing/wrong-target labels, context-invalid branches | source-once range/switch, ordered case evaluation, value/reference comparison, index mutation, nil/empty, Unicode, named collections, independent-Go labeled control transfer |
| Functions/arrows | expression/block bodies, annotations, callbacks, function/method values, native rest parameters on functions/generic functions/methods/interfaces/arrows/constructors | void or malformed/non-final rest parameters, signature mismatch, variadic arity/element/spread mismatch, fallthrough, invalid call controls | Go function literals/callbacks and independent variadic functions, methods, constructors, interfaces, and virtual forwarding execute |
| Results/errors | `Result<T>`, `Result<void>`, `ok`, `fail`, postfix `?`, explicit split bindings, direct forwarding, raw Go error bridge | invalid placement/type/arity, forbidden storage/nesting, implicit raw conversion | generated `(T, error)`/`error`, success, zero-value failure, propagation |
| Exceptions | built-in `Exception`, typed ordered catches, `throw`, bare rethrow, `try`, `finally`, nesting, return-through-finally, publishable Go API | non-error throws/catches, unreachable catch order, invalid bare rethrow scope, outer-target break/continue, pending tasks | independent-Go typed dispatch/rethrow, external consumer API, return/finally ordering, Result returns, nil errors, terminal flow, concurrency, and ordinary panic preservation |
| Structured tasks | ordinary/void/`Result` calls, direct and bound tasks, nested-expression `await`, explicit `detach`, eager callee/argument evaluation, concurrent start | non-call start, raw Go multi-result, copy/reassignment/capture/escape, double/maybe/unconsumed paths, invalid await/detach, generated-name collision | independent-Go value/error/panic results, completion ordering, concurrency, detach completion, and race execution |
| Null safety | nil-backed `T | null`, `null`, assignment/equality, immutable/stable-mutable branch and guard narrowing, definite non-null field initialization | non-nilable bases, raw `nil` mixing, unsafe member/call/index/slice/channel use, mutation/address/capture/alias invalidation, branch joins, loop backedges, incomplete constructor paths and early return | generated nil representation, present/absent runtime paths, pre-write acceptance/post-write rejection, re-narrowing, Result composition, initialized class runtime paths |
| Collections | slices, fixed arrays, maps, strings, indexes/slices/range, `len`/`cap`/`append`/`copy`/`delete`/`clear`/`min`/`max`/make built-ins, object types | length/type/bounds/key/addressability errors, incompatible ordered operands, conversion length panic, literal field errors | independent-Go values/evaluation/panic, real standard APIs, JSON tags/API dogfood, alias/reallocation/copy/nil behavior |
| Classes/interfaces | constructors, fields, methods, visibility, static, explicit conformance, dispatch, generic class/interface declarations and explicit instantiations, generic static inference and explicit/partial calls, generic inheritance and virtual dispatch with concrete/remapped/multi-level bases | duplicates, privacy, generated static-name collisions, static/instance mismatch, missing/signature mismatch, implicit conformance rejection, missing/excess/uninferred/invalid generic arguments, constraint failures, incompatible instantiations, invalid overrides/final methods, and incompatible hierarchy conversions | generated generic structs/constructors/methods/interfaces/static functions and inherited base state, substituted fields/methods/interfaces, construction-phase-safe virtual dispatch, reference identity, exact-target upcasts/downcasts, method values, multiple instantiations, linked modules, and external-Go public/static/hierarchy API use without private visibility leakage |
| Native structs | nominal and unconstrained generic declarations, explicit instantiation, substituted fields/methods, complete named literals, fields, assignment/argument/return copy, explicit pointers, nested/fixed-array/recursive-indirection shapes, comparability, relative imports | duplicate/missing/extra/mistyped fields/type parameters, missing/excess/invalid type arguments, incompatible instantiations, nonaddressable writes, non-comparable equality/map keys, value recursion, `new`/nullable misuse | generated named/generic Go structs and independent-Go copy/alias/method-value/evaluation-order/runtime matrices, linked modules, and external Go consumers |
| Imports/modules | selective/transitive behavior, deterministic roots, aliases | missing files/names, cycles, duplicate binding, undeclared transitive access | root order/duplication independent generated output |
| Go interop | basic/named/alias/anonymous types, fields/tags, pointers, nil, methods, variables, multiple results, callbacks, interfaces, assertions, generics, channels | missing/unexported APIs, identity mismatch, constraint failure, direction mismatch, unsupported/unsafe signatures | real stdlib/external fixtures and public API audit |
| Targets/dependencies | locked graph, replace, target settings, `install --go-module`, lower-level dependency commands, license hashes | malformed/inexact install input, duplicate modules, stale/tampered locks, target/cgo mismatch, transactional rollback, unknown/missing licenses | offline install/import, reproducibility, cross-build, target-specific fixture |
| Unsafe policy | explicit allow, pointers/conversions/built-ins | default denial, nested/public exposure, invalid arity/types/overflow | generated Go and panic matrix |
| C ABI | all fixed-width scalars, normalized boolean, void, symbols, gateway/header/manifest/fingerprint | nonzero boolean inputs, normalized boolean outputs, invalid/reserved/duplicate symbols, unsupported types, null out, panic, compatibility changes | build shared library and run a C caller |
| Incoming C FFI | fixed/C-width scalars, bool, borrowed strings/byte buffers, released-and-copied owned C strings and byte/scalar/enum/POD arrays, enums, nested POD structs, normalized tagged unions, call-scoped and explicitly registered callbacks carrying scalar/enum/POD/tagged-union values, copied string/byte inputs, transactional mutable byte buffers, registered C-owned string/byte/scalar/enum/POD-array results with paired release, registration-owned retained strings/byte buffers, and one optional handle lifetime lease, status and status/out, opaque handles, target flags, thread-safe/serialized/OS-thread-affine policies | malformed/injected manifests, embedded NUL/null result, owned null success, allocated empty string, C failure after allocation, empty/binary buffers, null/nonzero length, target-int byte/product overflow, unsupported array element, missing release, width mismatch, duplicate/recursive POD declarations, empty/duplicate/unsupported union declarations, invalid overlaid scalar variants, unknown union tags, invalid callback lifetime/signature/registration/combinations, call-scoped owned callback result, copied/inout null/length violations, null owned-result length output, embedded NUL owned callback result, missing/unsupported/unexpected callback result element, owned callback array-size overflow, unsupported callback pointer types, duplicate/borrowed/multiple-handle registration parameters, retained/copied/inout-type misuse, nil callback, callback panic, registration capacity failure, unregister failure/retry, active-registration handle close, double/nil close, closed/nil handles | generate cgo, compile real C fixtures and a Raylib-shaped context-free load/unload shim, count string/buffer/array releases on every path, execute external-tag scalar/enum payloads and SDL-shaped overlaid POD payloads, execute zero/one/many/concurrent C-thread callbacks with bool/void/enum and owned-string/byte/scalar/enum/POD-array results and scalar/enum/POD/tagged-union/copied/inout inputs through direct/status/status-out and thread-affine calls, prove callback input copies survive C-side mutation, prove inout copy-back on normal true/false/void completion and no copy-back on panic/input failure, exercise required/nullable strings, empty/binary/repeated buffers and oversized/null-invalid inputs, verify owned callback null/zero, embedded-NUL, and array-size failure, exact-once release, and delayed release after registration close, adapt `size_t *` plus context to `int *bytesRead` without context, multiple registered subscribers, unknown callback union tags, enum routing, POD-filter value copying, retained-input copy isolation/exact-pointer unregister/failure retry/empty values, handle-coupled register/fire/unregister and retry, panic/input-failure disabling, in-flight Close draining, post-close silence, direct Go and high-level OnsenTamago wrappers, compare C allocation/release/register thread identity, concurrent/race calls |
| LSP | lifecycle, transactional incremental synchronization, overlays, asynchronous requests, cancellation, content-modified suppression, UTF-16/CRLF diagnostics, hover including compiler-provided exception declarations, native generic signatures, and native type declarations, semantic definition/references, value/type/type-parameter/class/member/interface-family rename, symbols, lexical/import/Go package and Go value completion, generic type-parameter and native type completion, substituted source class/struct/interface member completion, callable/constructor/built-in/Go package function/Go value method signature help | malformed protocol/request IDs/cancellation, invalid/surrogate-splitting ranges, mismatched lengths, stale versions/results, invalid names, shadowing/capture/collisions, comments/strings, closed scopes, external Go declarations, source-less built-ins, static/instance and private/protected boundaries, unselected imports, unexported/ambiguous Go members, fields used as methods, incomplete and nested calls | CLI diagnostic consistency, cross-file overlay rename/signature help, target-aware module fixtures, shutdown ordering, and race coverage |
| HTTP dogfood | direct `net/http` and `encoding/json`, object DTOs, `ontama/http` App/Context routing | malformed JSON, method/path/status behavior, route conflicts, missing cookies, cancellation/timeout where relevant | compile/run/race against an in-process server, independent-Go method/path/query/header/cookie matrices, external Go handler use, and concurrent requests |

## Go interop compatibility method

Compatibility is driven by actual Go type information, not package allowlists.

- Use small synthetic packages to cover type shapes and real packages to validate toolchain integration.
- Treat standard and external packages with the same model.
- Preserve identity across source aliases, root ordering, and checkout paths.
- Test Go version, OS, architecture, tags, and CGO as independent target axes.
- Separate network-enabled dependency resolution from offline reproducible builds.
- Detect mismatch between generated `go.mod`/`go.sum` and the canonical lock.
- Test single, multiple, unknown, missing, misleading, modified, symlinked, and oversized license files.
- Run generated output through ordinary `go test`, and use `-race`/`go vet` where applicable.

`ontama interop audit --stdlib` is a repeatable coverage measurement for public API type connectivity. It must report safe-default, unsafe-enabled, unsupported, and package-load failures separately; an unreadable package is never counted as supported.

Real Go-package connectivity is a release property, not a collection of
allowlisted demos. Fixtures must exercise standard packages and external
modules through their actual exported types, generics, method sets, callbacks,
contexts, errors, target tags, CGO requirements, and module locks. A package
that loads but loses type identity or cannot cross a public generated API is not
counted as usable.

## Generated-Go publication gate

Generated output must remain useful outside an OnsenTamago checkout. Tests
should increasingly compile it from an external Go package, consume its public
API, run its own isolated Go tests, and scan generated module/source metadata
for machine-specific paths. Deterministic emission, `gofmt`, `go test`,
`go vet`, supported-target builds, and absence of an unnecessary runtime
dependency are mandatory properties.

Performance is tracked separately from semantic equality. Representative
benchmarks should compare allocations, startup, binary size, and throughput
against a handwritten Go implementation of the same behavior. Small explained
lowering costs may be accepted; accidental reflection, boxing, duplicate
evaluation, goroutine leaks, or allocation regressions are failures. Benchmarks
must never replace result/error/race differential tests.

The checked toolchain matrix tests source compatibility with Go 1.23, Go 1.26,
and Go 1.27. Release binaries are built with Go 1.27 and exercised against each
supported Go toolchain because direct package import uses versioned Go export
data. Go 1.27 runs the full suite on Linux, macOS, and Windows; Linux
additionally runs the race suite.
The generated-artifact matrix cross-builds a direct standard-library interop
program for Linux AMD64/ARM64, macOS ARM64, and Windows AMD64 with CGO disabled.
Build tags, target-specific external fixtures, locked CGO state, ambient-target
override rejection, and cross-run rejection remain separate tests.

## Release gates

Progress should be reported by evidence-backed gates, not one ambiguous percentage:

1. **Language gate**: practical application types, control flow, errors, and modules are expressible without escape Go.
2. **Interop gate**: locked standard/external packages check, build, and run for the selected target.
3. **IDE gate**: diagnostics, navigation, completion, and safe rename work immediately after installation.
4. **Operations gate**: HTTP lifecycle, shutdown, timeout, logging, configuration, race, and leak behavior are covered.
5. **Distribution gate**: multi-platform artifacts, installation/upgrade, compatibility, and release procedures are reproducible.

A gate completes only when normal, boundary, failure, generated-artifact, and runtime tests relevant to it pass.

## Roadmap

### Phase 0: executable specification

- Maintain representative syntax examples and expected Go forms.
- Resolve type/null/error/task questions with small executable cases.
- Keep a minimal complete web API as a design target.

Completion criterion: major syntax can be explained without guessing its output or failure path.

### Phase 1: minimal compiler

- Lexer, parser, AST, variables, functions, basic types, control flow.
- Name resolution, type checking, Go generation.
- `check`, `build`, `run`, and compile-and-run tests.

Core items are implemented and continuously extended.

### Phase 1.5: immediately usable LSP

Implemented: overlay frontend, same-binary `lsp --stdio`, lifecycle, transactional incremental document synchronization, monotonic version handling, multiple open documents, CLI-equivalent diagnostics, asynchronous semantic requests, explicit request cancellation, stale-result suppression, UTF-16/CRLF conversion, hover, semantic definition/references, scope-safe rename for values, types, classes, members, interface implementation families, and import aliases, symbols, local/import/Go completion, signature help for OnsenTamago and Go callables, and a thin official Visual Studio Code client with tested configuration, startup failure, retry, restart ordering, lifecycle behavior, real Extension Host coverage, and reproducible local VSIX generation.

### Phase 2: practical language and OOP foundation

- Classes, visibility, constructors, static members.
- Interfaces, composition, polymorphism.
- OnsenTamago module/package distribution.
- `Result<T>` and explicit error propagation. (implemented)
- Core nil-backed nullable references, assignment-sensitive local flow narrowing/joins, and definite constructor field initialization. (implemented)
- Object DTOs and public Go API generation.
- Nominal Go-style value structs, complete literals, copy/pointer/comparability semantics, nested and external value/pointer receiver methods, method values, module linking, and editor navigation. (implemented)
- Native nominal defined types and transparent non-generic aliases with
  conversions, generic defined-type declarations, inferred map-key constraints,
  finite recursive slice/map/pointer/function/channel declarations,
  collection/operator semantics, module linking, independent-Go comparison,
  generic and non-generic value/pointer receiver methods, method values, module linking,
  independent-Go comparison, and editor navigation. (implemented; generic
  aliases under the minimum Go target remain future work)
- Native generic structs with explicit instantiation, `extends comparable`, substituted
  nested/external value/pointer method behavior, relative linking, editor support, and
  independent-Go runtime comparison. (implemented)
- Native generic interfaces with explicit instantiation, `extends comparable`,
  substituted method contracts, explicit class conformance, relative linking,
  editor support, and independent-Go runtime comparison. (implemented)
- Native generic reference classes with explicit instantiation, `extends comparable`,
  substituted fields/constructors/instance methods and generic interface contracts,
  inferred/explicit/partial static calls, inheritance and virtual dispatch with
  concrete, remapped, and multi-level bases, exact-target hierarchy conversions,
  construction-phase safety, relative linking, editor support, public generated-Go
  APIs, and independent-Go runtime comparison. (implemented; descendant-aware
  intermediate generic downcasts remain future work)

Most class/interface/object foundations, explicit result propagation,
direct-assignment-sensitive local null narrowing and joins,
declaration-identity local address/capture invalidation, and
straight-line, conditional, switch, select, and statically guaranteed-iteration
constructor field initialization are implemented. Guaranteed forms include
conditionless loops; compile-time-true boolean, integer-comparison, and
string-comparison expressions; and range over nonempty array/constant-string
expressions, positive-length fixed arrays and pointers, provably nonempty
`append`, or positive-constant-length `makeSlice`. Stable member narrowing is
also implemented for direct class-field paths. Local, `for`-initializer,
same-file global, and explicitly imported `const` chains propagate these
boolean, integer, string, and cardinality proofs by declaration identity;
mutable or dynamic bindings do not. Broader cardinality proofs and package
distribution remain.

The nullable-flow matrix now covers non-null/null direct assignments,
re-narrowing, early exits, exhaustive/non-exhaustive structured joins,
fixed-point loop backedges, program-point-sensitive address-taking, nested
mutable capture, shadowed declarations, branch-local escapes, and `const`
snapshot recovery. Generated runtime tests compare mutable-capture and
pointer-to-nullable-slot behavior directly with equivalent Go implementations
over present/absent and mutate/no-mutate matrices. Loop tests cover fixed-point
backedges for `while`, ordinary `for`, and range forms, including `continue`,
`break`, unreachable trailing writes, capture/address escape, and stable
rechecks on every iteration. The next milestone adds escaping/call-in-place
callbacks, goroutine-visible mutation, stable private fields, and unstable
mutable properties. Every accepted pre-write access remains paired with a rejected
reachable post-write access and generated/runtime coverage showing that
analysis does not alter evaluation or representation.

The stable-member matrix covers parameters, locals, `this`, nested class-field
paths, null guards, inverse comparisons, non-null assignments, exhaustive
`if`/`switch` joins, loops, aliases, and shadowed receivers. Direct or aliased
field writes, receiver reassignment, address-taking, mutating closures, and
unknown calls invalidate reachable facts. The generated program is compared
with a separately declared handwritten Go object model over present/absent,
branch, switch, and loop-boundary inputs.

### Phase 2.5: explicit single inheritance (in progress)

- Reference identity and nil behavior across implicit upcasts. (implemented)
- Single `extends`, base state/implementation reuse. (implemented)
- `virtual`, explicit `override`, and `super`. (implemented)
- Deterministic base-to-derived constructor order and construction-phase virtual dispatch. (implemented)
- Multi-level lowering, inherited interface conformance, and independent Go runtime comparison. (implemented)
- Checked/forced downcasts with nil, failure, identity, multi-level, and single-evaluation coverage. (implemented)
- Protected fields, constructor fields, instance/static methods, and virtual overrides. (implemented)
- Final classes and final overrides. (implemented)
- Public Go hierarchy conversions and external-package virtual dispatch. (implemented)

### Phase 3: concurrency and HTTP interoperability

- Higher-level `Task<T>`, expression `go`, `await`, and `detach`. (implemented)
- Explicit task context parameters. (implemented through ordinary Go interop)
- Automatic request-context inheritance and cancellation propagation.
- Bounded context-aware outbound `fetch` adapter. (implemented)
- Server-side `net/http` adapter, method router, and request context access. (implemented)
- Router race and concurrent behavior tests. (implemented)

Raw Go concurrency, structured task lifecycle/panic transport, direct JSON API
dogfood, and the thin outbound plus method-routing HTTP kernel are implemented.
Automatic task cancellation and context inheritance remain language/runtime
work.

### Phase 4: package productivity

- Reproducible dependency resolution and locks.
- Explicit `emit-go`.
- Outgoing C ABI 0 for fixed-width scalars.
- Go API documentation generation and OnsenTamago package test runner.
- External Go module license reports.
- Incoming C FFI generation with integers, bytes, handles, results, and release.

Dependency locking, `emit-go`, license inspection, outgoing C ABI 0, and the
manifest-driven incoming schema 1 scalar/string/enum/POD/tagged-union/
call-scoped-and-registered-callback/status/opaque-handle path are implemented.
Exact Go module installation is also exposed as transactional
`ontama install --go-module`; source-only GitHub installation for OnsenTamago
packages remains future work.

### Phase 4.5: production C FFI

- Source-level typed FFI declarations over the implemented checked manifest IR.
- Ownership, lifetime, and thread-safety validation.
- ABI manifests/diffs, callbacks, goroutine behavior, and OS-specific integration tests.

### Phase 5: expand the language through real use

- Collect missing language and compiler features from real applications.
- Improve generation based on profiles.

### v0.2 and later: public language guide

- Publish a GitHub Pages site for first-time users rather than exposing design
  notes as the primary learning path.
- Cover installation, first program, syntax, types, functions, classes,
  structs, errors, nullability, concurrency, Go interop, FFI, projects, CLI,
  editor use, and complete applications in a progressive order.
- Compile every executable documentation example in CI and compare runtime
  examples with explicit expected output where applicable.
- Version documentation with the compiler so released syntax and the default
  guide cannot silently diverge.

The v0.2 track now includes the initial progressive VitePress guide, local
search, release navigation, and GitHub Pages deployment workflow. Executable
guide snippets are stored as ordinary `.otm` sources, checked with the release
compiler, run against explicit output files, and rebuilt with the site in the
required compatibility and release workflows. Broader examples and archived
version snapshots remain ongoing documentation work.

## Verification commands and reporting

Development should normally end with:

- Full normal test suite.
- Full race-enabled test suite.
- `go vet`.
- Lexer fuzz target.
- Parser fuzz target.

Public change descriptions should include only user-relevant implementation changes and concise test results. They must not expose credentials, machine-specific paths, local operational details, or registry administration.
