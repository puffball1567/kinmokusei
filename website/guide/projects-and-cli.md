# Projects and CLI

The CLI keeps checking, generation, dependency changes, and ABI work explicit.

```sh
ontama check main.otm
ontama check --json main.otm
ontama run main.otm
ontama build -o app main.otm
ontama emit-go main.otm
```

An `ontama.toml` project may lock target OS/architecture, CGO, tags, unsafe interop, local replacements, and exact dependencies. `ontama.lock` is canonical and reproducible.

Dependency-changing commands are transactional. Ordinary compilation never updates the module graph as a side effect.

For the complete command list, see the [CLI reference](../reference/cli).
