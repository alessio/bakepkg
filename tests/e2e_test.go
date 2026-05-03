package tests

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"al.essio.dev/cmd/bakepkg/installer"
)

// Helper to build the bakepkg CLI binary for tests
func buildCLI(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "bakepkg")

	cmd := exec.Command("go", "build", "-o", binPath, "..")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build bakepkg CLI: %v\nOutput:\n%s", err, string(out))
	}
	return binPath
}

// Helper to expand a .pkg and list its payload files
func expandPkg(t *testing.T, pkgPath, outDir string) {
	t.Helper()
	cmd := exec.Command("pkgutil", "--expand", pkgPath, outDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("pkgutil --expand failed: %v\nOutput:\n%s", err, string(out))
	}
}

func listPayload(t *testing.T, payloadPath string) []string {
	t.Helper()
	cmd := exec.Command("cpio", "-it")
	f, err := os.Open(payloadPath)
	if err != nil {
		t.Fatalf("Failed to open Payload: %v", err)
	}
	defer f.Close()

	cmd.Stdin = f
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("cpio -it failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var res []string
	for _, l := range lines {
		if l != "" {
			res = append(res, l)
		}
	}
	return res
}

func TestE2E_SimpleFlatPackage(t *testing.T) {
	cli := buildCLI(t)
	tmpDir := t.TempDir()

	// Create dummy files
	srcBin := filepath.Join(tmpDir, "mytool")
	os.WriteFile(srcBin, []byte("#!/bin/sh\necho hello\n"), 0755)

	pkgOutput := filepath.Join(tmpDir, "out.pkg")

	// Create bakepkg.json
	cfg := map[string]interface{}{
		"id":      "com.test.simple",
		"name":    "SimpleApp",
		"version": "1.0.0",
		"output":  pkgOutput,
		"files": map[string][]string{
			"bin": {srcBin},
		},
	}

	cfgFile := filepath.Join(tmpDir, "bakepkg.json")
	b, _ := json.Marshal(cfg)
	os.WriteFile(cfgFile, b, 0644)

	// Run bakepkg
	cmd := exec.Command(cli, "-config", cfgFile, "-verbose")
	cmd.Dir = tmpDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bakepkg failed: %v\n%s", err, string(out))
	}

	if _, err := os.Stat(pkgOutput); err != nil {
		t.Fatalf("Expected package output not found: %v", err)
	}

	// Expand and verify
	expanded := filepath.Join(tmpDir, "expanded")
	expandPkg(t, pkgOutput, expanded)

	// This is a flat pkg, so Payload is at the root
	payload := filepath.Join(expanded, "Payload")
	files := listPayload(t, payload)

	foundBin := false
	for _, f := range files {
		if strings.Contains(f, "bin/mytool") {
			foundBin = true
			break
		}
	}
	if !foundBin {
		t.Errorf("mytool binary not found in Payload. Found: %v", files)
	}
}

func TestE2E_DistributionPackage(t *testing.T) {
	cli := buildCLI(t)
	tmpDir := t.TempDir()

	// Create dummy files
	srcBin := filepath.Join(tmpDir, "disttool")
	os.WriteFile(srcBin, []byte("#!/bin/sh\necho dist\n"), 0755)

	readme := filepath.Join(tmpDir, "README.txt")
	os.WriteFile(readme, []byte("Hello Readme"), 0644)

	pkgOutput := filepath.Join(tmpDir, "dist.pkg")

	// Create bakepkg.json
	cfg := map[string]interface{}{
		"id":          "com.test.dist",
		"name":        "DistApp",
		"version":     "2.0.0",
		"output":      pkgOutput,
		"single_user": true,
		"files": map[string][]string{
			"bin": {srcBin},
		},
		"distribution": map[string]string{
			"readme": readme,
		},
	}

	cfgFile := filepath.Join(tmpDir, "bakepkg.json")
	b, _ := json.Marshal(cfg)
	os.WriteFile(cfgFile, b, 0644)

	// Run bakepkg
	cmd := exec.Command(cli, "-config", cfgFile, "-verbose")
	cmd.Dir = tmpDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bakepkg failed: %v\n%s", err, string(out))
	}

	// Expand and verify
	expanded := filepath.Join(tmpDir, "expanded")
	expandPkg(t, pkgOutput, expanded)

	// This is a distribution pkg. Should have Distribution file and a component pkg
	distXML := filepath.Join(expanded, "Distribution")
	distContent, err := os.ReadFile(distXML)
	if err != nil {
		t.Fatalf("Distribution file not found in pkg: %v", err)
	}

	distStr := string(distContent)
	if !strings.Contains(distStr, `readme file="README.txt"`) {
		t.Errorf("Distribution missing readme tag")
	}
	if !strings.Contains(distStr, `enable_currentUserHome="true"`) {
		t.Errorf("Distribution missing single user domains injection")
	}

	// The component pkg is usually named after the identifier or something similar.
	// We'll search for the Payload file.
	var payloadPath string
	filepath.Walk(expanded, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && filepath.Base(path) == "Payload" {
			payloadPath = path
		}
		return nil
	})

	if payloadPath == "" {
		t.Fatalf("Payload file not found inside distribution pkg")
	}

	files := listPayload(t, payloadPath)
	foundBin := false
	for _, f := range files {
		if strings.Contains(f, "bin/disttool") {
			foundBin = true
			break
		}
	}
	if !foundBin {
		t.Errorf("disttool binary not found in Payload. Found: %v", files)
	}
}

func TestE2E_AutoScriptGeneration(t *testing.T) {
	cli := buildCLI(t)
	tmpDir := t.TempDir()

	// Dummy files
	srcBin := filepath.Join(tmpDir, "fulltool")
	os.WriteFile(srcBin, []byte("#!/bin/sh\necho dist\n"), 0755)

	srcMan := filepath.Join(tmpDir, "fulltool.1")
	os.WriteFile(srcMan, []byte("manpage"), 0644)

	srcConf := filepath.Join(tmpDir, "config.json")
	os.WriteFile(srcConf, []byte("{}"), 0644)

	pkgOutput := filepath.Join(tmpDir, "full.pkg")

	// Create bakepkg.json with bin, man, config
	cfg := map[string]interface{}{
		"id":               "com.test.full",
		"name":             "FullApp",
		"version":          "1.5.0",
		"output":           pkgOutput,
		"symlink_binaries": true,
		"files": map[string][]string{
			"bin":            {srcBin},
			"share/man/man1": {srcMan},
			"etc":            {srcConf},
		},
	}

	cfgFile := filepath.Join(tmpDir, "bakepkg.json")
	b, _ := json.Marshal(cfg)
	os.WriteFile(cfgFile, b, 0644)

	// Run bakepkg
	cmd := exec.Command(cli, "-config", cfgFile)
	cmd.Dir = tmpDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bakepkg failed: %v\n%s", err, string(out))
	}

	// Expand
	expanded := filepath.Join(tmpDir, "expanded")
	expandPkg(t, pkgOutput, expanded)

	// Check Scripts for postinstall, preupgrade, postupgrade
	scriptsDir := filepath.Join(expanded, "Scripts")
	for _, script := range []string{"postinstall", "preupgrade", "postupgrade"} {
		path := filepath.Join(scriptsDir, script)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("Expected script %s not found: %v", script, err)
			continue
		}

		content := string(data)
		// Basic check that symlink code exists
		if !strings.Contains(content, "ln -sf") && script != "preupgrade" {
			t.Errorf("%s missing symlink instruction", script)
		}
		// Basic check that config backup logic exists
		if !strings.Contains(content, "/tmp/bakepkg-upgrade-${NAME}") && script != "postinstall" {
			t.Errorf("%s missing config backup/restore logic", script)
		}
	}

	// Check Payload for uninstall.sh
	payloadPath := filepath.Join(expanded, "Payload")
	files := listPayload(t, payloadPath)
	foundUninstall := false
	for _, f := range files {
		if strings.Contains(f, "bin/uninstall.sh") {
			foundUninstall = true
			break
		}
	}
	if !foundUninstall {
		t.Errorf("uninstall.sh not found in Payload")
	}
}

func TestE2E_InvalidFileDestination(t *testing.T) {
	cli := buildCLI(t)
	tmpDir := t.TempDir()

	// Dummy files
	srcBin := filepath.Join(tmpDir, "badtool")
	os.WriteFile(srcBin, []byte("#!/bin/sh\necho dist\n"), 0755)

	pkgOutput := filepath.Join(tmpDir, "bad.pkg")

	// Create bakepkg.json with a destination that escapes the dir (e.g. ../../)
	// Actually, the main.go checks for safe relative subpaths, so it should fail early.
	cfg := map[string]interface{}{
		"id":      "com.test.bad",
		"name":    "BadApp",
		"version": "1.0.0",
		"output":  pkgOutput,
		"files": map[string][]string{
			"../../../etc": {srcBin},
		},
	}

	cfgFile := filepath.Join(tmpDir, "bakepkg.json")
	b, _ := json.Marshal(cfg)
	os.WriteFile(cfgFile, b, 0644)

	// Run bakepkg
	cmd := exec.Command(cli, "-config", cfgFile)
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatalf("Expected bakepkg to fail on invalid path, but it succeeded")
	}

	if !strings.Contains(string(out), "must be a safe relative path") {
		t.Errorf("Expected error about safe relative path, got: %s", string(out))
	}
}

func TestE2E_LibraryAPI(t *testing.T) {
	tmpDir := t.TempDir()

	// Dummy files
	srcBin := filepath.Join(tmpDir, "libtool")
	os.WriteFile(srcBin, []byte("#!/bin/sh\necho lib\n"), 0755)

	customPreinstall := filepath.Join(tmpDir, "preinstall")
	os.WriteFile(customPreinstall, []byte("#!/bin/sh\necho preinstall_custom\n"), 0755)

	pkgOutput := filepath.Join(tmpDir, "lib.pkg")

	builder := installer.New().
		WithIdentifier("com.test.lib").
		WithName("LibApp").
		WithVersion("3.0.0").
		AddFile(srcBin, "bin/libtool").
		WithScripts(installer.Scripts{
			PreInstall: customPreinstall,
		})

	if err := builder.Build(pkgOutput); err != nil {
		t.Fatalf("Builder API failed: %v", err)
	}

	// Expand and verify
	expanded := filepath.Join(tmpDir, "expanded")
	expandPkg(t, pkgOutput, expanded)

	// Verify custom preinstall is there
	preinstallPath := filepath.Join(expanded, "Scripts", "preinstall")
	data, err := os.ReadFile(preinstallPath)
	if err != nil {
		t.Fatalf("Custom preinstall script missing in Scripts: %v", err)
	}

	if !strings.Contains(string(data), "preinstall_custom") {
		t.Errorf("Preinstall content mismatch. Got: %s", string(data))
	}
}
