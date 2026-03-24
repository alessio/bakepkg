package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"al.essio.dev/cmd/bakepkg/internal/version"
	"al.essio.dev/cmd/bakepkg/pkg/pkginstaller"
)

// Config represents the schema for the macOS package configuration file.
type Config struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Version         string            `json:"version"`
	SingleUser      bool              `json:"single_user"`
	SymlinkBinaries bool              `json:"symlink_binaries"`
	Output          string            `json:"output"`
	Files           map[string][]string `json:"files"`
	Distribution    struct {
		Readme         string `json:"readme"`
		License        string `json:"license"`
		Welcome        string `json:"welcome"`
		Background     string `json:"background"`
		BackgroundDark string `json:"background_dark"`
	} `json:"distribution"`
	Signing struct {
		Identity      string   `json:"identity"`
		Entitlements  []string `json:"entitlements"`
		Notarize      bool     `json:"notarize"`
		IssuerID      string   `json:"issuer_id"`
		KeyID         string   `json:"key_id"`
		PrivateKeyB64 string   `json:"private_key_b64"`
	} `json:"signing"`
}

func printHelp() {
	fmt.Print(`usage: bakepkg [OPTION...]

Options:
  -config string  Path to JSON configuration file (default: "bakepkg.json")
  -verbose        Enable verbose output
  -debug          Enable debug output
  -help, -h       Show this help message

bakepkg combines the rich feature set of advanced macOS package builders with
integrated code-signing and API-level notarization capabilities. It reads
a JSON configuration file to determine how to build your package.

Configuration file format:
By default, bakepkg looks for "bakepkg.json" in the current directory.
Here is the complete schema:

  {
    "id": "com.example.tool",                  // REQUIRED: Package identifier
    "name": "MyTool",                          // REQUIRED: App name (used for install prefix)
    "version": "1.0.0",                        // Default: "1.0.0"
    "single_user": false,                      // Allow per-user installation
    "symlink_binaries": false,                 // Create /usr/local/bin symlinks
    "output": "MyTool.pkg",                    // Default: "out.pkg"

    "files": {                                 // Map of dest_subdir -> [source_paths]
      "bin": ["./build/cli-tool"],             // Installed to /Library/<Name>/<Version>/bin/cli-tool
      "etc": ["./config/defaults.yaml"],       // Detected as config for upgrade scripts
      "share/man/man1": ["./docs/cli-tool.1"]  // Detected as man page
    },

    "distribution": {                          // UI elements for the installer
      "readme": "./docs/README.txt",
      "license": "./docs/LICENSE.txt",
      "welcome": "./docs/WELCOME.txt",
      "background": "./assets/bg.png",
      "background_dark": "./assets/bg-dark.png"
    },

    "signing": {                               // Code signing & Notarization
      "identity": "Developer ID Installer: Name (XYZ)",
      "entitlements": ["com.apple.security.cs.allow-jit"],
      "notarize": true,

      // Notarization credentials (can also be set via ENV variables)
      "issuer_id": "...",                      // Env: MACOSNOTARYLIB_ISSUER_ID
      "key_id": "...",                         // Env: MACOSNOTARYLIB_KID
      "private_key_b64": "..."                 // Env: MACOSNOTARYLIB_PRIVATE_KEY
    }
  }

Install location is always /Library. Files are installed to /Library/<Name>/<Version>/.
Scripts (postinstall, uninstall) are auto-generated with paths.d integration.

Examples:
  1. Build using the default bakepkg.json file:
     $ bakepkg

  2. Build using a specific config file with verbose output:
     $ bakepkg -config custom.json -verbose

  3. CI/CD Pipeline (using environment variables for secrets):
     $ export MACOSNOTARYLIB_ISSUER_ID="xxx"
     $ export MACOSNOTARYLIB_KID="yyy"
     $ export MACOSNOTARYLIB_PRIVATE_KEY="base64_encoded_key"
     $ bakepkg
`)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("bakepkg: ")
	log.SetOutput(os.Stderr)

	var (
		configFile = flag.String("config", "bakepkg.json", "Path to JSON configuration file")
		verbose    = flag.Bool("verbose", false, "Enable verbose output")
		debug      = flag.Bool("debug", false, "Enable debug output")
		ver        = flag.Bool("version", false, "output version information and exit.")
		verShort   = flag.Bool("V", false, "output version information and exit (shorthand)")
	)

	// Override default usage to show our comprehensive help screen
	flag.Usage = printHelp
	flag.Parse()

	if *ver || *verShort {
		printVersion()
		os.Exit(0)
	}

	// If the user didn't provide a flag and the default bakepkg.json doesn't exist, error out.
	if _, err := os.Stat(*configFile); os.IsNotExist(err) {
		if *configFile == "bakepkg.json" {
			log.Println("default configuration file bakepkg.json not found. Use -config to specify a path.")
			printHelp()
		} else {
			log.Fatalf("configuration file %s not found", *configFile)
		}
		os.Exit(1)
	}

	cfg := Config{}
	data, err := os.ReadFile(*configFile)
	if err != nil {
		log.Fatalf("error reading config file: %v\n", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("error parsing config file: %v\n", err)
	}

	// Set defaults
	if cfg.Version == "" {
		cfg.Version = "1.0.0"
	}
	if cfg.Output == "" {
		cfg.Output = "out.pkg"
	}

	// Fallback to environment variables for signing/notarization if not in config
	if cfg.Signing.IssuerID == "" {
		cfg.Signing.IssuerID = os.Getenv("MACOSNOTARYLIB_ISSUER_ID")
	}
	if cfg.Signing.KeyID == "" {
		cfg.Signing.KeyID = os.Getenv("MACOSNOTARYLIB_KID")
	}
	if cfg.Signing.PrivateKeyB64 == "" {
		cfg.Signing.PrivateKeyB64 = os.Getenv("MACOSNOTARYLIB_PRIVATE_KEY")
	}

	if cfg.ID == "" {
		log.Fatal("package 'id' is required in the config file")
	}
	if cfg.Name == "" {
		log.Fatal("package 'name' is required in the config file")
	}

	builder := pkginstaller.New().
		WithIdentifier(cfg.ID).
		WithName(cfg.Name).
		WithVersion(cfg.Version).
		WithSingleUser(cfg.SingleUser).
		WithSymlinkBinaries(cfg.SymlinkBinaries).
		WithVerbose(*verbose).
		WithDebug(*debug).
		WithLogger(func(format string, args ...any) {
			if *verbose || *debug {
				fmt.Printf(format+"\n", args...)
			}
		})

	for destSubdir, sources := range cfg.Files {
		for _, src := range sources {
			// dest is <subdir>/<basename of source>
			dst := destSubdir + "/" + filepath.Base(src)
			builder.AddFile(src, dst)
		}
	}

	builder.WithDistributionUI(pkginstaller.Distribution{
		Readme:         cfg.Distribution.Readme,
		License:        cfg.Distribution.License,
		Welcome:        cfg.Distribution.Welcome,
		Background:     cfg.Distribution.Background,
		BackgroundDark: cfg.Distribution.BackgroundDark,
	})

	builder.WithSigning(pkginstaller.Signing{
		Identity:      cfg.Signing.Identity,
		Entitlements:  cfg.Signing.Entitlements,
		Notarize:      cfg.Signing.Notarize,
		IssuerID:      cfg.Signing.IssuerID,
		KeyID:         cfg.Signing.KeyID,
		PrivateKeyB64: cfg.Signing.PrivateKeyB64,
	})

	if *verbose || *debug {
		log.Printf("building package %s...\n", cfg.Output)
	}
	if err := builder.Build(cfg.Output); err != nil {
		log.Fatalf("error building package: %v\n", err)
	}

	if *verbose || *debug {
		log.Printf("package built successfully")
	}
}

func printVersion() {
	fmt.Printf("bakepkg %s\n", version.Version())
	fmt.Println("Copyright (C) 2026 Alessio Treglia <alessio@debian.org>")
}
