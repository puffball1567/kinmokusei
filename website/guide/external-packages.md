---
title: Use external Kinmokusei packages
description: Define, acquire, lock, replace, and consume source packages through ordinary imports.
---

# Use external Kinmokusei packages

This feature is available since v0.4.1. Source
packages use ordinary imports; Go packages continue to use `import go`.
Both are checked and compiled into the application's Go output.

## Define a library

Generate a minimal library with:

```sh
keika new library --module example.com/greeting --license MIT greeting
cd greeting
keika check
keika emit-go -package greeting
```

This creates an exported `greet` function, matching source/Go manifests and an
offline initial lock. Set `--module` to your real repository path before
publishing. `--license` only records metadata; provide the appropriate license
text yourself. Without that option the generated library uses `UNLICENSED`.

A library has a `kinmokusei.toml` at its root:

```toml
[project]
name = "greeting"
version = "0.1.0"
go-module = "example.com/greeting"
go-version = "1.23"

[package]
entry = "index.km"
min-kinmokusei = "0.4.1"
backend = "go"
license = "MIT"

[exports]
"format" = "src/format.km"
```

The project `go-module` is also the source package's public module path.
`entry` is the file imported by that path. Each `exports` entry explicitly
maps a public submodule to a source file. Export paths and source paths are
canonical slash-separated paths, without leading `./` or parent traversal.
Use source `export` declarations or re-exports to define the public symbols.

Internal files use relative imports. They are not automatically public:
`example.com/greeting/format` is available, while an undeclared submodule is
rejected. Relative imports cannot leave the library root, including through
symlinks. Each library must declare its own external dependencies.

Dependency operations inspect the library entry and every declared public
submodule, following relative imports, source re-exports and declared source
dependencies. Unreferenced examples, tests and nested projects do not contribute
Go imports to the consuming build, including for replacements inside the
application directory. Reachable missing files, import cycles and syntax errors
are diagnosed before the new lock is written. Declared manifest dependencies
remain part of the graph even when no public source imports them.

For tagged distribution, include a `go.mod` with the same module path:

```go
module example.com/greeting

go 1.23
```

Commit the sources and manifests, and tag the repository `v0.1.0`. The tag's
version must equal the library's project version prefixed with `v`. The initial
source-package format uses complete tagged versions, not floating branches or
pseudo-versions. Acquisition uses the Go module download/cache infrastructure,
including its repository and proxy conventions; see the
[Go module reference](https://go.dev/ref/mod#go-mod-download).

Declare Go dependencies in the existing `[go.dependencies]` section and source
dependencies in `[dependencies]`. Source packages carry their Go requirements
into the consuming build. A dependency's replacement paths never authorize
access to arbitrary local directories: overrides belong to the consuming project.
Every non-standard Go import in a source library must be covered by its own
`[go.dependencies]`. For now, co-located Go implementation helpers need a
separate Go module declared there; they are not automatically exposed by the
source package's module path.

## Add and use a package

From an application containing the usual project manifest:

```sh
keika deps add example.com/greeting@v0.1.0
keika check
keika run
keika build
```

`deps add` recognizes a module with a `[package]` declaration as a Kinmokusei
dependency and writes it under `[dependencies]`. Modules without that declaration
retain the existing Go dependency behavior. `keika install --go-module` remains
the explicit Go-only command.

Use the declared module path in source:

```ts
import { greet } from "example.com/greeting"
import { format } from "example.com/greeting/format"
```

Without source arguments, `check`, `build`, `run`, and `emit-go` use
`main.km` in an application project, or the declared package entry in a library
project. Explicit source arguments remain supported. Files needed by the entry
are loaded through imports, not by concatenating every source in a directory.

## Short import names

In the next patch, `keika deps add` automatically registers a short source
import name and updates the lock in the same transaction:

```sh
keika deps add example.com/greeting@v0.1.0
keika check
```

The command writes these entries; no manual manifest edit is needed:

```toml
[dependencies]
"example.com/greeting" = "v0.1.0"

[imports]
"greeting" = "example.com/greeting"
```

```ts
import { greet } from "greeting"
import { format } from "greeting/format"
export { greet } from "greeting"
```

The default alias is the module path's last component; a trailing `/v2`, `/v3`,
etc. is skipped (`example.com/greeting/v2` becomes `greeting`). Package metadata
does not choose the consumer's alias. If the name conflicts with an existing
alias or a reserved/canonical path, the command fails without changing the
manifest or lock. Choose another name directly in the command:

```sh
keika deps add --alias hello example.com/greeting@v0.1.0
```

`--alias` is for Kinmokusei source dependencies only. `--offline` requires the
dependencies to be already cached or locally replaced. The alias only changes
source spelling: versions, acquisition, lock entries,
hash checks and type identity still use the canonical module path. Canonical
and short imports may coexist without loading a second copy of the module.

Aliases belong to the importing project's manifest. A library can define its
own aliases for its direct dependencies; it never inherits the application's
aliases, and its aliases never leak into the application or sibling libraries.
Relative imports, standard-library imports and `import go` are unchanged.

Continue to use canonical module paths with `keika deps add/update/remove`.
Dependency edits preserve aliases; removing a direct dependency removes aliases
targeting it, even when another library still needs that package transitively.
Only directly added packages receive aliases, not their transitive dependencies.
Manual `[imports]` edits remain possible; unlike `deps add`, they require an
explicit `keika deps lock` afterwards.
See the [alias rules](../reference/project-files#imports) for validation details.

## Develop two repositories together

The consumer can replace a package with a sibling checkout before publishing
a tag:

```sh
keika deps add --offline --replace ../greeting example.com/greeting@v0.1.0
keika check
keika run
```

This records:

```toml
[dependencies]
"example.com/greeting" = "v0.1.0"

[imports]
"greeting" = "example.com/greeting"

[replace]
"example.com/greeting" = "../greeting"
```

The version and module identity still have to match the library manifest.
An explicitly declared replacement may point outside the application root.
A root override may also replace a transitive source dependency. Replacements
declared by dependency manifests are not inherited.

Local source edits are live and do not require another lock operation.
Changing a library manifest, its dependency graph, version, or public entry
mapping requires `deps lock` or an explicit update. Local replacements are
development inputs, not immutable content snapshots; reproducing their source
contents requires the same checkouts.

The compiler and LSP use the same resolver. Diagnostics, completion and
definition navigation retain external source locations, while generated symbol
identities use module-relative names rather than machine-specific cache paths.
The consuming project can supply this context as an editor workspace folder;
its source files do not need to stay open. Workspace changes refresh diagnostics
without acquiring dependencies. See [editor setup](editor).

## Update, inspect and restore

```sh
keika deps list
keika deps check
keika deps update example.com/greeting
keika deps update example.com/greeting@v0.1.0
keika deps remove example.com/greeting
keika deps fetch
```

An update without a version selects the latest version of a declared source
package. Omitting the module updates all direct source dependencies. An exact
version supports either upgrade or downgrade and remains the update form for
Go dependencies. `--offline` requires cached exact versions; it does not discover
the latest remote version.

Removing a direct source dependency retains its local replacement if another
dependency still needs it. With a matching lock, replacements for packages that
become unreachable are removed together, without requiring their old checkouts
to remain available. A failed dependency change restores the previous manifest
and lock.

`deps fetch` restores the graph recorded in the lock, including its Go module
files, without changing the manifest or lock. This is the command to use after
cloning a project with an existing lock or clearing its dependency cache.
A cache miss identifies the missing module and version.

Normal checking, compilation, execution and editor analysis never acquire
dependencies or change the manifest/lock. They disable dependency downloads,
including direct private-module VCS and automatic toolchain downloads.
Build the dependency graph explicitly before those operations.

## Integrity and compatibility

Lock schema 4 records source package identities, exact versions, dependencies,
declared license identifiers, manifest/content hashes, replacement paths and
Go requirements, alongside the existing Go graph and restorable Go module files.
Go-only schema 3 locks remain readable; regenerate them explicitly to obtain
restorable schema 4 payloads.

Published source packages are checked against their locked full-content hash.
Changed cache content is an error, not a reason to silently update the lock.
Library minimum Kinmokusei/Go versions and declared OS, architecture, CGO and
build-tag requirements are checked against the consumer's target. Unversioned
development binaries currently use the compatibility floor `0.4.2`.

The initial source graph selects one exact version per module. Conflicting
direct/transitive source requirements are diagnosed with both versions instead
of silently choosing one. Align those requirements before relocking. Go
requirements retain Go's version-selection behavior. The license field is
author-provided metadata, not a legal-compliance determination.
