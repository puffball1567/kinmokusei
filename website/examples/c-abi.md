---
title: Export a C ABI library
description: Generate a stable C header, Go gateway, ABI manifest, and fingerprint from fixed-width Kinmokusei functions.
---

# Export a C ABI library

This recipe exports one fixed-width integer function and one floating-point function through explicit stable symbols.

## Project tree

```text
c-library/
└── library.km
```

## Source

<<< ../snippets/c-abi.km{ts}

## Generate artifacts

```sh
keika check library.km
keika emit-c-abi -o ./generated-c-abi library.km
```

Generated tree:

```text
generated-c-abi/
├── generated.go
├── generated_cabi.go
├── kinmokusei_abi.h
└── kinmokusei_abi.json
```

`generated.go` contains the ordinary Go implementations. `generated_cabi.go` contains panic-isolating cgo gateways. The header exposes explicit status/out signatures, while the canonical JSON manifest supplies the ABI fingerprint.

## Check a later version

Keep the previous manifest and compare before publishing:

```sh
keika abi check --baseline ./previous/kinmokusei_abi.json library.km
```

Adding a symbol is compatible. Removing or renaming a symbol, changing parameter/result transport, or changing the gateway contract is reported as breaking.

Machine-width `int`/`uint`, strings, slices, maps, objects, classes, interfaces, pointers, channels, and errors are deliberately rejected from this stable outgoing boundary.

See [C ABI and FFI](../guide/c-ffi) for status codes, boolean normalization, ownership, incoming bindings, and thread policies.
