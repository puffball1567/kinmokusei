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
| `keika check [--json] [sources...]` | Lex, parse, resolve, and type-check without generation |
| `keika run [sources...]` | Generate a main module and run it with the selected Go toolchain |
| `keika build [-o path] [sources...]` | Generate and build a native executable; default output is `keika.out` |
| `keika emit-go [-o file] [-package name] [sources...]` | Emit deterministic formatted Go; defaults to stdout and package `main` |

`run` rejects a locked non-host target before execution. Its positional arguments are all source inputs; it does not forward program arguments. Use `build`, then invoke the resulting executable directly, for programs that read `os.Args`. `build` uses the locked target and supports cross-building. Core compilation commands never resolve or update dependencies implicitly.

## Compiler updates

Available since v0.4.8:

| Command | Purpose |
| --- | --- |
| `keika update [--version vX.Y.Z]` | Verify and install an official stable compiler release on Linux/macOS |
| `keika update --check [--version vX.Y.Z]` | Report availability without modifying the installation |

The default selection is the latest stable release. Drafts, pre-releases,
invalid tags and explicit downgrades are rejected. An already current or newer
compiler is left unchanged. Untagged development builds require an explicit
version for replacement. Help exits with status 0, malformed arguments with 2,
and download/validation/installation failures with 1.

Only explicit compiler-management commands contact the release service; normal
compilation never checks for compiler updates. The updater verifies SHA-256,
archive paths, bundled license materials and the downloaded compiler version
before atomically switching the invoked launcher. It does not update the Go
toolchain, VSIX or dependency graph. Windows supports checking but not automatic
replacement yet. See [installation and update details](../learn/installation#update-the-compiler).

### Version selection (since v0.4.10)

| Command | Purpose |
| --- | --- |
| `keika use [--offline] vX.Y.Z` | Select an exact stable compiler, allowing downgrades |
| `keika-use [--offline] vX.Y.Z` | Select again independently of the active compiler's command support |

These commands require v0.4.10 or later. Automatic selection supports
Linux/macOS. Verified cache hits require no network; `--offline` rejects a
cache miss. Non-cached selections use official Release verification. Existing
unmanaged bundles must match the entire official archive before adoption.
Changed managed bundles, invalid layouts and unrelated companion files are
rejected without replacement. `update` still rejects explicit downgrades.
See [switching versions](../learn/installation#switch-compiler-versions).

### Version inventory (since v0.4.11)

| Command | Purpose |
| --- | --- |
| `keika versions` / `keika versions list` | List retained bundles in the invoked installation, newest first |
| `keika versions [list] --remote` | Fetch official stable release candidates with archive/checksum metadata for this OS/architecture |
| `keika-use versions [list] [--remote]` | Use the independent manager's inventory support after selecting an older compiler |

These commands require v0.4.11 or later. `use` also accepts the numeric spelling
`keika use 0.4.11`, normalized to the canonical `v0.4.11` tag.

Local listing is read-only and never accesses the network, runs a cached
compiler, creates a lock/helper or switches versions. It shows bundle bytes,
the active compiler, protected running/manager files, and `managed`, `unmanaged`
or `invalid` receipt/inventory status. An unmanaged bundle has no receipt;
invalid means validation failed, not permission to delete or repair it.
Only this installation's retained cache is scanned, not arbitrary manual
compiler copies elsewhere. Concurrent switches can change the observed state.
Local inventory follows the Linux/macOS managed-installation layout.

Only `--remote` requests release metadata. Drafts/prereleases, noncanonical tags
and releases without the target archive and `SHA256SUMS` are excluded. Results
are paginated, deduplicated and sorted by numeric version; errors do not return
an incomplete listing. The listing does not download or verify archive contents.
`use`/`update` still perform full checksum, archive, license and executable checks;
older incompatible bundles may require manual installation. Windows can list
its downloadable archives but still installs them manually.

An existing `keika-use` companion needs an inventory-capable management snapshot.
Run `keika use` from the newer compiler to refresh it before downgrading.

## Core input and output behavior

| Command | Standard input/output | Files written |
| --- | --- | --- |
| `check` | Silent on success; diagnostics use stderr; `--json` uses stdout | None |
| `run` | Connects the generated program to the current stdin, stdout, and stderr | Compiler-managed `.kinmokusei/gen/` state |
| `build` | Go build/attribution diagnostics use stdout/stderr | Compiler-managed `.kinmokusei/gen/` state, executable at `-o` (default `keika.out`), and `<output>-licenses/` |
| `emit-go` | Formatted Go uses stdout unless `-o` is present | The exact `-o` file when selected |

All source arguments form one generated package. Relative imports may load additional `.km` files, so callers normally pass entry files rather than every file in a project. Project-aware invocations validate the existing manifest, lock, and generated dependency state before compilation.

With no source arguments, the four core commands find the enclosing project and
select its `main.km`, or its declared library entry. Outside a project, omitting
sources is a usage error. Passing a directory is not shorthand for selecting
that project's sources. C ABI emission and ABI checks still require explicit
source arguments.

## Application redistribution notices

`keika build -o app` produces `app` and `app-licenses/` (or
`app.exe-licenses/` when `-o app.exe` is selected). Ship the entire notice
directory alongside the executable. The published v0.4.5 compiler uses
`<output>.licenses/`; v0.4.6 uses `<output>-licenses/` and leaves old
directories untouched. After rebuilding, distribute the newly generated
directory, not the legacy one. It contains:

- Go's original license and patent grant from the actual build toolchain;
- nested licenses/notices and complete additional-attribution source supplements
  for target-selected Go packages, including cgo source/header files;
- detected notice files for imported Go modules, local/version replacements and
  the locked external Kinmokusei source-package graph;
- the Apache-2.0 license for Kinmokusei-generated runtime/standard-library code;
- `INDEX.json` with toolchain, target, tags, dependency identities, notice hashes
  and the executable's SHA-256 digest, without local checkout paths.

Collection is conservative: notices may cover code discarded by the linker and
unused files/dependencies in source packages. Recognized file families are
`LICENSE`, `LICENCE`, `COPYING`, `NOTICE`, and `PATENTS`, case-insensitively,
including suffixes such as `.txt` or `-MIT`, and text files in `LICENSES/` or
`LICENCES/` directories. Missing license text, empty notices,
unsafe paths, oversized attribution files or opaque binary objects stop the
build. An SPDX identifier in a manifest is not sufficient license text.

Collection and compilation are offline/read-only dependency operations. Failed
builds preserve the previous executable and notice directory. Rebuilding an
unmodified generated directory replaces the pair; an unmanaged or manually
modified notice directory is preserved and produces an error. Keep additional
handwritten notices separately rather than editing generated output.

This inventory is not legal-compliance certification or a source-offer generator.
Review the actual terms and separately linked native/system libraries. `run`,
`emit-go` and direct `go build` do not create an application notice directory.

## C ABI and FFI

| Command | Purpose |
| --- | --- |
| `keika emit-c-abi -o <dir> <sources...>` | Write Go implementation/gateway, C header, and ABI manifest |
| `keika abi check --baseline <manifest> <sources...>` | Compare a new outgoing ABI against `kinmokusei_abi.json` |
| `keika ffi generate --manifest <file> -o <dir>` | Validate schema 1 and write `generated_ffi.go` |

`emit-c-abi` writes `generated.go`, `generated_cabi.go`, `kinmokusei_abi.h`, and `kinmokusei_abi.json`. ABI check succeeds for an identical fingerprint or additive symbols and reports breaking changes to standard error.

## Dependencies and projects

### Project creation

```text
keika new app [--name name] [--module path] <directory>
keika new library [--name name] [--module path] [--license identifier] <directory>
```

Available since v0.4.1. Options follow `app` or `library` and precede the
destination. The project name defaults to the destination's basename and the
module path to `example.com/<name>`. Both templates start at version `0.1.0`
with Go `1.23` as their minimum. The library template also records the supported
Kinmokusei version, Go backend, public entry and license metadata.

Creation requires an installed Go toolchain and initializes the lock and local
dependency state offline. It never downloads dependencies, initializes Git, or
overwrites an existing destination, including empty directories and symlinks.
The parent directory must exist. Preparation failures leave the destination
absent; a failure during final publication reports and retains partial output.
Success prints the created project location to stdout.

Both templates write `kinmokusei.toml`, `kinmokusei.lock`, `.gitignore`, a README
and `.kinmokusei/deps/` state. Apps include `main.km`; libraries include
`index.km` and `go.mod`. `--license` is library-only and defaults to `UNLICENSED`;
it records metadata without generating license text.

### Dependency operations

| Command | Purpose |
| --- | --- |
| `keika install --go-module [--offline] [--replace path] <module>@<version> [project]` | Add an exact Go module transactionally |
| `keika deps add [--offline] [--replace path] [--alias name] <module>@<version> [project]` | Add a source package and its automatic import alias when its manifest declares `[package]`, otherwise a Go module (`--alias` is source-only) |
| `keika deps update [--offline] <module>@<version> [project]` | Update one declared module to an exact version |
| `keika deps update [--offline] [module] [project]` | Select the latest version of one or all direct source packages |
| `keika deps list [project]` | Print locked source and Go dependencies |
| `keika deps fetch [--offline] [project]` | Restore exact locked packages and Go module files without relocking |
| `keika deps remove [--offline] <module> [project]` | Remove one declared module |
| `keika deps lock [--offline] [project]` | Resolve and write canonical lock/internal module state |
| `keika deps check [project]` | Validate manifest, lock, generated module files, and license hashes |
| `keika deps licenses [--strict] [project]` | Print module, license path, and SHA-256 hash; strict mode rejects unknowns |
| `keika target [project]` | Print locked `GOOS`, `GOARCH`, `CGO_ENABLED`, and tags |

The optional project argument defaults to the current directory. Complete versions, including complete pseudo-versions, are required. Add/update/remove/lock preserve prior state on failure.

Source packages currently require tagged versions matching their manifest; the
pseudo-version allowance above applies to Go dependencies. Source-free `check`,
`build`, `run`, and `emit-go` commands select the project's `main.km`, or a
library's declared package entry. See [external source packages](../guide/external-packages).

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

Plain `check` is silent on success. Source diagnostics and project/input errors
go to standard error. With valid command usage, `--json` writes one JSON object
to standard output, including source/project/load failures. Invalid options or
missing sources outside a project return usage status `2` on stderr instead;
they do not produce a diagnostic JSON object. See [Diagnostics](./diagnostics).

## Exit status

| Status | Meaning |
| --- | --- |
| `0` | Successful command or valid check |
| `1` | Invalid source, project/input failure, build or launch failure, incompatible ABI, or incomplete audit |
| `2` | Invalid command or option usage |

After a successful build, `keika run` returns the program's own normal exit code,
including nonzero codes. It forwards standard input/output/error without adding
an `exit status` message. Build and launch failures, or termination without a
normal exit code, return `1`. The temporary executable is removed when the
command finishes; generated Go and source maps remain in the managed directory.
