# Editor setup

The official Visual Studio Code extension launches `ontama lsp --stdio`, so diagnostics and language-server behavior stay tied to the compiler version.

Download the matching `.vsix` from the [OnsenTamago release](https://github.com/puffball1567/onsentamago/releases/latest), then install it through Visual Studio Code's **Install from VSIX** command.

The extension provides `.otm` highlighting and connects the compiler's diagnostics, hover, definitions, references, rename, symbols, completion, and signature help. Configure `onsentamago.path` only when `ontama` is not discoverable on `PATH`.

Other LSP clients can launch the same server directly:

```sh
ontama lsp --stdio
```
