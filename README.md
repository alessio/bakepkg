<div align="center">

# 📦 bakepkg

**The Modern macOS Package Builder**

[![Go](https://github.com/alessio/bakepkg/actions/workflows/build.yml/badge.svg)](https://github.com/alessio/bakepkg/actions/workflows/build.yml)
[![GoDoc](https://godoc.org/al.essio.dev/cmd/bakepkg?status.svg)](https://pkg.go.dev/al.essio.dev/cmd/bakepkg)
[![Go Report Card](https://goreportcard.com/badge/github.com/alessio/bakepkg)](https://goreportcard.com/report/github.com/alessio/bakepkg)
[![License](https://img.shields.io/github/license/alessio/bakepkg.svg)](https://github.com/alessio/bakepkg/blob/main/LICENSE)
[![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/alessio/bakepkg)](https://github.com/alessio/bakepkg/releases)

<p align="center">
  <br />
  <b>bakepkg</b> is a robust, modern CLI tool and Go library for building macOS installer packages (<code>.pkg</code>).<br />
  It wraps native macOS tooling with integrated code-signing, Apple Notarization, and auto-generated install scripts.
  <br />
</p>

</div>

---

## ✨ Features

- 🚀 **Simple:** Build a signed, notarized `.pkg` from a single JSON config.
- ⚙️ **Auto-scripting:** Generates `postinstall`, `uninstall.sh`, and upgrade scripts automatically — no manual scripting required.
- 🛤️ **PATH Integration:** Writes `/etc/paths.d` and `/etc/manpaths.d` entries so binaries and man pages are immediately discoverable.
- 🖥️ **Distribution UI:** First-class support for Welcome screens, READMEs, Licenses, and Background images (including Dark Mode).
- 👤 **Single-User Mode:** Generates distribution packages with per-user installation domain choices.
- 🔐 **Integrated Signing:** Automatically signs binaries and the final installer with your Developer ID.
- 🍎 **Modern Notarization:** Directly integrates with Apple's Notary API — no more `xcrun altool`.
- 🧹 **Automatic Sanitization:** Strips `com.apple.quarantine` attributes to ensure smooth Gatekeeper validation.
- 🔬 **Simulation Mode:** Dry-run mode prints all commands without touching the system.
- 📚 **Go Library:** Use as a fluent API in your own Go tooling.

## 📦 Installation

### Pre-built Binaries

Download the latest pre-built binary for macOS from the [GitHub Releases page](https://github.com/alessio/bakepkg/releases).

1. Visit the [releases page](https://github.com/alessio/bakepkg/releases).
2. Download the archive matching your architecture (`x86_64` or `arm64`).
3. Extract and move the `bakepkg` binary to a directory in your `$PATH` (e.g., `/usr/local/bin`).

### From Source

Requires Go 1.24 or later.

```sh
go install al.essio.dev/cmd/bakepkg@latest
```

To build from a local checkout:

```sh
make build
```

## 🚀 Usage

```sh
bakepkg [OPTION...]
```

`bakepkg` reads its configuration from `bakepkg.json` in the current directory by default.

```sh
# Use default config (bakepkg.json)
bakepkg

# Specify a custom config file
bakepkg -config custom.json

# Verbose output
bakepkg -verbose
```

## ⚙️ Options

| Flag | Description | Default |
| :--- | :--- | :--- |
| `-config` | Path to JSON configuration file | `bakepkg.json` |
| `-verbose` | Enable verbose output | `false` |
| `-debug` | Enable debug output | `false` |
| `-version`, `-V` | Print version information and exit | |
| `-help`, `-h` | Display the help message and exit | |

## 📄 JSON Configuration

All build settings are defined in `bakepkg.json`.

### Example `bakepkg.json`

```json
{
  "id": "com.example.mytool",
  "name": "MyTool",
  "version": "1.2.0",
  "output": "MyTool-1.2.0.pkg",
  "symlink_binaries": true,

  "files": {
    "bin":             ["./build/mytool"],
    "share/man/man1":  ["./docs/mytool.1"],
    "etc":             ["./config/defaults.yaml"]
  },

  "distribution": {
    "readme":           "./docs/README.txt",
    "license":          "./docs/LICENSE.txt",
    "welcome":          "./docs/WELCOME.txt",
    "background":       "./assets/bg.png",
    "background_dark":  "./assets/bg-dark.png"
  },

  "signing": {
    "identity":         "Developer ID Installer: Your Name (XYZ)",
    "notarize":         true,
    "issuer_id":        "...",
    "key_id":           "...",
    "private_key_b64":  "..."
  }
}
```

Files are always installed under `/Library/<Name>/<Version>/`. The `files` object maps destination subdirectories (relative to the install prefix) to lists of source paths. For example, `"bin": ["./build/mytool"]` installs to `/Library/MyTool/1.2.0/bin/mytool`.

### Configuration Reference

| Field | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `id` | `string` | *(Required)* | Reverse-DNS package identifier, e.g. `com.example.tool`. |
| `name` | `string` | *(Required)* | Application name. Used as the install prefix component and in `paths.d`. |
| `version` | `string` | `"1.0.0"` | Package version string. |
| `output` | `string` | `"out.pkg"` | Output `.pkg` filename. |
| `single_user` | `boolean` | `false` | Produce a distribution package with per-user installation domain choices. |
| `symlink_binaries` | `boolean` | `false` | Create symlinks in `/usr/local/bin/` for each detected binary. |
| `files` | `object` | | Map of `dest_subdir → [source_paths]`. Sources are copied preserving their basename. |
| `distribution` | `object` | | UI assets for the installer wizard (readme, license, welcome, background). |
| `signing.identity` | `string` | | Developer ID Installer certificate name or hash. |
| `signing.notarize` | `boolean` | `false` | Submit the package to Apple's Notary API after signing. |
| `signing.issuer_id` | `string` | | App Store Connect API issuer ID. Env: `MACOSNOTARYLIB_ISSUER_ID`. |
| `signing.key_id` | `string` | | App Store Connect API key ID. Env: `MACOSNOTARYLIB_KID`. |
| `signing.private_key_b64` | `string` | | Base64-encoded private key for notarization. Env: `MACOSNOTARYLIB_PRIVATE_KEY`. |

Notarization credentials can also be supplied via environment variables — the config fields take precedence if both are set.

## 📚 Library Usage

```go
import "al.essio.dev/cmd/bakepkg/pkg/pkginstaller"

builder := pkginstaller.New().
    WithIdentifier("com.example.mytool").
    WithName("MyTool").
    WithVersion("1.2.0").
    AddFile("build/mytool", "bin/mytool").
    WithSymlinkBinaries(true).
    WithDistributionUI(pkginstaller.Distribution{
        Readme:  "docs/README.md",
        License: "docs/LICENSE.txt",
    }).
    WithSigning(pkginstaller.Signing{
        Identity:      "Developer ID Installer: Example (XYZ)",
        Notarize:      true,
        IssuerID:      os.Getenv("MACOSNOTARYLIB_ISSUER_ID"),
        KeyID:         os.Getenv("MACOSNOTARYLIB_KID"),
        PrivateKeyB64: os.Getenv("MACOSNOTARYLIB_PRIVATE_KEY"),
    })

err := builder.Build("MyTool-1.2.0.pkg")
```

See the [pkg/pkginstaller](./pkg/pkginstaller/) package documentation and the [examples/](./examples) directory for more.

## 🔧 Requirements

The following native macOS tools must be available in `$PATH`:

- `pkgbuild`
- `productbuild`
- `productsign`
- `codesign`
- `stapler`

---

<div align="center">
  Made with ❤️ by <a href="https://github.com/alessio">Alessio Treglia</a>
</div>
