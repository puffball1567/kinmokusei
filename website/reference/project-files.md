---
title: Project-file reference
description: Exact kinmokusei.toml sections, canonical kinmokusei.lock contents, target resolution, and dependency rules.
---

# Project-file reference

Projects use strict `kinmokusei.toml` input and a compiler-written canonical JSON `kinmokusei.lock`. Unknown/duplicate sections and keys are rejected.

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

Dependencies require complete versions, including complete pseudo-versions. Initial replacements are project-relative local paths inside the project root and must correspond to declared dependencies.

Only explicit dependency commands may resolve or mutate the graph. Normal compilation validates the lock and uses the graph read-only/offline.

## Lock contents

The canonical lock records:

- manifest hash and selected Go version;
- resolved GOOS, GOARCH, tags, and CGO state;
- module graph and checksums;
- project-relative replacements;
- generated `go.mod` and `go.sum` hashes;
- recognized root license-file paths and hashes.

It contains no machine-specific absolute paths. Modified or missing generated module/lock/license files are validation errors.

## License inventory

Recognized root files use `LICENSE`, `LICENCE`, and `COPYING` families. `NOTICE` alone is not classified as a license. The compiler reports paths and SHA-256 hashes; it does not infer SPDX identifiers from prose or claim legal compliance.
