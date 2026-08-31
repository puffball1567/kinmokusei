# OnsenTamago for Visual Studio Code

This directory contains the official thin Visual Studio Code client for
OnsenTamago. It recognizes `.otm` files, provides syntax highlighting, and
launches the language server as:

```sh
ontama lsp --stdio
```

The extension does not duplicate compiler or language-analysis logic. Hover,
completion, navigation, rename, diagnostics, symbols, and signature help are
provided by the versioned language server distributed with the `ontama` CLI.

## Configuration

`onsentamago.server.path` selects the `ontama` executable. The default resolves
`ontama` from the editor process's `PATH`. After changing the setting, the
extension serially stops the old server and starts the configured executable.
Use **OnsenTamago: Restart Language Server** to retry a failed startup or restart
the active server manually.

The client intentionally selects saved `file` documents only. Untitled buffers
are excluded because relative imports, module discovery, and diagnostics depend
on a real source path.

## Development

From this directory:

```sh
npm install
npm test
npm run test:e2e
npm run package:vsix
```

The end-to-end test builds the current `ontama` command, starts a real Extension
Development Host, and verifies language identification, hover, live diagnostics,
diagnostic recovery, and language-server restart. It uses the local stable Visual
Studio Code executable when available; set `VSCODE_EXECUTABLE_PATH` to select a
different executable. If no local executable is found, the test runner obtains
the minimum supported Visual Studio Code release.

The package command writes `dist/onsentamago-<version>.vsix`. It creates the
archive twice with fixed timestamps, checks byte-for-byte reproducibility, and
verifies that runtime assets and dependencies are present while tests, scripts,
lockfiles, and development dependencies are absent. The matching VSIX is
attached to each GitHub release with the `ontama` binaries; marketplace
publication remains a separate release decision.

For interactive development, open this directory in Visual Studio Code and
start an Extension Development Host. The `ontama` executable must be on `PATH`
or configured through `onsentamago.server.path`.

Other LSP-capable editors do not need this extension; configure them to launch
`ontama lsp --stdio` for `.otm` files.
