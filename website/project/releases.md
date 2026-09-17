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
Intentional source or public API breaks require a minor release and migration
notes; compiler fixes may newly reject invalid programs and are documented.

v0.5 is the milestone for completing the audited Go language compatibility work,
with independent runtime comparisons and documented deliberate language
differences. Statement coverage and package import counts are not measures of
complete language compatibility. OOP improvements continue during v0.4.x.

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
