---
title: Diagnostics reference
description: Kinmokusei diagnostic streams, JSON schema, positions, categories, and process exit status.
---

# Diagnostics reference

Diagnostics identify the failed rule and original `.km` source range. Generated-Go validation is a final invariant; normal source errors should be reported before generation.

## Plain mode

```sh
keika check src/main.km
```

Valid input is silent. Source diagnostics and project/input errors are written to standard error.

## JSON mode

```sh
keika check --json src/main.km
```

After valid command-line parsing and source selection, the command writes one
object to standard output, including source and project/load failures:

```json
{
  "valid": false,
  "diagnostics": [
    {
      "message": "cannot use string as int",
      "path": "src/main.km",
      "start": { "line": 3, "column": 10, "offset": 52 },
      "end": { "line": 3, "column": 17, "offset": 59 }
    }
  ]
}
```

Lines and columns are one-based. Offsets are zero-based UTF-8 byte offsets. Ranges are half-open: `start` is included and `end` is excluded.

A source diagnostic leaves the optional top-level `error` absent. An input, locked-project, or package-loader failure sets `error` and leaves `diagnostics` empty:

```json
{
  "valid": false,
  "diagnostics": [],
  "error": "project error description"
}
```

## Categories

The compiler reports source-positioned failures for:

- lexical and syntax rules with parser recovery;
- name, scope, visibility, and import resolution;
- type identity, assignability, arity, and generic constraints;
- mutability, addressability, and invalid update targets;
- nullable proof invalidation and incomplete constructor initialization;
- invalid `Result`, exception, or `Task` state/use;
- package, target, CGO, unsafe, lock, and dependency boundaries;
- generated public Go or C ABI name/type conflicts.

Diagnostics are compatibility-sensitive and have regression coverage. Important safety diagnostics should identify the write, capture, alias, or flow join that invalidated a proof when applicable.

## Exit status

| Status | Check meaning |
| --- | --- |
| `0` | Source is valid |
| `1` | Source diagnostic or input/project/load failure |
| `2` | Invalid command usage |

Editor clients use the same checker through `keika lsp --stdio` and convert positions to LSP UTF-16 coordinates.

Usage errors are not JSON diagnostic reports: an unknown option, or no source
arguments outside a project, returns status `2`, writes a usage message to
stderr and leaves stdout empty even if `--json` was supplied. A successful
check writes `{"valid": true, "diagnostics": []}` and returns status `0`.

The documentation suite executes a known-invalid nullable example in both plain and JSON modes. It asserts the diagnostic fragment and the JSON path, line, and column so the examples and documented envelope cannot drift independently.
