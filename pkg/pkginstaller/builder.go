// Package bakepkg provides a fluent API for building macOS installer packages (.pkg).
// It wraps native macOS tooling like pkgbuild, productbuild, and productsign, while
// adding advanced features like Gatekeeper quarantine stripping and Apple Notary API integration.
package pkginstaller

import (
	"fmt"
	"strings"
)

// Builder orchestrates the creation of a macOS package.
// It uses a fluent interface for configuration.
type Builder struct {
	opts Options
}

// Options holds the configuration for the macOS package.
type Options struct {
	Identifier      string
	Name            string
	Version         string
	InstallLocation string // computed internally: always /Library
	Files           map[string]string // Source -> Destination
	Scripts         Scripts
	DistributionUI  Distribution
	Signing         Signing
	SingleUser      bool
	SymlinkBinaries bool
	Verbose         bool
	Debug           bool
	Simulate        bool
	Infof           func(format string, args ...any)
}

const (
	InstallLocationLibrary     = "/Library"
	InstallLocationUserLibrary = "~/Library"
)

// Scripts defines the paths to install scripts.
// Scripts must be executable and will be bundled into the package.
type Scripts struct {
	PreInstall  string
	PostInstall string
	PreUpgrade  string
	PostUpgrade string
}

// Distribution defines the UI elements for a Distribution package.
// Providing any of these will automatically convert a flat package into a Distribution package.
type Distribution struct {
	Readme         string
	License        string
	Welcome        string
	Background     string
	BackgroundDark string
}

// Signing holds the configuration for code-signing and notarizing the package.
type Signing struct {
	Identity      string
	Entitlements  []string
	Notarize      bool
	IssuerID      string
	KeyID         string
	PrivateKeyB64 string
}

// New creates a new Builder instance with default options.
func New() *Builder {
	return &Builder{
		opts: Options{
			Files: make(map[string]string),
			Version: "1.0.0",
			Infof:   func(format string, args ...any) {},
		},
	}
}

// WithIdentifier sets the package identifier (e.g., com.example.app).
func (b *Builder) WithIdentifier(id string) *Builder {
	b.opts.Identifier = id
	return b
}

// WithName sets the application name (required, used for install prefix and paths.d).
func (b *Builder) WithName(name string) *Builder {
	b.opts.Name = name
	return b
}

// WithVersion sets the package version (e.g., 1.0.0).
func (b *Builder) WithVersion(version string) *Builder {
	b.opts.Version = version
	return b
}

// AddFile adds a file mapping to the package.
// src is the local file path, and dst is the target path.
// If dst is relative (doesn't start with /), it is inferred based on Name, Version, and InstallLocation.
func (b *Builder) AddFile(src, dst string) *Builder {
	b.opts.Files[src] = dst
	return b
}

// WithScripts configures custom install scripts for the package.
// When set, these override the auto-generated scripts.
func (b *Builder) WithScripts(scripts Scripts) *Builder {
	b.opts.Scripts = scripts
	return b
}

// WithDistributionUI configures the Distribution XML UI elements (Welcome, Readme, License, Backgrounds).
func (b *Builder) WithDistributionUI(ui Distribution) *Builder {
	b.opts.DistributionUI = ui
	return b
}

// WithSigning configures the Developer ID signing identity and optional Notarization credentials.
func (b *Builder) WithSigning(signing Signing) *Builder {
	b.opts.Signing = signing
	return b
}

// WithSingleUser sets whether the package should support per-user installation.
// When true, a distribution package is generated with domain choices
// (enable_currentUserHome + enable_localSystem).
func (b *Builder) WithSingleUser(singleUser bool) *Builder {
	b.opts.SingleUser = singleUser
	return b
}

// WithSymlinkBinaries sets whether the postinstall script should create
// symlinks in /usr/local/bin/ for each detected binary.
func (b *Builder) WithSymlinkBinaries(symlink bool) *Builder {
	b.opts.SymlinkBinaries = symlink
	return b
}

// WithVerbose sets whether the builder should provide verbose output.
func (b *Builder) WithVerbose(verbose bool) *Builder {
	b.opts.Verbose = verbose
	return b
}

// WithDebug sets whether the builder should provide debug output (e.g., raw command output).
func (b *Builder) WithDebug(debug bool) *Builder {
	b.opts.Debug = debug
	return b
}

// WithSimulate sets whether the builder should simulate the build process without actually executing it.
func (b *Builder) WithSimulate(simulate bool) *Builder {
	b.opts.Simulate = simulate
	return b
}

// WithLogger sets the logging function for the builder.
func (b *Builder) WithLogger(infof func(format string, args ...any)) *Builder {
	if infof != nil {
		b.opts.Infof = infof
	}
	return b
}

// Build compiles the package according to the builder's configuration and writes it to the specified output path.
func (b *Builder) Build(output string) error {
	if err := b.Validate(); err != nil {
		return err
	}

	// Install location is always /Library
	b.opts.InstallLocation = InstallLocationLibrary

	// Compute install prefix: /Library/<Name>/<Version>/
	prefix := fmt.Sprintf("%s/%s/%s", InstallLocationLibrary, b.opts.Name, b.opts.Version)

	// Infer relative paths before building
	for src, dst := range b.opts.Files {
		if !strings.HasPrefix(dst, "/") {
			b.opts.Files[src] = fmt.Sprintf("%s/%s", prefix, dst)
		}
	}

	return build(b.opts, output)
}

// Validate checks if the builder configuration is valid.
func (b *Builder) Validate() error {
	if b.opts.Identifier == "" {
		return fmt.Errorf("package identifier is required")
	}
	if b.opts.Name == "" {
		return fmt.Errorf("package name is required")
	}
	return nil
}
