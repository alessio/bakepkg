// Package bakepkg provides a fluent API for building macOS installer packages (.pkg).
// It wraps native macOS tooling like pkgbuild, productbuild, and productsign, while
// adding advanced features like Gatekeeper quarantine stripping and Apple Notary API integration.
//
// The library handles two main types of packages:
//  1. Flat Packages: A basic .pkg file that installs files to /Library/<Name>/<Version>/.
//  2. Distribution Packages: A more complex .pkg that can include a custom user interface
//     (Welcome, Readme, License), domain choices for per-user installation, and multiple
//     internal component packages.
//
// Script Generation:
//
// Install scripts (postinstall, uninstall.sh) are auto-generated from templates.
// The postinstall script writes /etc/paths.d and /etc/manpaths.d entries so binaries
// and man pages are discoverable. If SymlinkBinaries is set, symlinks are created in
// /usr/local/bin/. An uninstall.sh script is included in the payload for clean removal.
//
// When config files are detected (etc/ directory in payload), preupgrade and postupgrade
// scripts are also generated to back up and restore configuration during upgrades.
//
// Basic Usage (Flat Package):
//
//	builder := pkginstaller.New().
//	    WithIdentifier("com.example.tool").
//	    WithName("MyTool").
//	    WithVersion("1.0.0").
//	    AddFile("build/mytool", "bin/mytool")
//
//	err := builder.Build("MyTool.pkg")
//
// Advanced Usage (Distribution Package with UI and Signing):
//
//	builder := pkginstaller.New().
//	    WithIdentifier("com.example.tool").
//	    WithName("MyTool").
//	    WithVersion("1.0.0").
//	    AddFile("build/mytool", "bin/mytool").
//	    WithSingleUser(true).
//	    WithSymlinkBinaries(true).
//	    WithDistributionUI(pkginstaller.Distribution{
//	        Readme: "docs/README.md",
//	        License: "docs/LICENSE.txt",
//	    }).
//	    WithSigning(pkginstaller.Signing{
//	        Identity: "Developer ID Installer: Example (XYZ)",
//	        Notarize: true,
//	    })
//
//	err := builder.Build("MyTool-Signed.pkg")
//
// Quarantine Stripping:
//
// When files are downloaded or moved, macOS often attaches a "com.apple.quarantine"
// extended attribute. If these files are bundled into a package without stripping
// this attribute, the installed application may trigger "App is damaged" errors.
// This builder automatically runs 'xattr -d com.apple.quarantine' on all staged
// files to prevent this issue.
package pkginstaller
