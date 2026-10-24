# kinmokusei/input

Compiler-managed, source-written input adapters for reading fresh raw DTOs,
constructing checked native values, and publishing only after successful
validation. These are development APIs after v0.4.9.

See the [input adapter guide](../../website/guide/input-adapters.md) for the
boundary and aliasing contract, and the
[standard-module reference](../../website/reference/standard-library.md) for
`decodeInput` and `bindInput` signatures. The module grants no additional opaque
Go write or unsafe capability.
