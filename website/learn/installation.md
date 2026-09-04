---
title: Installation
description: Install the Kinmokusei compiler and verify the Go toolchain on Linux, macOS, or Windows.
---

# Installation

Kinmokusei is distributed as the `keika` command. Running, building, dependency resolution, and direct Go package imports use an installed Go toolchain.

## Requirements

| Requirement | Supported |
| --- | --- |
| Operating systems | 64-bit Linux, macOS, and Windows release targets listed below |
| Go toolchain | Go 1.23 through Go 1.27 |
| Kinmokusei source | Files ending in `.km` |

The release compiler is built with Go 1.27 so it can consume package export data from all supported toolchains. A compiler installed from source reads export data with the Go version used to build it.

## Install a release archive

Download the archive and `SHA256SUMS` from the [latest GitHub release](https://github.com/puffball1567/kinmokusei/releases/latest).

| Platform | Archive pattern |
| --- | --- |
| Linux x86-64 | `keika_<version>_linux_amd64.tar.gz` |
| Linux ARM64 | `keika_<version>_linux_arm64.tar.gz` |
| macOS Intel | `keika_<version>_darwin_amd64.tar.gz` |
| macOS Apple Silicon | `keika_<version>_darwin_arm64.tar.gz` |
| Windows x86-64 | `keika_<version>_windows_amd64.zip` |

Verify the checksum before moving the binary onto `PATH`:

::: code-group

```sh [Linux]
sha256sum -c SHA256SUMS
tar -xzf keika_<version>_linux_amd64.tar.gz
mkdir -p ~/.local/bin
install -m 0755 keika_<version>_linux_amd64/keika ~/.local/bin/keika
```

```sh [macOS]
shasum -a 256 -c SHA256SUMS
tar -xzf keika_<version>_darwin_arm64.tar.gz
mkdir -p ~/.local/bin
install -m 0755 keika_<version>_darwin_arm64/keika ~/.local/bin/keika
```

```powershell [Windows PowerShell]
Get-FileHash .\keika_<version>_windows_amd64.zip -Algorithm SHA256
Expand-Archive .\keika_<version>_windows_amd64.zip
```

:::

On Windows, move `keika.exe` into a dedicated tools directory and add that directory to the user `Path` setting.

## Install with Go

If Go is already installed, install the latest tagged compiler directly:

```sh
go install github.com/puffball1567/kinmokusei/cmd/keika@latest
```

For a reproducible installation, replace `latest` with the exact `vX.Y.Z` tag shown on the release page. Use the same exact release for the compiler and editor extension.

Go writes the command to `GOBIN`, or to the default Go bin directory when `GOBIN` is unset. That directory must be on `PATH`.

## Verify the installation

```sh
keika version
go version
```

An untagged compiler built from a checkout reports `devel`.

## Install the editor extension

The matching `.vsix` is attached to each GitHub release. In Visual Studio Code, run **Extensions: Install from VSIX**, select the downloaded file, and ensure `keika` is visible on the editor process's `PATH`.

The extension and compiler should use the same release. See [Editor and diagnostics](../guide/editor) for the LSP contract and other editor clients.

## Build from a checkout

From the repository root:

```sh
go build -trimpath -o ./keika ./cmd/keika
./keika version
```

Continue with the [five-minute quick start](./quick-start).
