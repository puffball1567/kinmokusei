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

Private/protected source declarations do not become public merely because Go capitalization is different.

## Publish or integrate

You may distribute `.km` source, generated Go, or both. A consumer of published generated Go does not need Kinmokusei or a compiler checkout.

Handwritten Go remains a supported neighboring layer for measured optimizations, adapters, or APIs not yet safely expressible in Kinmokusei. Keep the package boundary explicit and test it from an external Go consumer.

## Verification model

Readable output and a successful Go build are necessary, but they do not prove semantics by themselves. Runtime features with a Go equivalent are compared against independently handwritten Go programs that do not import or inspect generated output.

For application-level tests, [Testing applications](./testing) shows a checked Kinmokusei package consumed by an ordinary Go `_test.go` file. This verifies the public boundary without treating generated implementation details as expected behavior.

See the [Quality promise](../project/quality) for the full contract.
