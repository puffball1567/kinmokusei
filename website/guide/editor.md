---
title: Editor and diagnostics
description: Configure the official Visual Studio Code extension or connect another LSP client to the Kinmokusei compiler.
---

# Editor and diagnostics

The language server ships inside the same `keika` binary as the compiler. Editor behavior and CLI checking therefore use the same lexer, parser, resolver, type checker, target loader, and diagnostics.

## Visual Studio Code

Download the matching `.vsix` from the [current Kinmokusei release](https://github.com/puffball1567/kinmokusei/releases/latest), then run **Extensions: Install from VSIX** in Visual Studio Code.

The extension provides `.km` syntax highlighting and launches:

```sh
keika lsp --stdio
```

Ensure `keika` is visible on the editor process's `PATH`. If it is not, set the extension's `kinmokusei.server.path` option to the executable. Keep the extension and compiler on the same release.

## Implemented language features

- source diagnostics with UTF-16-accurate editor positions;
- hover, definitions, references, and document symbols;
- scope-safe rename for values, types, enums, classes, structs, fields, methods, interface implementation families, imports, and type parameters;
- lexical, import, Go package/value, and Kinmokusei member completion;
- signature help for Kinmokusei callables, constructors, compiler built-ins, Go functions, and Go methods;
- multiple unsaved documents and transactional incremental synchronization;
- asynchronous request cancellation and stale-result suppression.

External Go declarations remain read-only. Rename validates the edited overlay and rejects capture, collisions, or a program that no longer resolves to the same declarations.

## Other LSP clients

Any editor that supports the Language Server Protocol can launch `keika lsp --stdio` for `.km` files. The server accepts only standard input/output transport and no positional source arguments.

The LSP never acquires dependencies, changes the manifest, or mutates the module graph implicitly.

## Command-line diagnostics

Plain checking is silent on success and writes source-positioned diagnostics to standard error on failure:

```sh
keika check src/main.km
```

Tools and AI integrations should use the stable JSON envelope:

```sh
keika check --json src/main.km
```

See [Diagnostics reference](../reference/diagnostics) for positions, error categories, and exit status.
