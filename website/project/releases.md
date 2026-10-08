---
title: Releases and compatibility
description: Understand Kinmokusei pre-1.0 release status, version matching, generated API expectations, and migration policy.
---

# Releases and compatibility

Kinmokusei is currently pre-1.0. Released behavior is tested and documented, but source syntax, CLI details, and generated public APIs may change between minor versions while the language converges.

## Documentation version and release tag

The navigation label **v0.4** identifies the language version described by this site. Published compiler and editor artifacts are identified by a matching release tag; use the release list to confirm which artifacts are available before installing.

When a version is tagged, use these sources for different questions:

| Question | Authoritative source |
| --- | --- |
| What does this compiler accept? | Documentation matching the installed compiler release |
| What changed in the repository? | [`CHANGELOG.md`](https://github.com/puffball1567/kinmokusei/blob/main/CHANGELOG.md) and the tagged release notes |
| Which files can I install? | Assets and `SHA256SUMS` on the tagged GitHub release |
| Is a generated C boundary compatible? | The prior manifest plus `keika abi check` |

A migration requirement exists when the release notes identify a source, lock, CLI, generated-API, or boundary change. The documentation version alone does not replace those notes.

## Version policy

The pre-1.0 v0.4.x series delivers compatible feature additions and fixes in
patch releases. This project policy differs from strict SemVer feature numbering.
Intentional source or public API breaks normally require a minor release and
migration notes; compiler fixes may newly reject invalid programs and are
documented. v0.4.6 is an explicit exception: verified text changes source/public
APIs in this patch release. Review the migration notes below before upgrading.

v0.5 is the milestone for completing the audited Go language compatibility work,
with independent runtime comparisons and documented deliberate language
differences. Statement coverage and package import counts are not measures of
complete language compatibility. OOP improvements continue during v0.4.x.

## Unreleased: development branch

Fixed-array type lengths accept compile-time integer expressions on the development
branch. Source and Go constants, enum members, static class constants and constant
`len`/`cap`/`min`/`max` can supply the length; runtime values, negative lengths and
target-width overflow are rejected. Length references participate in definition
jump, find-references and rename. This addition is not included in v0.4.6.

Source generic declarations also accept native classes, structs/pointers,
source interface values and distinct types that satisfy Go method contracts.
Checks include inherited/generic signatures, dependent method-based inference,
getter/setter names, `Result`/variadic lowering, pointer method sets and all
remaining type-set requirements. Private/static/generic methods and mismatched
text/nullability contracts are rejected. This addition is not in v0.4.6 either.

## v0.4.6 highlights

Use matching v0.4.6 compiler and editor artifacts. This release includes an
intentional source/API compatibility change despite its patch version number.

- Application attribution directory naming changes from `<output>.licenses/`
  to `<output>-licenses/`, without modifying or deleting legacy directories.
- Incoming FFI typed `borrowedArray` inputs and registration-owned
  `retainedArray` inputs for scalar, enum and POD elements.
- Verified UTF-8 `string` and raw immutable `bstring`, checked decoding and
  text slicing. Go strings and error messages are raw;
  `Response.text()` returns `Result<string>` and `rawText()` preserves bytes.
- Multiple opaque handle arguments with ordered, deduplicated locking and
  registration lifetime leases; value-copy protection for resource objects.
- `mainThread` policy for normal Go executables, including startup
  `runtime.LockOSThread` and wrong-thread errors instead of implicit dispatch.
- Multiple call-scoped callbacks, copied/mutable typed callback arrays,
  subscription-only manifests and checked array allocation bounds.
- Stronger FFI cleanup, callback failure attribution, generated parameter-name
  isolation and `panic(nil)` containment.
- Verified-text storage checks across structural interfaces and instantiated
  generic/interface inheritance, preventing hidden-field reflection escapes.
- Updated user manual, syntax/type/built-in references and executable examples,
  including offline source-library and C-backed package workflows.

The [FFI guide](../guide/c-ffi) and [manifest reference](../reference/c-ffi-manifest)
describe the expanded surface. The matching
[v0.4.6 release notes](https://github.com/puffball1567/kinmokusei/releases/tag/v0.4.6)
include migration examples; use tagged documentation for older compilers.

### Migrating from v0.4.5

Use `bstring` for arbitrary Go strings, shared Go string collections and callback
parameters. Ordinary `string` literals and scalar text can widen to `bstring`,
but raw data cannot become verified text by assignment or interface assertion.
Decode it using `string(raw)` and handle the `Result<string>` with `?`, an
explicit split or direct result forwarding. Byte-slice decoding copies a
snapshot; invalid bytes are rejected without replacement.

`Response.text()` now returns `Result<string>`. Use `rawText()` when the previous
arbitrary-byte behavior is intended. `Exception.message` and `.error()` return
raw text. Verified string slicing checks UTF-8 boundaries; slice `bstring(text)`
for arbitrary byte offsets. Go-generated public APIs still use Go `string`, so
external callers must honor verified-text contracts or use a raw-data facade.

Libraries relying on this behavior should declare `min-kinmokusei = "0.4.6"`
and release updated versions. Already published dependencies may need their own
source migration. See [text at the Go boundary](../guide/go-interop#text-at-the-go-boundary)
and [strings and Unicode](../book/types-and-values#strings-and-unicode).

## v0.4.5 highlights

- Application binaries from `keika build` now include `<output>.licenses/` with
  original Go/module/source-package notices, generated-runtime attribution and
  a target-specific inventory with executable and notice hashes.
- Missing dependency license text stops the build. Offline dependency policy,
  old outputs and manually added notice files are preserved.
- Nullable/Task checking follows intermediate exception states, `finally`
  cleanup and ordered value-switch selection.

Ship the complete notice directory with redistributed executables. Review
actual terms and separately linked native/system libraries; see
[application redistribution notices](../reference/cli#application-redistribution-notices).
Use matching compiler/editor v0.4.5 artifacts and the
[v0.4.5 release notes](https://github.com/puffball1567/kinmokusei/releases/tag/v0.4.5).

## v0.4.4 highlights

- Instance and static getter/setter properties, interface property contracts,
  virtual/abstract overrides, mutable static fields and typed class constants.
- Typed decorator registration and checked construction, method and accessor
  invocation for external DI and routing libraries.
- Source multiple-result signatures and forwarding, sole-call argument
  expansion, method-only anonymous interfaces, local import aliases and `make[T]`.
- Improved generic inference, target-sized constant checks, nullable contracts,
  Task ownership and branch/select evaluation analysis.
- Stronger external-package boundaries, name isolation and editor diagnostics,
  with 103 implemented runtime contract groups backed by independent Go tests.

Use matching compiler and editor versions. Libraries depending on these additions
should declare `min-kinmokusei = "0.4.4"`. Stronger diagnostics may reject programs
that previously erased nullable contracts, reused/unhandled tasks, or depended
on captured generated names. See the
[v0.4.4 release notes](https://github.com/puffball1567/kinmokusei/releases/tag/v0.4.4)
for upgrade guidance and remaining boundaries.

## v0.4.3 highlights

- Short source imports are configured automatically by `keika deps add`, with
  optional `--alias` overrides and transactional conflict checks. Aliases are
  package-local and preserve canonical dependency and type identities.
- Imported `main` bindings remain ordinary module-local data or functions;
  applications still declare their own entry point.
- `copyArray` supports array-constrained generic target types, including
  mixed lengths and generic class methods.
- Parser and Go generation are split into smaller responsibility-focused files.

Existing dependencies can opt into short names through `[imports]` followed by
`keika deps lock`. Libraries that use manifest aliases should declare
`min-kinmokusei = "0.4.3"`. See [external packages](../guide/external-packages).

## v0.4.2 highlights

- CLI archives include Go and third-party attribution materials in
  `THIRD_PARTY_NOTICES.md` and `licenses/go/`. Keep these alongside redistributed
  binaries. Kinmokusei remains Apache-2.0; the Go compiler is an external dependency.
- Channel-constrained generics support send, receive, checked receives, `select`
  and closing, preserving direction and source nullability.
- Constraint intersections can discard provably disjoint parameter-dependent
  shapes. Nullable array-pointer constants and select-send class upcasts are fixed.
- Destructured locals show inferred types in editor hover and completion.

## v0.4.1 highlights

- External `.km` libraries with tagged acquisition, direct/transitive dependencies,
  locked content hashes, public submodules and local development replacements.
- Shared compiler/LSP package resolution, including editor workspace context
  without an open consumer document.
- `keika new app` and `keika new library` with offline initial locks and usable
  samples; library templates declare Kinmokusei `0.4.1` as their minimum.
- Generic collection indexing/slicing, mixed array/slice bounds and string/byte
  slice operations with preserved Go behavior and source type contracts.

### Migrating from v0.4.0

Existing Go-only schema 3 locks remain readable. Explicit dependency operations
write schema 4; commit the updated lock and use `keika deps fetch` to restore
locked state after cloning. Normal compilation never downloads packages or
rewrites the graph. External package authors should declare
`min-kinmokusei = "0.4.1"`, a public entry and matching module/version metadata before tagging.
See [external packages](../guide/external-packages) for the package format and
current exact-version selection rules.

## v0.4.0 highlights

- Optional semicolons, named Go imports, arrow-style functions and entry points,
  contextual/forward inference, and recursive local arrow groups.
- Source exports, re-exports, and aliases retaining declaration identity and
  shared storage through module chains.
- Abstract classes and methods, generic dependency-injection contracts, and
  stricter interface/override signature checks.
- Result-returning function values, explicit Result discard, and diagnostics
  for unused named local error bindings.
- Constraint intersections, imported Go type sets and method interfaces,
  generic callback inference, and generic collection built-ins/conversions.
- Preserved scalar, ordered-operation, and array-length constants, with
  stronger bounds, overflow, initialization, and source-encoding diagnostics.
- Semantic-analysis refactoring, editor integration, executable documentation,
  and independent Go comparisons across 100 covered runtime contract groups.

This example combines the new source syntax, abstract dependency injection,
generic array copies, and explicit Result handling:

<<< ../snippets/release-v0-4.km{ts}

It prints `hello 5 1 2`.

### Migrating from v0.3.0

- Keep a returned/thrown value on the `return`/`throw` line, or open its grouped
  expression there. A newline directly after the keyword now ends the statement.
  `break`/`continue` labels must also stay on the keyword line.
- A module-level `const` arrow with an unnamed function type now emits a Go
  function rather than an assignable variable. Use a mutable binding where
  reassignment or addressable function storage is required.
- A directly initialized local arrow sees its own binding; consecutive local
  arrows see peer bindings throughout their group. Rename accidental shadowing.
  Unresolved recursive result types still need annotations.
- A file using source exports exposes only its selected declarations. Export
  every name that other source modules need; `export {}` exposes none.
- Use, propagate, return, or explicitly discard Result errors. Numeric overflow,
  incompatible nullable/generic method contracts, cyclic global initialization,
  and uninstantiated Go generic function values are checked more precisely.
- Scalar and eligible `len`/`cap` bindings may now be real Go constants and are
  not addressable. Use `let` when runtime storage is needed; do not rely on side
  effects inside unevaluated constant array-length operands.

## v0.3.0 highlights

- Generic class/struct methods, reusable source type-set constraints, dependent
  bounds and inference, and type-parameter conversions.
- Multiple interface inheritance, imported Go interface bases, anonymous Go
  runtime interfaces, and per-instance class field initializers.
- Integer and function-iterator ranges, including collection-shaped generic
  bounds and Go `iter.Seq` / `iter.Seq2`.
- Go numeric literal forms, complex numbers, precise numeric constants, and
  improved generic numeric inference.
- Editor support for the new constructs, stronger constructor initialization
  checks, bounded parallel differential tests, and 98 covered runtime contracts.

Kinmokusei targets Go. Abstract classes were added later, in v0.4.0.

When upgrading from v0.2.0, recheck code that relied on permissive numeric or
generic assignment diagnostics. Overflow, fractional integer contexts, and
incompatible nullable/generic assignments are rejected more precisely. A
value-returning function must end with a terminating statement; remove trailing
unreachable statements. Type parameters used by generated constructors or
generic method helpers must not hide their enclosing type name.

See the [complete changelog](https://github.com/puffball1567/kinmokusei/blob/main/CHANGELOG.md)
for detailed behavior and fixes.

This runnable example combines constraints, interface inheritance, instance
defaults, generic methods, iterator/integer ranges, and complex literals:

<<< ../snippets/release-v0-3.km{ts}

It prints `3 3 2 2.5 4`.

## Match compiler and editor versions

Use the compiler and Visual Studio Code extension from the same release. Direct Go package loading also depends on the Go toolchain version used to build `keika`; the release notes identify the supported Go range.

## Compatibility boundaries

- `.km` behavior is defined by the matching compiler and documentation release.
- `kinmokusei.lock` records the selected Go/target/module graph and is validated before normal compilation.
- Generated Go is standalone for consumers, but exact public generated API changes before 1.0 must be reviewed like any source API change.
- Published C ABI symbols use a canonical manifest/fingerprint and an explicit compatibility checker.
- Incoming FFI manifests are schema-versioned; unsupported ownership shapes are rejected rather than inferred.

## Upgrading

Projects moving from v0.1 should begin with
[Migrate from v0.1](../guide/migrating-from-v0-1).

Before changing a project compiler version:

1. Read the release notes for syntax, diagnostic, CLI, lock, Go-version, and generated-API changes.
2. Use the matching editor extension.
3. Run `keika deps check` and refresh the lock only with an explicit dependency command when required.
4. Run application tests against generated artifacts and external Go/C consumers.
5. For a published C boundary, run `keika abi check` against the prior manifest.

Versioned documentation snapshots and formal breaking-release migration pages are planned as the release history grows. Until then, the [release list](https://github.com/puffball1567/kinmokusei/releases) and repository [`CHANGELOG.md`](https://github.com/puffball1567/kinmokusei/blob/main/CHANGELOG.md) are the authoritative change history.
