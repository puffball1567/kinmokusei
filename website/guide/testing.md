---
title: Testing applications
description: Check Kinmokusei source, test observable recipes, and exercise an emitted public package with ordinary Go tests.
---

# Testing applications

Kinmokusei v0.2 does not provide a `keika test` command. Use `keika check` for source validity, executable recipes for end-to-end behavior, and ordinary Go tests when you need to exercise an emitted package API.

## Choose the boundary under test

| Goal | Test boundary |
| --- | --- |
| Reject invalid source before generation | `keika check` and its diagnostic/exit status |
| Verify a command's observable behavior | `keika run` plus an exact output assertion |
| Test public functions and types as a package | `keika emit-go -package ...`, then `go test` |
| Test HTTP routing without a listener | `net/http/httptest` through `kinmokusei/http` |
| Verify compiler compatibility with Go | Independent handwritten Go oracle; compiler contributors only |

Generated Go can be tested, but generated output must not become the oracle for its own semantics. Assert the API behavior your application promises.

## A tested package

The Kinmokusei source exposes two Go-callable functions. `Add` and `Half` begin with uppercase letters, so their generated top-level Go declarations are exported:

<<< ../snippets/testing/public_api.km{ts}

Emit it as package `calculator`:

```sh
keika check public_api.km
keika emit-go -package calculator -o calculator.go public_api.km
```

An ordinary Go test can call the generated API directly:

<<< ../snippets/testing/public_api_test.go.txt{go}

Place the files in one Go package and run:

```sh
go test ./...
```

The documentation CI performs this exact generation and test. It proves that `Add` is callable and that `Result<int>` is exposed as `(int, error)`, including the zero value on failure.

## Public API discipline

- A top-level source name beginning with an uppercase Unicode letter becomes exported from the generated Go package; lowercase top-level names remain package-local.
- Relative `.km` imports are a separate boundary and select written names explicitly, regardless of capitalization.
- Keep parameter, result, named-type, pointer, interface, generic, channel, and error shapes within the documented interoperability matrix.
- Treat generated helper names as private unless the generated-Go guide documents them as public API.
- Regenerate before testing when `.km` source, the dependency lock, target, or compiler version changes.
- Do not hand-edit `calculator.go`; fix the source or add a handwritten neighboring Go file with an explicit package boundary.

For HTTP handlers, use the [port-free router recipe](../examples/http-router). For artifact guarantees and publication boundaries, read [Generated Go](./generated-go) and the [Quality promise](../project/quality).
