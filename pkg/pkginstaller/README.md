# pkginstaller

`pkginstaller` is the core library behind `macpkg`. It provides a fluent Go API for programmatically building, signing, and notarizing macOS installer packages (`.pkg`).

## Features

- **Fluent Builder API:** Easily compose and configure your macOS packages in Go.
- **Distribution UI Support:** First-class support for Welcome screens, READMEs, Licenses, and Background images (including Dark Mode backgrounds).
- **Integrated Signing:** Automatically signs embedded executables and the final installer package.
- **Modern Notarization:** Directly integrates with Apple's Notary API using JWT credentials.
- **Automatic Sanitization:** Automatically strips Gatekeeper's `com.apple.quarantine` extended attributes.
- **Smart Component Mapping:** Generates `Component.plist` configurations for macOS App Bundles.
- **Simulation Mode:** Supports a "dry-run" mode that prints the commands to be executed without modifying the system.

## Usage

```go
import "al.essio.dev/pkg/pkginstaller"

builder := pkginstaller.New().
    WithIdentifier("com.example.app").
    WithVersion("1.0.0").
    WithInstallLocation(pkginstaller.InstallLocationApplications).
    AddFile("build/MyApp.app", "/Applications/MyApp.app").
    WithSimulate(true) // Enable simulation mode

err := builder.Build("output/MyApp.pkg")
```

## Requirements

The library utilizes native macOS tooling under the hood. The following commands must be available in your `$PATH`:
- `pkgbuild`
- `productbuild`
- `productsign`
- `codesign`
- `stapler`
- `xattr`
