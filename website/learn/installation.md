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

Verify only the downloaded archive: `SHA256SUMS` also lists other platforms and
the editor extension, which need not be present locally. The commands below
use v0.4.10; set `release_version` to the tag's numeric version if installing a
different release. Choose the archive matching your OS and architecture.

::: code-group

```sh [Linux]
release_version=0.4.10
release_archive="keika_${release_version}_linux_amd64.tar.gz"
awk -v archive="$release_archive" '$2 == archive { print }' SHA256SUMS | sha256sum -c - || exit 1
tar -xzf "$release_archive"
mkdir -p ~/.local/bin
install -m 0755 "keika_${release_version}_linux_amd64/keika" ~/.local/bin/keika
```

```sh [macOS]
release_version=0.4.10
release_archive="keika_${release_version}_darwin_arm64.tar.gz"
awk -v archive="$release_archive" '$2 == archive { print }' SHA256SUMS | shasum -a 256 -c - || exit 1
tar -xzf "$release_archive"
mkdir -p ~/.local/bin
install -m 0755 "keika_${release_version}_darwin_arm64/keika" ~/.local/bin/keika
```

```powershell [Windows PowerShell]
$release_version = "0.4.10"
$release_archive = "keika_${release_version}_windows_amd64.zip"
$checksum_lines = @(Get-Content .\SHA256SUMS | Where-Object { ($_ -split '\s+', 2)[1] -eq $release_archive })
if ($checksum_lines.Count -ne 1) { throw "Expected one checksum for $release_archive" }
$expected_hash = ($checksum_lines[0] -split '\s+', 2)[0]
$actual_hash = (Get-FileHash -LiteralPath $release_archive -Algorithm SHA256).Hash
if ($actual_hash -ne $expected_hash) { throw "Archive checksum mismatch" }
Expand-Archive -LiteralPath $release_archive
```

:::

On Windows, move `keika.exe` into a dedicated tools directory and add that directory to the user `Path` setting.

The Go compiler itself is not bundled. The `keika` binary includes Go runtime
and standard-library code; release archives carry `LICENSE`,
`THIRD_PARTY_NOTICES.md` and `licenses/go/`. Retain these materials when
redistributing the compiler. See the repository's
[third-party notices](https://github.com/puffball1567/kinmokusei/blob/main/THIRD_PARTY_NOTICES.md).

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

## Update the compiler

::: info Available since v0.4.8
Compilers through v0.4.7 must first be updated using a release archive or
`go install` above. After installing v0.4.8, use the following command for
subsequent updates.
:::

```sh
keika update --check
keika update
keika version
```

`--check` contacts the official release service without changing the installation.
`update` selects the latest stable GitHub release; choose an exact stable tag
with `keika update --version vX.Y.Z`. Downgrades are rejected. A `devel` build
requires an explicit `--version` before replacing itself.

Automatic replacement currently supports Linux and macOS. On Windows,
`--check` works and the update command directs you to the release archive
instead of attempting to replace the running executable.

Downloads use HTTPS and the release's `SHA256SUMS`. The entire release bundle,
including `LICENSE`, `THIRD_PARTY_NOTICES.md` and `licenses/go/`, is retained.
The downloaded compiler must report the expected version before the launcher
is switched. Failed downloads, validation or switching leave the existing
compiler in place. Checksums detect download corruption; they are not an
independent signature of the publisher.

The command needs write access to the launcher's directory and release storage;
it never elevates privileges or invokes a package manager. An existing
`kinmokusei/<version>/keika` installation reached through a symlink keeps that
layout and older versions. Other installations use `keika-releases/<version>/`
beside the invoked launcher, replacing the launcher with a symlink. Existing
release directories are never overwritten. A concurrent update or an existing
target directory produces an error instead of risking another installation.

Self-update does not change your Go toolchain, editor extension, project
manifest or lock. Install the matching VSIX separately after updating.

## Switch compiler versions

::: info Available since v0.4.10
Update to v0.4.10 or later once to enable `keika use` and its independent
`keika-use` companion. Older compilers do not provide these commands.
:::

On Linux/macOS, select an exact official stable version, including an older one:

```sh
keika use v0.4.8
keika version
keika-use v0.4.10
keika version
```

The first selection saves an independent management executable and creates
`keika-use` beside your existing launcher. Use this companion command to switch
again if the selected older compiler has no `use` command. The success message
prints its exact path. The selected compiler, not the manager, handles normal
compilation; there is no wrapper process on build/run paths.
Selecting a version from a newer compiler that supports `use` also refreshes
the management snapshot. Invoking the independent companion reuses its existing
snapshot; it is not a separate automatically updating package manager.

An uncached version is fetched from the official Release, using the same HTTPS,
checksum, archive, notice and executable-version checks as `update`. A complete
local bundle recorded by the new updater is checked against its file-hash receipt
and reused without network access. Force cache-only operation with:

```sh
keika-use --offline v0.4.8
```

An older installation without a receipt is adopted only after all bundle files
match a freshly verified official archive. A missing cache under `--offline`, a
changed bundle, a symlink inside the bundle, a target mismatch or an unrelated
`keika-use` file is an error, not permission to overwrite it. Local receipts detect
accidental changes; they are not publisher signatures or protection against a
user with write access to the entire installation.

Switching is explicit and atomic, shares the update lock, preserves old releases
and bundled notices, and never changes `PATH`, the Go toolchain, the VSIX or
project files. Only stable tags in `vX.Y.Z` form are accepted. `keika update`
remains upgrade-only. Windows users select a release archive manually.

### List retained and published versions (development)

The following inventory commands are development additions after v0.4.10:

```sh
keika versions
keika versions --remote
keika-use versions
```

The first and third commands inspect only this installation's retained bundles,
including inactive versions, without network access or execution of cached
compilers. `active` identifies the selected compiler; receipt/inventory status
distinguishes managed, unmanaged and invalid bundles. These labels do not
automatically repair or remove files. Other manually installed copies are not
discovered by a whole-system search.

`--remote` explicitly fetches the official stable release candidates for the
current platform. It does not install them; archive verification still happens
when selecting a version. The development compiler also accepts
`keika use 0.4.10` as shorthand for `keika use v0.4.10`.
See the [CLI inventory contract](../reference/cli#version-inventory-development-after-v0410).

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
