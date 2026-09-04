---
title: Read and write a temporary file
description: Connect Go filesystem errors to Result, validate a short write, and order cleanup with defer.
---

# Read and write a temporary file

This application recipe uses the real Go `os` package without a wrapper runtime. Every fallible operation remains visible, and temporary resources are cleaned on success or early return.

## Source

<<< ../snippets/filesystem-round-trip.km{ts}

## Run

```sh
keika check filesystem-round-trip.km
keika run filesystem-round-trip.km
```

Expected output:

```text
steaming
```

The temporary path is intentionally absent from the output, so the example has the same observable contract on every supported platform.

## Follow the failure path

`os.CreateTemp`, `WriteString`, and `os.ReadFile` return Go errors. Postfix `?` handles both `(T, error)` shapes:

1. evaluate the call once;
2. return the result type's zero value plus the error when it is non-nil;
3. otherwise yield `T` to the binding.

`WriteString` reports a byte count, so the function also rejects a short write even when the returned error is nil. `len(text)` is correct here because file writes count UTF-8 bytes, not Unicode code points.

At the call site, `[contents, err]` exposes the generated Go `(string, error)` boundary. The non-nil branch returns before `err.Error()` can be called with a nil receiver.

## Cleanup order

Deferred-call arguments are captured when each `defer` statement executes. Calls run in last-in, first-out order when `writeAndRead` returns:

```ts
defer os.Remove(path); // runs second
defer file.Close();    // runs first
```

Closing before removal makes the example portable to platforms that do not permit deleting an open file. This concise form intentionally ignores cleanup errors. Code that must report a close failure should close explicitly on the success path while retaining a deferred fallback for earlier returns.

Use [Errors, Result, and exceptions](../book/errors-results-exceptions) for propagation semantics and [Go interoperability](../guide/go-interop) for imported pointers, methods, and raw errors.
