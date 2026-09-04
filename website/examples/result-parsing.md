---
title: Parse input with Result
description: Validate a port number while bridging a Go error into a Kinmokusei Result.
---

# Parse input with `Result`

This recipe calls Go's `strconv.Atoi`, propagates its raw `(int, error)` result with postfix `?`, and adds an application-level range check.

## Prerequisites

- `keika` and a supported Go toolchain on `PATH`
- one empty working directory

## Project tree

```text
port-parser/
└── main.km
```

## Source

Save as `main.km`:

<<< ../snippets/result-parsing.km{ts}

## Check and run

```sh
keika check main.km
keika run main.km
```

Expected output:

```text
8080 true
0 true
```

The first call succeeds. The second returns the `int` zero value plus a non-nil error because `70000` is outside the accepted port range.

## Contract demonstrated

- `strconv.Atoi(text)?` evaluates once and propagates a non-nil Go error.
- `fail(err)` emits the payload type's zero value and the error.
- `Result<int>` lowers directly to Go `(int, error)`.
- `[value, err]` keeps the two results explicit at the call site.

To inspect the exact error rather than only its presence, call `err.Error()` inside a branch where `err !== nil` is known.

See [Errors and nullability](../guide/errors-and-nullability) for storage restrictions and exception separation.
