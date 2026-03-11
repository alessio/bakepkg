package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestBuildSelf packages bakepkg itself using the project's bakepkg.json.
// It mirrors what the user does manually:
//
//	make build && ./bakepkg
//
// The test builds the binary, runs it against bakepkg.json, then expands
// the resulting .pkg and verifies the payload structure and scripts.
func TestBuildSelf(t *testing.T) {
	// Locate the module root (where bakepkg.json lives)
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("failed to resolve module root: %v", err)
	}

	tmpDir := t.TempDir()
	binary := filepath.Join(tmpDir, "bakepkg")
	outputPkg := filepath.Join(tmpDir, "bakepkg-0.1.0.pkg")

	// 1. Build the binary into a temp dir
	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	// 2. Run bakepkg with bakepkg.json — override output path
	//    We use a temporary copy of bakepkg.json so we can override "output".
	cfgData, err := os.ReadFile(filepath.Join(root, "bakepkg.json"))
	if err != nil {
		t.Fatalf("failed to read bakepkg.json: %v", err)
	}
	// Patch the output path to our temp dir
	cfgPatched := strings.Replace(string(cfgData),
		`"bakepkg-0.1.0.pkg"`, `"`+outputPkg+`"`, 1)
	cfgFile := filepath.Join(tmpDir, "bakepkg.json")
	if err := os.WriteFile(cfgFile, []byte(cfgPatched), 0644); err != nil {
		t.Fatalf("failed to write patched config: %v", err)
	}

	cmd = exec.Command(binary, "-config", cfgFile)
	cmd.Dir = root // source files (bakepkg binary, README.md, etc.) are relative to root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bakepkg run failed: %v\n%s", err, out)
	}

	if _, err := os.Stat(outputPkg); os.IsNotExist(err) {
		t.Fatalf("output package not created: %s", outputPkg)
	}

	// 3. Expand the package
	expandedDir := filepath.Join(tmpDir, "expanded")
	cmd = exec.Command("pkgutil", "--expand", outputPkg, expandedDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("pkgutil --expand failed: %v\n%s", err, out)
	}

	// 4. Enumerate the payload
	payload := filepath.Join(expandedDir, "Payload")
	cmd = exec.Command("cpio", "-it")
	cmd.Stdin, err = os.Open(payload)
	if err != nil {
		t.Fatalf("failed to open Payload: %v", err)
	}
	payloadOut, err := cmd.Output()
	if err != nil {
		t.Fatalf("cpio failed: %v", err)
	}
	payloadEntries := string(payloadOut)

	// 5. Verify payload contains the expected paths
	wantPaths := []string{
		"./bakepkg/0.1.0/bin/bakepkg",
		"./bakepkg/0.1.0/bin/uninstall.sh",
		"./bakepkg/0.1.0/share/README.md",
		"./bakepkg/0.1.0/share/LICENSE",
		"./bakepkg/0.1.0/share/examples",
		"./bakepkg/0.1.0/share/examples/clitool",
		"./bakepkg/0.1.0/share/examples/customui",
	}
	for _, want := range wantPaths {
		if !strings.Contains(payloadEntries, want) {
			t.Errorf("payload missing: %s", want)
		}
	}

	// 6. Verify postinstall script
	postinstall, err := os.ReadFile(filepath.Join(expandedDir, "Scripts", "postinstall"))
	if err != nil {
		t.Fatalf("failed to read postinstall: %v", err)
	}
	post := string(postinstall)

	postChecks := []struct {
		desc string
		want string
	}{
		{"name variable", `NAME="bakepkg"`},
		{"paths.d entry", `"/etc/paths.d/${NAME}"`},
		{"install prefix", `PREFIX="/Library/bakepkg/0.1.0"`},
		{"symlink creation", `ln -sf "/Library/bakepkg/0.1.0/bin/bakepkg" "/usr/local/bin/bakepkg"`},
	}
	for _, c := range postChecks {
		if !strings.Contains(post, c.want) {
			t.Errorf("postinstall missing %s: %q", c.desc, c.want)
		}
	}
}
