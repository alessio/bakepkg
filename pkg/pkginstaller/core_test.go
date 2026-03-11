package pkginstaller

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuild_InvalidSourceFile(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "bakepkg-core-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	outputPkg := filepath.Join(tmpDir, "out.pkg")

	opts := Options{
		Identifier:      "com.test.invalid",
		Name:            "TestTool",
		Version:         "1.0.0",
		InstallLocation: InstallLocationLibrary,
		Files: map[string]string{
			"non_existent_file.txt": "/Library/non_existent.txt",
		},
		Infof: func(format string, args ...any) { t.Logf(format, args...) },
	}

	err = build(opts, outputPkg)
	if err == nil {
		t.Errorf("Expected build to fail due to missing source file, but it succeeded")
	}
}

func TestBuild_InvalidScript(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "bakepkg-core-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	outputPkg := filepath.Join(tmpDir, "out.pkg")

	opts := Options{
		Identifier:      "com.test.script",
		Name:            "TestTool",
		Version:         "1.0.0",
		InstallLocation: InstallLocationLibrary,
		Files:           make(map[string]string),
		Scripts: Scripts{
			PreInstall: "non_existent_script.sh",
		},
		Infof: func(format string, args ...any) { t.Logf(format, args...) },
	}

	err = build(opts, outputPkg)
	if err == nil {
		t.Errorf("Expected build to fail due to missing script, but it succeeded")
	}
}

func TestBuild_NoFilesNoScripts(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "bakepkg-core-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	outputPkg := filepath.Join(tmpDir, "out.pkg")

	opts := Options{
		Identifier:      "com.test.empty",
		Name:            "EmptyTool",
		Version:         "1.0.0",
		InstallLocation: InstallLocationLibrary,
		Files:           make(map[string]string),
		Infof:           func(format string, args ...any) { t.Logf(format, args...) },
	}

	// Auto-generated scripts will be present, so pkgbuild should succeed
	err = build(opts, outputPkg)
	if err != nil {
		t.Fatalf("Empty build failed unexpectedly: %v", err)
	}

	if _, err := os.Stat(outputPkg); os.IsNotExist(err) {
		t.Fatalf("Empty package was not created")
	}
}

func TestBuild_SingleUserForcesDist(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "bakepkg-core-test-singleuser-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	outputPkg := filepath.Join(tmpDir, "out.pkg")

	opts := Options{
		Identifier:      "com.test.singleuser",
		Name:            "SingleUserTool",
		Version:         "1.0.0",
		InstallLocation: InstallLocationLibrary,
		SingleUser:      true,
		Files: map[string]string{
			"testdata/dummy_bin": "/Library/SingleUserTool/1.0.0/bin/dummy_bin",
		},
		Infof: func(format string, args ...any) { t.Logf(format, args...) },
	}

	err = build(opts, outputPkg)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Verify it's a distribution package by expanding
	expandedDir := filepath.Join(tmpDir, "expanded")
	if err := os.MkdirAll(expandedDir, 0755); err != nil {
		t.Fatalf("Failed to create expanded dir: %v", err)
	}

	// Use pkgutil to expand
	if err := runCmd(Options{Infof: func(format string, args ...any) {}}, "pkgutil", "--expand", outputPkg, filepath.Join(expandedDir, "pkg")); err != nil {
		t.Fatalf("Failed to expand package: %v", err)
	}

	distXml := filepath.Join(expandedDir, "pkg", "Distribution")
	data, err := os.ReadFile(distXml)
	if err != nil {
		t.Fatalf("Failed to read Distribution XML (package should be a distribution pkg): %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "enable_currentUserHome") {
		t.Errorf("Distribution XML missing domains element for single-user mode")
	}
}

func TestInjectDomains(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "bakepkg-domains-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	distXml := filepath.Join(tmpDir, "Distribution.xml")
	initial := `<?xml version="1.0" encoding="utf-8"?>
<installer-gui-script minSpecVersion="1">
    <pkg-ref id="com.test"/>
</installer-gui-script>`
	if err := os.WriteFile(distXml, []byte(initial), 0644); err != nil {
		t.Fatalf("Failed to write test Distribution.xml: %v", err)
	}

	if err := injectDomains(distXml); err != nil {
		t.Fatalf("injectDomains failed: %v", err)
	}

	data, err := os.ReadFile(distXml)
	if err != nil {
		t.Fatalf("Failed to read Distribution.xml: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, `enable_anywhere="false"`) {
		t.Errorf("Missing enable_anywhere=false")
	}
	if !strings.Contains(content, `enable_currentUserHome="true"`) {
		t.Errorf("Missing enable_currentUserHome=true")
	}
	if !strings.Contains(content, `enable_localSystem="true"`) {
		t.Errorf("Missing enable_localSystem=true")
	}
}

func TestCopyFileOrDir(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "bakepkg-test-copy-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	src := filepath.Join(tmpDir, "src.txt")
	err = os.WriteFile(src, []byte("hello"), 0644)
	if err != nil {
		t.Fatalf("Failed to write src file: %v", err)
	}

	dst := filepath.Join(tmpDir, "dst.txt")
	opts := Options{Infof: func(format string, args ...any) { t.Logf(format, args...) }}
	err = copyFileOrDir(opts, src, dst)
	if err != nil {
		t.Fatalf("copyFileOrDir failed: %v", err)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("Failed to read dst file: %v", err)
	}

	if string(data) != "hello" {
		t.Errorf("dst file content = %q, want %q", string(data), "hello")
	}
}

func TestSetupScript(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "bakepkg-test-script-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	src := filepath.Join(tmpDir, "src.sh")
	err = os.WriteFile(src, []byte("#!/bin/sh\necho hello"), 0644)
	if err != nil {
		t.Fatalf("Failed to write src script: %v", err)
	}

	dst := filepath.Join(tmpDir, "scripts", "dst.sh")
	opts := Options{Infof: func(format string, args ...any) { t.Logf(format, args...) }}
	err = setupScript(opts, src, dst)
	if err != nil {
		t.Fatalf("setupScript failed: %v", err)
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("Failed to stat dst script: %v", err)
	}

	if info.Mode()&0111 == 0 {
		t.Errorf("dst script is not executable: %v", info.Mode())
	}
}
