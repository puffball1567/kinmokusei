---
title: Example gallery
description: Complete Kinmokusei examples for command-line programs, Go interop, JSON APIs, web frameworks, and C ABI export.
---

# Example gallery

The repository examples are complete source trees rather than presentation-only fragments. Documentation recipes embed separately checked source, and behavior-bearing snippets have committed expected output.

## Start and core language

<div class="doc-cards">
  <a class="doc-card" href="https://github.com/puffball1567/kinmokusei/tree/main/examples/hello"><strong>Hello world</strong><span>One file, one Go import, and one executable entry point.</span></a>
  <a class="doc-card" href="./result-parsing"><strong>Parse with Result</strong><span>Bridge a Go error, validate a port, and inspect explicit success/failure values.</span></a>
  <a class="doc-card" href="./collections"><strong>Collections</strong><span>Trace copying and shared views across slices, arrays, pointers, allocation, maps, and collection built-ins.</span></a>
  <a class="doc-card" href="./numeric-operators"><strong>Numeric & bitwise operators</strong><span>Combine integer flags, shifts, complement, updates, and explicit width conversions.</span></a>
  <a class="doc-card" href="./comparisons-and-short-circuit"><strong>Comparisons & short-circuiting</strong><span>Compare typed values and observe exactly which boolean operands execute.</span></a>
  <a class="doc-card" href="./unicode-strings"><strong>UTF-8 strings</strong><span>Compare byte length and indexing with Unicode code-point iteration and byte offsets.</span></a>
  <a class="doc-card" href="./generics"><strong>Generics</strong><span>Instantiate a generic value struct and call an explicitly typed generic function.</span></a>
  <a class="doc-card" href="./variadics"><strong>Variadics & spread</strong><span>Accept individual arguments and expand one final typed slice.</span></a>
  <a class="doc-card" href="./defined-types"><strong>Defined domain types</strong><span>Give strings and maps nominal identity and their own method sets.</span></a>
  <a class="doc-card" href="./modules"><strong>Relative modules</strong><span>Split declarations across files with explicit, non-transitive imports.</span></a>
</div>

## Types and control flow

<div class="doc-cards">
  <a class="doc-card" href="./polymorphism"><strong>Interface polymorphism</strong><span>Implement an interface with a reference class and dispatch through it.</span></a>
  <a class="doc-card" href="./type-switch"><strong>Go interface type switch</strong><span>Narrow imported interfaces to concrete pointer types with case-local bindings.</span></a>
  <a class="doc-card" href="./inheritance"><strong>Inheritance & downcasts</strong><span>Override virtual methods and recover a derived class without losing identity.</span></a>
  <a class="doc-card" href="./exceptions"><strong>Typed exceptions</strong><span>Order typed catches, bridge Go errors, and guarantee cleanup with finally.</span></a>
  <a class="doc-card" href="./nullable-flow"><strong>Nullable flow</strong><span>Snapshot mutable storage and re-establish non-null proofs after writes.</span></a>
  <a class="doc-card" href="./control-flow-boundaries"><strong>Control-flow boundaries</strong><span>Observe defer evaluation, explicit fallthrough, labeled continue, and checked goto rules.</span></a>
  <a class="doc-card" href="./struct-receivers"><strong>Struct receivers</strong><span>Compare copied value receivers with explicit pointer mutation.</span></a>
</div>

## Concurrency

<div class="doc-cards">
  <a class="doc-card" href="./tasks"><strong>Structured tasks</strong><span>Start work eagerly, consume every task once, and join Result-producing work before error propagation.</span></a>
  <a class="doc-card" href="./channels"><strong>Channels</strong><span>Send, receive, close, and observe the checked state after draining.</span></a>
  <a class="doc-card" href="./select"><strong>Select</strong><span>Choose ready channel operations and handle a non-blocking default path.</span></a>
</div>

## Go and applications

<div class="doc-cards">
  <a class="doc-card" href="./command-line-app"><strong>Command-line application</strong><span>Build a native executable, validate positional arguments, and test exact process output.</span></a>
  <a class="doc-card" href="./json"><strong>JSON value</strong><span>Pass a structural object to the real Go encoding/json package.</span></a>
  <a class="doc-card" href="./filesystem-round-trip"><strong>Filesystem round trip</strong><span>Propagate real file errors, detect a short write, and order portable cleanup.</span></a>
  <a class="doc-card" href="./go-standard-library"><strong>Go standard-library values</strong><span>Use an imported struct, pointer methods, explicit results, raw errors, and a callback into Go.</span></a>
  <a class="doc-card" href="./http-router"><strong>Test an HTTP router</strong><span>Exercise kinmokusei/http routes with net/http/httptest and no open port.</span></a>
  <a class="doc-card" href="./bounded-http-fetch"><strong>Bounded HTTP fetch</strong><span>Test a caller-owned transport, copied response state, and an oversize failure without opening a socket.</span></a>
  <a class="doc-card" href="../guide/testing"><strong>Test an emitted package</strong><span>Generate a public Go package and exercise its values and Result boundary with go test.</span></a>
  <a class="doc-card" href="./external-go-module"><strong>External Go module</strong><span>Lock an exact version, audit its surface, and keep builds offline.</span></a>
  <a class="doc-card" href="./web-backend"><strong>React + Gin/Fiber</strong><span>One frontend with interchangeable Kinmokusei framework backends and an independent Go oracle.</span></a>
  <a class="doc-card" href="https://github.com/puffball1567/kinmokusei/tree/main/examples/json-api"><strong>Complete JSON API</strong><span>Direct <code>net/http</code> and <code>encoding/json</code> package interoperability.</span></a>
</div>

## Native boundaries

<div class="doc-cards">
  <a class="doc-card" href="./c-abi"><strong>C ABI export</strong><span>Generate a header, gateway, canonical manifest, and compatibility fingerprint.</span></a>
  <a class="doc-card" href="./incoming-c-ffi"><strong>Incoming C FFI</strong><span>Generate a checked private cgo package from an ownership manifest.</span></a>
</div>

The documentation check compiles every embedded valid snippet, runs each example with an output contract, validates diagnostic fragments for intentionally invalid sources, and checks generated Go and C-boundary artifacts.

## Running repository examples

From a source checkout, invoke the compiler through Go:

```sh
go run ./cmd/keika check examples/hello/main.km
go run ./cmd/keika run examples/hello/main.km
```

Projects with locked external dependencies include their own manifest, lock, and verification instructions. Normal compilation stays offline/read-only with respect to the module graph.
