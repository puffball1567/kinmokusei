---
title: Generated Go
description: Inspect, test, publish, and integrate the deterministic Go emitted by Kinmokusei.
---

# Generated Go

Generated Go is a first-class deliverable, not an opaque cache. It is designed to be understandable by reviewers and usable by ordinary Go tooling.

## Emit source explicitly

```sh
keika emit-go -package example -o generated.go src/main.km
```

Without `-o`, the command writes source to standard output. The package name defaults to `main`.

Project `build` and `run` commands use `.kinmokusei/gen/` for compiler-managed intermediate modules. Do not commit that directory. Use `emit-go` for a durable artifact.

## Source locations and coverage

`build` and `run` also write `generated.go.map.json` beside their generated Go
file. To keep a durable source/map pair, request the sidecar explicitly:

```sh
keika emit-go -o generated.go -source-map generated.go.map.json src/main.km
```

`-source-map` requires `-o`; the two outputs must be distinct and cannot overwrite
any loaded `.km` input. Without this flag, `emit-go` keeps its existing output.

The versioned JSON connects physical Go ranges to executable `.km` statement
origins and expression-bodied arrows. It includes input/output SHA-256 hashes
and stable origin IDs so test tools can locate failures and combine results from
different test executables. It does not change Go line numbers, inject runtime
calls, or aggregate coverage itself. Consumers must verify hashes and archive
the matching Go file and sidecar before another build overwrites compiler state.

Generated runtime support and declarations without an executable origin remain
unmapped; use the original Go location as a fallback. Statement ranges are not
branch coverage and are not a promise that every line of a source range ran.
See [Testing applications](./testing) for consumer boundaries and
the [source-map schema](https://github.com/puffball1567/kinmokusei/blob/devel/docs/compiler-architecture.md#source-map-interchange)
in the repository for the machine-readable contract.

## Generation guarantees

Generated source is:

- deterministic for the same source, lock, target, and compiler;
- formatted by `gofmt`;
- validated for Go syntax and types after Kinmokusei checking;
- buildable with ordinary `go build` and `go test`;
- free of a Kinmokusei compiler-runtime dependency;
- portable—module metadata must not contain machine-specific checkout paths;
- direct at Go package boundaries, without reflection proxies.

Generated validation is a final invariant check, not the first place normal source errors should appear.

## Public APIs

Where a Kinmokusei type has a stable Go representation, the generated package exposes an ordinary Go API. Public functions retain scalar, collection, named-type, pointer, interface, generic, channel, and multiple-result shapes.

Classes become pointer-backed structs with constructors and methods. Public static methods become idiomatic package functions. Inheritance emits public upcast/downcast helpers and dispatch entry points. Native structs, enums, defined types, and their receiver methods remain ordinary named Go declarations.

Top-level declarations retain their written capitalization: `export function
greet` permits source imports but does not make `greet` exported under Go rules.
Use an initial uppercase letter, such as `Greet`, for an API intended for direct
Go consumers. Source `export` lists/re-exports control a different boundary.

Module-level `const` arrows with inferred or unnamed function types become Go
functions, not assignable function variables. `let` bindings and explicitly
named Go function types retain storage semantics. Instance accessors become
methods such as `GetValue()` and `SetValue(int)`. Public static properties use
package functions; static fields/constants use package variables/constants.
Abstract classes have no public constructor factory. See
[classes and interfaces](../book/structs-classes-interfaces) for the source rules.

Private/protected source declarations do not become public merely because Go capitalization is different.

Since v0.4.6, both `string` and `bstring` lower to Go
`string`. UTF-8 is a source contract, not a new Go storage type. A direct Go
consumer must provide valid UTF-8 for verified-text parameters and preserve
that invariant when mutating exposed fields or collections. Use a raw-data
facade and checked decoding when the external caller cannot guarantee it.
Generated decoding and text-slice helpers are self-contained; they add no
Kinmokusei runtime dependency. See [Go text contracts](./go-interop#text-at-the-go-boundary).

## Publish or integrate

For a distributable executable, `keika build -o app` also creates
`app-licenses/`. Application attribution was introduced in v0.4.5 under the name
`app.licenses/`; v0.4.6 uses the hyphenated name and preserves existing
legacy directories. Keep the newly generated directory with the binary;
the target-specific index identifies the actual toolchain and dependency notices.
See [application redistribution notices](../reference/cli#application-redistribution-notices)
for missing-license errors, safe rebuilding and native-library review boundaries.
Source-only `emit-go` and a subsequent direct `go build` do not create this bundle.

You may distribute `.km` source, generated Go, or both. A consumer of published generated Go does not need Kinmokusei or a compiler checkout.

Standalone does not mean dependency-free: preserve the imported Go module
versions, target/build-tag requirements and any cgo headers, native sources or
link libraries. `emit-go` emits source, not a vendored dependency bundle.
Applications using cgo also require their native build toolchain. Review runtime
and dependency notices for the application you redistribute; the compiler's
own notice files are not an inventory of every generated application's imports.

Handwritten Go remains a supported neighboring layer for measured optimizations, adapters, or APIs not yet safely expressible in Kinmokusei. Keep the package boundary explicit and test it from an external Go consumer.

## Verification model

Readable output and a successful Go build are necessary, but they do not prove semantics by themselves. Runtime features with a Go equivalent are compared against independently handwritten Go programs that do not import or inspect generated output.

For application-level tests, [Testing applications](./testing) shows a checked Kinmokusei package consumed by an ordinary Go `_test.go` file. This verifies the public boundary without treating generated implementation details as expected behavior.

See the [Quality promise](../project/quality) for the full contract.
