package pkginstaller

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bep/macosnotarylib"
	"github.com/golang-jwt/jwt/v4"
)

// build is the internal implementation that orchestrates the package creation process.
// It creates a staging environment, copies files, strips Gatekeeper attributes,
// signs binaries, and invokes pkgbuild/productbuild to generate the final package.
func build(opts Options, output string) error {
	// 1. Create staging directory
	staging, err := os.MkdirTemp("", "bakepkg-staging-*")
	if err != nil {
		return fmt.Errorf("failed to create staging dir: %w", err)
	}
	defer func(infof func(format string, args ...any)) {
		if err := os.RemoveAll(staging); err != nil {
			infof("failed to remove staging dir: %w", err)
		}
	}(opts.Infof)

	buildDir := filepath.Join(staging, "build")
	scriptsDir := filepath.Join(staging, "scripts")

	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return err
	}

	// 2. Copy files and remove quarantine
	for src, dst := range opts.Files {
		relDst := dst
		if strings.HasPrefix(dst, opts.InstallLocation) {
			relDst = strings.TrimPrefix(dst, opts.InstallLocation)
		}
		internalDst := filepath.Join(buildDir, strings.TrimPrefix(relDst, "/"))
		if err := os.MkdirAll(filepath.Dir(internalDst), 0755); err != nil {
			return err
		}
		if err := copyFileOrDir(opts, src, internalDst); err != nil {
			return fmt.Errorf("failed to copy %s: %w", src, err)
		}
		if err := stripQuarantine(opts, internalDst); err != nil {
			return fmt.Errorf("failed to strip quarantine from %s: %w", internalDst, err)
		}
	}

	// Set up scripts
	hasScripts := false
	hasExplicitScripts := opts.Scripts.PreInstall != "" || opts.Scripts.PostInstall != "" ||
		opts.Scripts.PreUpgrade != "" || opts.Scripts.PostUpgrade != ""

	if hasExplicitScripts {
		// User provided explicit script paths — copy them (backward compat for library users)
		if opts.Scripts.PreInstall != "" {
			if err := setupScript(opts, opts.Scripts.PreInstall, filepath.Join(scriptsDir, "preinstall")); err != nil {
				return err
			}
			hasScripts = true
		}
		if opts.Scripts.PostInstall != "" {
			if err := setupScript(opts, opts.Scripts.PostInstall, filepath.Join(scriptsDir, "postinstall")); err != nil {
				return err
			}
			hasScripts = true
		}
		if opts.Scripts.PreUpgrade != "" {
			if err := setupScript(opts, opts.Scripts.PreUpgrade, filepath.Join(scriptsDir, "preupgrade")); err != nil {
				return err
			}
			hasScripts = true
		}
		if opts.Scripts.PostUpgrade != "" {
			if err := setupScript(opts, opts.Scripts.PostUpgrade, filepath.Join(scriptsDir, "postupgrade")); err != nil {
				return err
			}
			hasScripts = true
		}
	} else {
		// Auto-generate scripts from templates
		if err := generateScripts(opts, scriptsDir, buildDir); err != nil {
			return fmt.Errorf("failed to generate scripts: %w", err)
		}
		hasScripts = true
	}

	// 3. Codesign binaries before packaging
	if opts.Signing.Identity != "" {
		if err := codesignStaging(opts, buildDir, opts.Signing); err != nil {
			return fmt.Errorf("codesign failed: %w", err)
		}
	}

	// 4. pkgbuild (flat package)
	flatPkg := filepath.Join(staging, "flat.pkg")
	pkgArgs := []string{
		"--root", buildDir,
		"--identifier", opts.Identifier,
		"--version", opts.Version,
		"--install-location", opts.InstallLocation,
	}
	if hasScripts {
		pkgArgs = append(pkgArgs, "--scripts", scriptsDir)
	}
	pkgArgs = append(pkgArgs, flatPkg)

	if err := runCmd(opts, "pkgbuild", pkgArgs...); err != nil {
		return fmt.Errorf("pkgbuild failed: %w", err)
	}

	// 5. productbuild (distribution package) if UI exists or single_user mode
	finalPkg := flatPkg // Default if no distribution needed

	hasDist := opts.SingleUser ||
		opts.DistributionUI.Readme != "" || opts.DistributionUI.License != "" ||
		opts.DistributionUI.Welcome != "" || opts.DistributionUI.Background != "" ||
		opts.DistributionUI.BackgroundDark != ""

	if hasDist {
		finalPkg = filepath.Join(staging, "dist.pkg")
		distXml := filepath.Join(staging, "Distribution.xml")
		resDir := filepath.Join(staging, "Resources")
		if err := os.MkdirAll(resDir, 0755); err != nil {
			return err
		}

		if opts.Simulate {
			// Mock distXml content so setupDistributionUI does not fail reading it
			_ = os.WriteFile(distXml, []byte("<installer-gui-script></installer-gui-script>"), 0644)
		}

		if err := runCmd(opts, "productbuild", "--synthesize", "--package", flatPkg, distXml); err != nil {
			return fmt.Errorf("productbuild synthesize failed: %w", err)
		}

		// Inject domain choices for single-user mode
		if opts.SingleUser {
			if err := injectDomains(distXml); err != nil {
				return fmt.Errorf("failed to inject domains: %w", err)
			}
		}

		// Setup Distribution UI elements
		if err := setupDistributionUI(opts, opts.DistributionUI, resDir, distXml); err != nil {
			return fmt.Errorf("failed to setup distribution UI: %w", err)
		}

		prodArgs := []string{
			"--distribution", distXml,
			"--resources", resDir,
			"--package-path", staging,
			finalPkg,
		}
		if err := runCmd(opts, "productbuild", prodArgs...); err != nil {
			return fmt.Errorf("productbuild failed: %w", err)
		}
	}

	// 6. productsign
	signedPkg := finalPkg
	if opts.Signing.Identity != "" {
		signedPkg = filepath.Join(staging, "signed.pkg")
		if err := runCmd(opts, "productsign", "--sign", opts.Signing.Identity, finalPkg, signedPkg); err != nil {
			return fmt.Errorf("productsign failed: %w", err)
		}
	}

	// 7. Notarization
	if opts.Signing.Notarize {
		if err := notarize(opts, signedPkg, opts.Signing); err != nil {
			return fmt.Errorf("notarize failed: %w", err)
		}
		if err := runCmd(opts, "stapler", "staple", signedPkg); err != nil {
			return fmt.Errorf("stapler failed: %w", err)
		}
	}

	// Finally, copy to output
	if err := copyFileOrDir(opts, signedPkg, output); err != nil {
		return fmt.Errorf("failed to copy final package: %w", err)
	}

	return nil
}

// injectDomains adds a <domains> element to the Distribution.xml file
// to enable per-user installation alongside system-wide installation.
func injectDomains(distXml string) error {
	data, err := os.ReadFile(distXml)
	if err != nil {
		return err
	}
	content := string(data)

	domainsTag := `    <domains enable_anywhere="false" enable_currentUserHome="true" enable_localSystem="true"/>` + "\n"
	content = strings.Replace(content, "</installer-gui-script>", domainsTag+"</installer-gui-script>", 1)

	return os.WriteFile(distXml, []byte(content), 0644)
}

// codesignStaging walks through the staging directory and recursively invokes
// the 'codesign' utility on executable binaries, applying entitlements if provided.
func codesignStaging(opts Options, buildDir string, signing Signing) error {
	var entitlementsFile string
	if len(signing.Entitlements) > 0 {
		f, err := os.CreateTemp("", "entitlements-*.plist")
		if err != nil {
			return err
		}
		defer func() {
			if err := os.Remove(f.Name()); err != nil {
				opts.Infof("failed to remove temporary entitlements file: %v", err)
			}
		}()
		entitlementsFile = f.Name()

		var content strings.Builder
		content.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
		content.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
		content.WriteString(`<plist version="1.0">` + "\n")
		content.WriteString(`<dict>` + "\n")
		for _, e := range signing.Entitlements {
			content.WriteString(fmt.Sprintf("    <key>%s</key><true/>\n", e))
		}
		content.WriteString(`</dict>` + "\n")
		content.WriteString(`</plist>` + "\n")

		if _, err := f.WriteString(content.String()); err != nil {
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
	}

	err := filepath.Walk(buildDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Try to sign only executables or bundles
		if !info.IsDir() && info.Mode()&0111 != 0 {
			args := []string{"--force", "--options", "runtime", "--sign", signing.Identity}
			if entitlementsFile != "" {
				args = append(args, "--entitlements", entitlementsFile)
			}
			args = append(args, path)

			if opts.Simulate {
				fmt.Printf("codesign %s\n", strings.Join(args, " "))
				return nil
			}

			// Ignore codesign errors for generic files that aren't mach-o
			cmd := exec.Command("codesign", args...)
			if err := cmd.Run(); err != nil {
				opts.Infof("codesign failed for %s: %v", path, err)
			}
		}
		return nil
	})
	return err
}

// setupDistributionUI injects Distribution XML elements (Background, License, Readme)
// into the synthesized Distribution file, copying required assets to the resources dir.
func setupDistributionUI(opts Options, ui Distribution, resDir string, distXml string) error {
	// 1. Copy UI files to resDir
	if ui.Readme != "" {
		if err := copyFileOrDir(opts, ui.Readme, filepath.Join(resDir, filepath.Base(ui.Readme))); err != nil {
			opts.Infof("failed to copy Readme file: %v", err)
		}
	}
	if ui.License != "" {
		if err := copyFileOrDir(opts, ui.License, filepath.Join(resDir, filepath.Base(ui.License))); err != nil {
			opts.Infof("failed to copy License file: %v", err)
		}
	}
	if ui.Welcome != "" {
		if err := copyFileOrDir(opts, ui.Welcome, filepath.Join(resDir, filepath.Base(ui.Welcome))); err != nil {
			opts.Infof("failed to copy Welcome file: %v", err)
		}
	}
	if ui.Background != "" {
		if err := copyFileOrDir(opts, ui.Background, filepath.Join(resDir, filepath.Base(ui.Background))); err != nil {
			opts.Infof("failed to copy Background image: %v", err)
		}
	}
	if ui.BackgroundDark != "" {
		if err := copyFileOrDir(opts, ui.BackgroundDark, filepath.Join(resDir, filepath.Base(ui.BackgroundDark))); err != nil {
			opts.Infof("failed to copy Dark Background image: %v", err)
		}
	}

	// 2. Read existing Distribution.xml
	data, err := os.ReadFile(distXml)
	if err != nil {
		return err
	}
	content := string(data)

	// 3. Inject UI elements into Distribution.xml before the </installer-gui-script> tag
	var injection strings.Builder
	if ui.Readme != "" {
		injection.WriteString(fmt.Sprintf("    <readme file=\"%s\"/>\n", filepath.Base(ui.Readme)))
	}
	if ui.License != "" {
		injection.WriteString(fmt.Sprintf("    <license file=\"%s\"/>\n", filepath.Base(ui.License)))
	}
	if ui.Welcome != "" {
		injection.WriteString(fmt.Sprintf("    <welcome file=\"%s\"/>\n", filepath.Base(ui.Welcome)))
	}
	if ui.Background != "" || ui.BackgroundDark != "" {
		bgLine := `    <background`
		if ui.Background != "" {
			bgLine += fmt.Sprintf(` file="%s"`, filepath.Base(ui.Background))
		}
		if ui.BackgroundDark != "" {
			bgLine += fmt.Sprintf(` file-dark="%s"`, filepath.Base(ui.BackgroundDark))
		}
		bgLine += ` alignment="bottomleft"/>` + "\n"
		injection.WriteString(bgLine)
	}

	if injection.Len() > 0 {
		content = strings.Replace(content, "</installer-gui-script>", injection.String()+"</installer-gui-script>", 1)
		return os.WriteFile(distXml, []byte(content), 0644)
	}

	return nil
}

// notarize interacts with Apple's Notary API using JWT credentials to submit
// the built package for automated malware scanning and approval.
func notarize(opts Options, filename string, signing Signing) error {
	if signing.IssuerID == "" || signing.KeyID == "" || signing.PrivateKeyB64 == "" {
		return fmt.Errorf("IssuerID, KeyID, and PrivateKeyB64 must be set for notarization")
	}

	if opts.Simulate {
		fmt.Printf("notarize %s\n", filename)
		return nil
	}

	if err := os.Setenv("MACOSNOTARYLIB_PRIVATE_KEY", signing.PrivateKeyB64); err != nil {
		return err
	}

	n, err := macosnotarylib.New(
		macosnotarylib.Options{
			IssuerID:          signing.IssuerID,
			Kid:               signing.KeyID,
			SubmissionTimeout: 15 * time.Minute,
			SignFunc: func(token *jwt.Token) (string, error) {
				key, err := macosnotarylib.LoadPrivateKeyFromEnvBase64("MACOSNOTARYLIB_PRIVATE_KEY")
				if err != nil {
					return "", err
				}
				return token.SignedString(key)
			},
		},
	)
	if err != nil {
		return err
	}

	return n.Submit(filename)
}

// copyFileOrDir duplicates files or entire folder hierarchies using native macOS commands.
func copyFileOrDir(opts Options, src, dst string) error {
	if opts.Simulate {
		fmt.Printf("cp -R %s %s\n", src, dst)
		return nil
	}
	cmd := exec.Command("cp", "-R", src, dst)
	if err := cmd.Run(); err != nil {
		opts.Infof("failed to copy %s to %s: %v", src, dst, err)
		return err
	}
	return nil
}

// stripQuarantine removes Gatekeeper's "com.apple.quarantine" extended attribute
// to prevent "App is damaged and can't be opened" errors upon installation.
func stripQuarantine(opts Options, path string) error {
	if opts.Simulate {
		fmt.Printf("xattr -r -d com.apple.quarantine %s\n", path)
		return nil
	}
	cmd := exec.Command("xattr", "-r", "-d", "com.apple.quarantine", path)
	if err := cmd.Run(); err != nil {
		opts.Infof("failed to strip quarantine: %v", err)
	} // Ignore errors, as it might not have the attribute
	return nil
}

// setupScript stages install scripts by copying them
// to the script directory, setting executable permissions, and unquarantining them.
func setupScript(opts Options, src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	if err := copyFileOrDir(opts, src, dst); err != nil {
		return err
	}
	if !opts.Simulate {
		if err := os.Chmod(dst, 0755); err != nil {
			return err
		}
	}
	return stripQuarantine(opts, dst)
}

// runCmd executes an arbitrary shell command directly attached to standard out/err.
func runCmd(opts Options, name string, args ...string) error {
	if opts.Simulate {
		fmt.Printf("%s %s\n", name, strings.Join(args, " "))
		return nil
	}
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
