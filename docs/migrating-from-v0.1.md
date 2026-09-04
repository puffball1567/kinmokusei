# Migrating from v0.1

Version 0.1 was released as **OnsenTamago**. Beginning with v0.2, the language
is named **Kinmokusei** (金木犀, Japanese: きんもくせい, romanized
*kin-moku-sei*). Because the project is
still pre-1.0, this is an intentional clean break rather than a permanent
compatibility layer.

Update project files and commands as follows:

| v0.1 | v0.2 and later |
| --- | --- |
| `ontama` | `keika` |
| `*.otm` | `*.km` |
| `ontama.toml` | `kinmokusei.toml` |
| `ontama.lock` | `kinmokusei.lock` |
| `.ontama/` | `.kinmokusei/` |
| `import ... from "ontama/http"` | `import ... from "kinmokusei/http"` |
| `github.com/puffball1567/onsentamago` | `github.com/puffball1567/kinmokusei` |
| `onsentamago.server.path` | `kinmokusei.server.path` |

Remove the old `.ontama` compiler-state directory instead of copying it.
`keika` will regenerate `.kinmokusei` from the renamed `.km` sources and the current
lock file. The compiler still accepts `.otm` source files during migration, but
new source and generated examples use `.km`.

Generated C boundaries also use the new identity. Regenerate C ABI and incoming
FFI artifacts, then rebuild their consumers. This changes generated
`ONTAMA_ABI_*` macros, `ontama_cffi_*` support symbols, default ABI artifact
names, and compiler-owned `__ontama*` Go identifiers to their `KINMOKUSEI_ABI_*`,
`kinmokusei_cffi_*`, and `__kinmokusei*` equivalents. Explicit C symbol strings chosen
by application authors remain application-owned and need changing only when
the application wants the new naming convention.

The v0.1 Visual Studio Code extension and compiler should be removed together.
Install matching Kinmokusei versions so the `.km` language identity and
`keika lsp --stdio` command stay aligned.
