---
title: Migrate from v0.1
description: Rename v0.1 source, project, editor, generated-code, and C-boundary identities for Kinmokusei v0.2.
---

# Migrate from v0.1

Version 0.1 was released as **OnsenTamago**. Version 0.2 renames the language to
**Kinmokusei** (金木犀), with the suggested English pronunciation
*kin-moh-koo-say*, and
intentionally makes a clean pre-1.0 break from the old tool identity.

| v0.1 | v0.2 and later |
| --- | --- |
| `ontama` | `keika` |
| `*.otm` | `*.km` |
| `ontama.toml` | `kinmokusei.toml` |
| `ontama.lock` | `kinmokusei.lock` |
| `.ontama/` | `.kinmokusei/` |
| `ontama/http` | `kinmokusei/http` |
| `github.com/puffball1567/onsentamago` | `github.com/puffball1567/kinmokusei` |
| `onsentamago.server.path` | `kinmokusei.server.path` |

Delete `.ontama` after renaming the source and project files. The `keika`
command will regenerate `.kinmokusei`; old generated state should not be copied.
Existing `.otm` source remains accepted during migration, while new files use
`.km`.

Regenerate checked C ABI and incoming FFI artifacts because compiler-owned
headers, support symbols, manifests, and generated Go identifiers now use the
Kinmokusei identity. Explicit external C symbol strings remain under application
control.

Remove the v0.1 editor extension and install the Kinmokusei extension that matches
the compiler version. It recognizes `.km` and launches `keika lsp --stdio`.
