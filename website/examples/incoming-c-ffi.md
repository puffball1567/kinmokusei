---
title: Generate an incoming C binding
description: Validate a schema 1 manifest and generate a private, gofmt-formatted cgo package.
---

# Generate an incoming C binding

Incoming FFI starts from an explicit ownership manifest. The generator never infers lifetime from a pointer or symbol name.

## Minimal scalar manifest

<<< ../manifests/scalar-ffi.json{json}

The corresponding C header contract is:

```c
#include <stdint.h>

int32_t fixture_value(void);
```

Generate the private package:

```sh
keika ffi generate \
  --manifest ./scalar-ffi.json \
  -o ./internal/fixture
```

The command validates the complete JSON document and writes `internal/fixture/generated_ffi.go`. For this manifest, the generated Go API contains:

```go
func Value() int32
```

The documentation check runs this exact generation command and verifies the generated source is gofmt-formatted.

## Integrate it safely

The header must be visible to cgo and the native symbol must be linked for a full application build. Keep generated bindings under `internal/`; expose a small application wrapper instead of leaking raw boundary details.

For strings, byte buffers, arrays, handles, callbacks, and status/out functions, declare ownership and release behavior in the manifest. See [C ABI and FFI](../guide/c-ffi) for lifetime and thread-policy rules.
