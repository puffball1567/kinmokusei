## Summary

Describe the user-visible compiler, language, interop, editor, or documentation
behavior changed by this pull request.

## Verification

- [ ] Focused tests cover the changed behavior and relevant edge cases.
- [ ] Go-equivalent runtime behavior is compared with an independently
      handwritten Go oracle.
- [ ] `go test ./...` passes.
- [ ] `./scripts/coverage.sh` passes when implementation code changed.
- [ ] Generated Go builds and produces the expected result where applicable.

## Compatibility

- [ ] Syntax or generated API changes are documented.
- [ ] Go package interop remains valid on supported toolchains.
- [ ] C ABI/FFI changes include ABI compatibility tests when applicable.
- [ ] No generated output contains machine-specific paths or private data.
