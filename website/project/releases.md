---
title: Releases and compatibility
description: Understand Kinmokusei pre-1.0 release status, version matching, generated API expectations, and migration policy.
---

# Releases and compatibility

Kinmokusei is currently pre-1.0. Released behavior is tested and documented, but source syntax, CLI details, and generated public APIs may change between minor versions while the language converges.

## Documentation version and release tag

The navigation label **v0.3** identifies the language version described by this site. Published compiler and editor artifacts are identified by a matching release tag; use the release list to confirm which artifacts are available before installing.

When a version is tagged, use these sources for different questions:

| Question | Authoritative source |
| --- | --- |
| What does this compiler accept? | Documentation matching the installed compiler release |
| What changed in the repository? | [`CHANGELOG.md`](https://github.com/puffball1567/kinmokusei/blob/main/CHANGELOG.md) and the tagged release notes |
| Which files can I install? | Assets and `SHA256SUMS` on the tagged GitHub release |
| Is a generated C boundary compatible? | The prior manifest plus `keika abi check` |

A migration requirement exists when the release notes identify a source, lock, CLI, generated-API, or boundary change. The documentation version alone does not replace those notes.

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

The compiler still emits Go. KIR, C++, and Nim backend integration, abstract
classes, property accessors, and static fields are not part of this release.

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
