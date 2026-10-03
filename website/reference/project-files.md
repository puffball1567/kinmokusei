---
title: Project-file reference
description: Exact kinmokusei.toml sections, canonical kinmokusei.lock contents, target resolution, and dependency rules.
---

# Project-file reference

Projects use strict `kinmokusei.toml` input and a compiler-written canonical JSON `kinmokusei.lock`. Unknown/duplicate sections and keys are rejected.

The manifest accepts the sections and quoted string values documented here,
not every TOML feature. For example, write `cgo = "enabled"`, not a Boolean,
and write build tags as one comma-separated string, not a TOML array.

## Complete example

```toml
[project]
name = "example"
version = "0.1.0"
go-module = "example.com/example"
go-version = "1.23"

[target]
goos = "linux"
goarch = "amd64"
cgo = "disabled"
tags = "production"

[go.interop]
unsafe = "deny"

[go.dependencies]
"github.com/google/uuid" = "v1.6.0"
"example.com/local/api" = "v0.1.0"

[go.replacements]
"example.com/local/api" = "./local-api"
```

## `[project]`

| Key | Required | Meaning |
| --- | --- | --- |
| `name` | Yes | Project identity |
| `version` | Yes | Project version |
| `go-module` | Yes | Generated Go module path |
| `go-version` | Yes | Go language/toolchain version in `1.N` or `1.N.P` form |

All four keys are required. Source inputs are resolved below the project root; `source` is not a supported key.

## `[target]`

| Key | Values |
| --- | --- |
| `goos` | Go target OS |
| `goarch` | Go target architecture |
| `cgo` | `auto`, `enabled`, or `disabled` |
| `tags` | Canonicalized unique build tags |

Omitted GOOS/GOARCH resolve to compiler-host values, not ambient process overrides. Cross-target `auto` CGO becomes disabled. The lock controls package loading, checking, building, and cross-run rejection.

## `[go.interop]`

`unsafe` is `deny` by default and may be set to `allow` project-wide. Allowing it permits use of supported unsafe boundary shapes; it does not add memory-safety guarantees.

## Dependencies and replacements

`[dependencies]` records exact external `.km` package versions; `[replace]`
declares consumer-owned local overrides, including sibling paths and overrides
of transitive source dependencies. These are separate from the Go dependency
and replacement sections. See [external source packages](../guide/external-packages)
for the acquisition, automatic alias and local-development workflows.

Go dependencies require complete versions, including complete pseudo-versions;
source dependencies require complete tags matching their package manifests.
Go replacements are project-relative local paths inside the project root and
must correspond to declared Go dependencies. Source `[replace]` entries can
point to sibling projects; these distinct policies are not interchangeable.

Only explicit dependency commands may resolve or mutate the graph. Normal compilation validates the lock and uses the graph read-only/offline.

## `[package]` and `[exports]`

Applications need no `[package]` section. A distributable source library uses:

```toml
[package]
entry = "index.km"
min-kinmokusei = "0.4.4"
backend = "go"
license = "MIT"

[exports]
"format" = "src/format.km"
```

| Package key | Required | Meaning |
| --- | --- | --- |
| `entry` | Yes | Canonical relative `.km` file selected by the package module path |
| `min-kinmokusei` | Yes | Complete minimum compiler version, without leading `v` |
| `backend` | Yes | Currently `go` |
| `license` | Yes | Nonempty author-provided identifier; no license text is generated |

The library's `[project].go-module` is its canonical source module identity.
`[project].version` must match the requested tag without its leading `v`, and
the accompanying `go.mod` uses the same module path. Its `[project].go-version`
is also a minimum requirement on the consumer's selected Go version.

An `[exports]` key names a public submodule, and its value names the source
file. Paths are canonical slash-separated paths without a leading `./`, parent
traversal, or backslashes; source targets end in `.km` and stay inside the
library root, including after symlink resolution. Internal `.git` and
`.kinmokusei` paths cannot supply package source. A file-level `export` then
chooses the available declarations within that file. An undeclared submodule
is not public merely because its source file contains exports.

In a library, `[target].goos`/`goarch` constrain the consumer to the written
target; omitted values impose no corresponding source-package requirement.
`cgo = "enabled"` requires CGO, and every declared tag must be active. These
requirements are validated, not merged into or enabled on the consumer's target.
Dependency-owned local replacements are not inherited; the consumer supplies
any local overrides itself.

## `[imports]`

Available since v0.4.3. Each quoted key is a source import prefix; its
quoted value must exactly match a direct `[dependencies]` module:

```toml
[dependencies]
"example.com/command" = "v0.1.0"

[imports]
"cli" = "example.com/command"
```

`cli` resolves to the package entry; `cli/flags` resolves to the canonical
`example.com/command/flags` export. The same rules apply to `export ... from`.

`keika deps add` automatically writes an alias from the module's last path
component, skipping a trailing semantic major-version component such as `/v2`.
Use `--alias <name>` to override that choice. Automatic registration rejects
overlapping existing aliases without overwriting them, and participates in the
dependency/lock transaction. Updates preserve the choice; removal deletes the
corresponding aliases. Transitive dependencies do not acquire consumer aliases.

- Prefixes are canonical, slash-separated, non-relative paths. Characters are
  ASCII letters, digits, `_`, `@`, `.`, `-` and `/`; the first character must be
  a letter, digit, `_` or `@`. Empty paths, dot segments, trailing/repeated
  slashes, backslashes, drive paths and whitespace are rejected.
- `kinmokusei` and its subpaths are reserved. Aliases cannot equal, contain, or
  be a parent prefix of a declared canonical dependency path or the project's
  own module path.
- Matching uses slash boundaries and the longest alias prefix. Expansion
  happens once: alias chains, submodule targets, local filesystem targets,
  undeclared/transitive-only targets and Go-only dependencies are rejected.
- Resolution uses the importing package's own `[imports]`. Consumer aliases
  do not override dependency manifests. `import go` never expands these aliases.
- Aliases do not create dependencies or alter package/type identity. The lock
  keeps canonical module paths and hashes; changing `[imports]` requires an
  explicit `keika deps lock`, like other manifest changes. Normal compilation
  and editor operations do not rewrite the manifest or lock.

## Lock contents

The canonical lock records:

- manifest hash and selected Go version;
- resolved GOOS, GOARCH, tags, and CGO state;
- module graph and checksums;
- project-relative replacements;
- generated `go.mod` and `go.sum` hashes;
- recognized root license-file paths and hashes.

It contains no machine-specific absolute paths. Modified or missing generated module/lock/license files are validation errors.

Schema 4 additionally records source-package edges, hashes and license identifiers,
and includes the Go module file contents needed by `deps fetch`. Existing Go-only
schema 3 locks remain readable. Fetch verifies and restores the locked graph;
it does not re-resolve versions or modify the lock.

## License inventory

Recognized root files use `LICENSE`, `LICENCE`, and `COPYING` families. `NOTICE` alone is not classified as a license. The compiler reports paths and SHA-256 hashes; it does not infer SPDX identifiers from prose or claim legal compliance.
