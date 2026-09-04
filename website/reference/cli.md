---
title: CLI reference
description: Complete reference for keika commands, options, outputs, side effects, and exit status.
---

# CLI reference

The command-line executable is `keika`. Source inputs end in `.km`.

## Invocation

```text
keika <command> [options] [arguments]
```

`keika help`, `keika -h`, and `keika --help` print top-level usage. `keika version` and `keika --version` print the command name and embedded version. An option applies to the command that immediately contains it and must appear before that command's first positional argument.

## Core commands

| Command | Purpose |
| --- | --- |
| `keika version` | Print command name and embedded version |
| `keika check [--json] <sources...>` | Lex, parse, resolve, and type-check without generation |
| `keika run <sources...>` | Generate a main module and run it with the selected Go toolchain |
| `keika build [-o path] <sources...>` | Generate and build a native executable; default output is `keika.out` |
| `keika emit-go [-o file] [-package name] <sources...>` | Emit deterministic formatted Go; defaults to stdout and package `main` |

`run` rejects a locked non-host target before execution. Its positional arguments are all source inputs; it does not forward program arguments. Use `build`, then invoke the resulting executable directly, for programs that read `os.Args`. `build` uses the locked target and supports cross-building. Core compilation commands never resolve or update dependencies implicitly.

## Core input and output behavior

| Command | Standard input/output | Files written |
| --- | --- | --- |
| `check` | Silent on success; diagnostics use stderr; `--json` uses stdout | None |
| `run` | Connects the generated program to the current stdin, stdout, and stderr | Compiler-managed `.kinmokusei/gen/` state |
| `build` | Go build diagnostics use stdout/stderr | Executable at `-o`, or `keika.out` |
| `emit-go` | Formatted Go uses stdout unless `-o` is present | The exact `-o` file when selected |

All source arguments form one generated package. Relative imports may load additional `.km` files, so callers normally pass entry files rather than every file in a project. Project-aware invocations validate the existing manifest, lock, and generated dependency state before compilation.

## C ABI and FFI

| Command | Purpose |
| --- | --- |
| `keika emit-c-abi -o <dir> <sources...>` | Write Go implementation/gateway, C header, and ABI manifest |
| `keika abi check --baseline <manifest> <sources...>` | Compare a new outgoing ABI against `kinmokusei_abi.json` |
| `keika ffi generate --manifest <file> -o <dir>` | Validate schema 1 and write `generated_ffi.go` |

`emit-c-abi` writes `generated.go`, `generated_cabi.go`, `kinmokusei_abi.h`, and `kinmokusei_abi.json`. ABI check succeeds for an identical fingerprint or additive symbols and reports breaking changes to standard error.

## Dependencies and projects

| Command | Purpose |
| --- | --- |
| `keika install --go-module [--offline] [--replace path] <module>@<version> [project]` | Add an exact Go module transactionally |
| `keika deps add [--offline] [--replace path] <module>@<version> [project]` | Lower-level spelling of exact add |
| `keika deps update [--offline] <module>@<version> [project]` | Update one declared module to an exact version |
| `keika deps remove [--offline] <module> [project]` | Remove one declared module |
| `keika deps lock [--offline] [project]` | Resolve and write canonical lock/internal module state |
| `keika deps check [project]` | Validate manifest, lock, generated module files, and license hashes |
| `keika deps licenses [--strict] [project]` | Print module, license path, and SHA-256 hash; strict mode rejects unknowns |
| `keika target [project]` | Print locked `GOOS`, `GOARCH`, `CGO_ENABLED`, and tags |

The optional project argument defaults to the current directory. Complete versions, including complete pseudo-versions, are required. Add/update/remove/lock preserve prior state on failure.

## Go interoperability audit

```text
keika interop audit [--stdlib] [--json] [--allow-incomplete] [packages...]
```

The command requires `--stdlib` or at least one import path. Plain output summarizes supported, unsafe-required, and unsupported public shapes. JSON includes the full inventory. Package load failures make the command fail unless `--allow-incomplete` is present.

## Language server

```text
keika lsp --stdio
```

No other transport or positional source argument is accepted. The server reads/writes Language Server Protocol messages on standard input/output.

## Check output

Plain `check` is silent on success. Source diagnostics and project/input errors go to standard error. `--json` always writes one JSON object to standard output. See [Diagnostics](./diagnostics).

## Exit status

| Status | Meaning |
| --- | --- |
| `0` | Successful command or valid check |
| `1` | Invalid source, project/input failure, build/run failure, incompatible ABI, or incomplete audit |
| `2` | Invalid command or option usage |
