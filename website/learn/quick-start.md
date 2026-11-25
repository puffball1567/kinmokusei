---
title: Five-minute quick start
description: Check, run, build, and inspect a small Kinmokusei program.
---

# Five-minute quick start

By the end of this page you will have checked one source file, run it, built an executable, and inspected its generated Go.

## 1. Create a source file

Save this program as `hello.km`:

<<< ../snippets/hello.km{ts}

`console` comes from the compiler-managed `kinmokusei/console` standard module.
`console.log` writes a line to standard output.

## 2. Check before running

```sh
keika check hello.km
```

A valid file produces no output and exits with status `0`. `check` lexes, parses, resolves imports, and performs semantic checking; it does not generate or execute a program.

## 3. Run the program

```sh
keika run hello.km
```

Expected output:

```text
Hello from Kinmokusei
```

`run` generates a project-local Go module, invokes the selected Go toolchain, and executes the program. A locked cross-target project is rejected before execution; use `build` for cross-compilation.

## 4. Build an executable

```sh
keika build -o hello hello.km
```

The output is a native executable for the active or project-locked target.

## 5. Inspect generated Go

```sh
keika emit-go -o generated.go hello.km
```

The formatted Go file includes the `main` function and the linked console
implementation. The console uses Go's `fmt` package internally.

`emit-go` is a publication boundary, not only a debugging aid. Generated output is standalone Go and does not require the compiler at consumer build time.

## Read a diagnostic

If a call uses an unknown console member or an incompatible argument type,
`check` reports the original `.km` path and source span. Machine consumers can
request the stable JSON envelope:

```sh
keika check --json hello.km
```

The exit status is `0` for valid source, `1` for invalid source or project input, and `2` for command misuse. See [Diagnostics](../reference/diagnostics) for the schema.

## Where next?

- Follow one program through the [language tour](./language-tour).
- Learn exact value and reference behavior in [Types and data](../guide/types-and-data).
- Add real packages with [Go interoperability](../guide/go-interop).
