---
title: Frequently asked questions
description: Short answers about Kinmokusei's relationship to TypeScript and Go, runtime types, memory, packages, generated code, and releases.
---

# Frequently asked questions

These answers describe the current v0.2 preview. Follow each link when you need the complete contract or an executable example.

## Is Kinmokusei TypeScript?

No. It borrows recognizable syntax such as type annotations, classes, interfaces, and arrows, but it does not run JavaScript or accept arbitrary TypeScript. There is no DOM, Node.js global environment, npm module resolution, prototype mutation, or JavaScript coercion model.

Start with [Coming from TypeScript](./from-typescript) for the important semantic differences.

## Is it another spelling of Go?

No. `.km` is its own checked source language. Go is the current compilation target, runtime model, package ecosystem, and low-level interoperability boundary. Kinmokusei adds contracts such as reference classes, nullable-flow checking, `Result<T>`, typed exceptions, and structured tasks.

[Coming from Go](./from-go) maps the shared concepts and deliberate differences.

## Does it have `dynamic` or `any` values?

There is no native universal `dynamic` or `any` type. Ordinary bindings and expressions have a static type, and generic type parameters remain statically checked.

Interface values are different: they have a known interface type while carrying a concrete runtime value. Imported Go interfaces support checked or forced assertions and type switches; class hierarchies support virtual dispatch and checked or forced downcasts. This is typed runtime polymorphism, not unrestricted dynamic property access. See [Types and values](../book/types-and-values#interfaces).

## Is memory garbage-collected?

With the current compiler, generated programs use the ordinary Go runtime and memory model. Classes are pointer-backed references; slices, maps, channels, interfaces, closures, and imported Go values retain their documented Go behavior. The Go compiler decides stack versus heap placement, and Kinmokusei exposes no manual object deletion or deterministic object destructor.

External resources still need explicit lifecycle management. Use APIs such as `Close`, pair them with `defer` or `finally` where appropriate, and keep C ownership rules explicit. [Types and values](../book/types-and-values) defines copying and aliasing; [C ABI and FFI](../guide/c-ffi) defines native ownership boundaries.

## Can I add methods outside a type declaration?

Yes, for a supported native struct, enum, or defined type declared in the same source module. The receiver is an explicit first `this` parameter:

```ts
public function reset(this: *Counter): void {
  this.value = 0;
}
```

This is a compile-time method declaration, not runtime monkey patching. Cross-module extension of an unrelated type is rejected, and the compiler checks one fixed method set before generation. See [Function semantics and generics](../book/functions-and-generics#methods-as-functions-with-receivers).

## Can it use every Go package?

It can import standard-library and locked external Go packages whose referenced public shapes are representable. Package loading and symbol support are separate: an unused unsupported export does not reject the rest of a package.

Target files, build tags, CGO, `internal` visibility, unsafe pointers, generic constraints, and reachable API types still apply. Use [`keika interop audit`](../reference/cli#go-interoperability-audit) to inspect a package rather than assuming all exports work.

## Does `Result<T>` allocate a wrapper?

No. `Result<T>` is a function or method return effect that lowers to Go `(T, error)`; `Result<void>` lowers to one `error`. It cannot be stored in a field, collection, or ordinary local value. Postfix `?` is an explicit early-return boundary.

The [Result parsing recipe](../examples/result-parsing) demonstrates success, validation failure, and a Go parsing error.

## Does a task behave like a JavaScript Promise?

No. `Task<T>` is a local, non-escaping, exactly-once capability. Every continuing path must consume it with `await` or `detach`; it cannot be copied, stored, captured, or returned. Starting a task evaluates the call target and arguments synchronously once before its worker goroutine begins.

See [Concurrency and tasks](../book/concurrency-and-tasks) for lifecycle and panic behavior.

## Does generated Go require Kinmokusei at runtime?

No. Explicitly emitted Go is formatted, standalone source without a Kinmokusei runtime dependency. It can be inspected, tested, built, or published with ordinary Go tooling. Some constructs generate helper declarations, but those helpers are part of the emitted package.

[Generated Go](../guide/generated-go) explains the artifact and public API guarantees.

## How are dependencies installed?

Current project dependency commands manage exact Go module versions. They write a canonical lock and compiler-managed module state transactionally. Normal check, build, run, emit, and editor operations never fetch or update dependencies implicitly.

Source-only Kinmokusei package distribution is planned rather than implemented. See [Modules and projects](../guide/projects-and-cli) for the current workflow.

## Is v0.2 a stable release?

The site documents the v0.2 preview implementation, but a preview label is not evidence that a matching release tag has been published. Kinmokusei is pre-1.0, so source, generated API, manifest, and CLI compatibility may still change deliberately with release notes.

Use [Releases and compatibility](../project/releases) to distinguish the latest published tag from the documentation branch.

## Where should I report a compiler problem?

Reduce the problem to the smallest `.km` source, record `keika version`, preserve the original diagnostic and target information, and report it in the project issue tracker. Do not repair generated Go when the compiler already reports a Kinmokusei source error.

The [Troubleshooting guide](../guide/troubleshooting) provides a diagnostic workflow and machine-readable evidence format.
