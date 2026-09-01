# CLI reference

| Command | Purpose |
| --- | --- |
| `ontama version` | Print the compiler version |
| `ontama check [--json] <inputs...>` | Lex, parse, resolve, and type-check without generation |
| `ontama run <input>` | Generate Go and run the program |
| `ontama build [-o path] <input>` | Generate Go and build a native executable |
| `ontama emit-go <input>` | Write deterministic formatted Go |
| `ontama emit-c-abi -o <dir> <input>` | Generate an outgoing C ABI gateway |
| `ontama abi check --baseline <manifest> <input>` | Detect C ABI changes |
| `ontama ffi generate --manifest <file> -o <dir>` | Generate a checked incoming C binding |
| `ontama install --go-module <module@version>` | Add an exact Go module transactionally |
| `ontama deps check` | Verify the locked project graph |
| `ontama licenses` | Report external Go module licenses |
| `ontama interop audit [--json] <packages...>` | Measure usable public Go API shapes |
| `ontama lsp --stdio` | Run the language server |

Invalid command usage exits with status 2. Invalid source or project input exits with status 1. A successful command exits with status 0.
