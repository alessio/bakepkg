package pkginstaller

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBuilder_Configuration(t *testing.T) {
	t.Parallel()

	builder := New().
		WithIdentifier("com.test.app").
		WithName("TestApp").
		WithVersion("1.5.0").
		AddFile("src.txt", "/Library/dst.txt").
		AddFile("src2.txt", "/Library/dst2.txt").
		WithScripts(Scripts{
			PreInstall:  "pre.sh",
			PostInstall: "post.sh",
			PreUpgrade:  "preupgrade.sh",
			PostUpgrade: "postupgrade.sh",
		}).
		WithDistributionUI(Distribution{
			Readme: "readme.md",
		}).
		WithSigning(Signing{
			Identity:     "DevID",
			Notarize:     true,
			Entitlements: []string{"ent1"},
		}).
		WithSingleUser(true).
		WithSymlinkBinaries(true).
		WithVerbose(true).
		WithDebug(true)

	opts := builder.opts

	if opts.Identifier != "com.test.app" {
		t.Errorf("Expected Identifier 'com.test.app', got '%s'", opts.Identifier)
	}
	if opts.Name != "TestApp" {
		t.Errorf("Expected Name 'TestApp', got '%s'", opts.Name)
	}
	if opts.Version != "1.5.0" {
		t.Errorf("Expected Version '1.5.0', got '%s'", opts.Version)
	}

	expectedFiles := map[string]string{
		"src.txt":  "/Library/dst.txt",
		"src2.txt": "/Library/dst2.txt",
	}
	if !reflect.DeepEqual(opts.Files, expectedFiles) {
		t.Errorf("Expected Files %v, got %v", expectedFiles, opts.Files)
	}

	if opts.Scripts.PreInstall != "pre.sh" {
		t.Errorf("Expected PreInstall 'pre.sh', got '%s'", opts.Scripts.PreInstall)
	}
	if opts.Scripts.PostInstall != "post.sh" {
		t.Errorf("Expected PostInstall 'post.sh', got '%s'", opts.Scripts.PostInstall)
	}
	if opts.Scripts.PreUpgrade != "preupgrade.sh" {
		t.Errorf("Expected PreUpgrade 'preupgrade.sh', got '%s'", opts.Scripts.PreUpgrade)
	}
	if opts.Scripts.PostUpgrade != "postupgrade.sh" {
		t.Errorf("Expected PostUpgrade 'postupgrade.sh', got '%s'", opts.Scripts.PostUpgrade)
	}

	if opts.DistributionUI.Readme != "readme.md" {
		t.Errorf("Expected Distribution Readme 'readme.md', got '%s'", opts.DistributionUI.Readme)
	}

	if opts.Signing.Identity != "DevID" {
		t.Errorf("Expected Signing Identity 'DevID', got '%s'", opts.Signing.Identity)
	}
	if !opts.Signing.Notarize {
		t.Errorf("Expected Notarize to be true")
	}
	if len(opts.Signing.Entitlements) != 1 || opts.Signing.Entitlements[0] != "ent1" {
		t.Errorf("Expected Entitlements ['ent1'], got %v", opts.Signing.Entitlements)
	}

	if !opts.SingleUser {
		t.Errorf("Expected SingleUser to be true")
	}
	if !opts.SymlinkBinaries {
		t.Errorf("Expected SymlinkBinaries to be true")
	}
	if !opts.Verbose {
		t.Errorf("Expected Verbose to be true")
	}
	if !opts.Debug {
		t.Errorf("Expected Debug to be true")
	}
}

func TestBuilder_Validate_RequiresName(t *testing.T) {
	t.Parallel()

	builder := New().WithIdentifier("com.test.app")
	// Name is empty
	if err := builder.Validate(); err == nil {
		t.Errorf("Expected validation to fail when Name is empty")
	}
}

func TestBuilder_Validate_RequiresIdentifier(t *testing.T) {
	t.Parallel()

	builder := New().WithName("TestApp")
	// Identifier is empty
	if err := builder.Validate(); err == nil {
		t.Errorf("Expected validation to fail when Identifier is empty")
	}
}

func TestBuilder_Validate_Success(t *testing.T) {
	t.Parallel()

	builder := New().WithIdentifier("com.test.app").WithName("TestApp")
	if err := builder.Validate(); err != nil {
		t.Errorf("Expected validation to succeed, got: %v", err)
	}
}

func TestFlatPackageBuilder(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "bakepkg-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	outputPkg := filepath.Join(tmpDir, "test.pkg")

	builder := New().
		WithIdentifier("com.test.dummy").
		WithName("DummyTool").
		WithVersion("1.0.0").
		AddFile("testdata/dummy_bin", "bin/dummy_bin").
		WithLogger(func(format string, args ...any) {
			t.Logf(format, args...)
		})

	err = builder.Build(outputPkg)
	if err != nil {
		t.Fatalf("Failed to build package: %v", err)
	}

	// Verify package was created
	if _, err := os.Stat(outputPkg); os.IsNotExist(err) {
		t.Fatalf("Output package does not exist: %s", outputPkg)
	}

	// Verify package contents using pkgutil --expand
	expandedDir := filepath.Join(tmpDir, "expanded")
	cmd := exec.Command("pkgutil", "--expand", outputPkg, expandedDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to expand package: %v", err)
	}

	// For a flat package, we expect a Payload file inside
	payloadPath := filepath.Join(expandedDir, "Payload")
	if _, err := os.Stat(payloadPath); os.IsNotExist(err) {
		t.Fatalf("Expanded package does not contain a Payload file")
	}
}

func TestDistributionPackageBuilder(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "bakepkg-test-dist-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	outputPkg := filepath.Join(tmpDir, "test-dist.pkg")

	builder := New().
		WithIdentifier("com.test.dummy.dist").
		WithName("DummyDist").
		WithVersion("2.0.0").
		AddFile("testdata/dummy_bin", "bin/dummy_bin").
		WithDistributionUI(Distribution{
			Readme: "testdata/readme.txt",
		}).
		WithLogger(func(format string, args ...any) {
			t.Logf(format, args...)
		})

	err = builder.Build(outputPkg)
	if err != nil {
		t.Fatalf("Failed to build distribution package: %v", err)
	}

	// Verify package was created
	if _, err := os.Stat(outputPkg); os.IsNotExist(err) {
		t.Fatalf("Output package does not exist: %s", outputPkg)
	}

	// Verify package contents
	expandedDir := filepath.Join(tmpDir, "expanded")
	cmd := exec.Command("pkgutil", "--expand", outputPkg, expandedDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to expand package: %v", err)
	}

	// A distribution package should have a Distribution XML file
	distXmlPath := filepath.Join(expandedDir, "Distribution")
	if _, err := os.Stat(distXmlPath); os.IsNotExist(err) {
		t.Fatalf("Expanded package does not contain a Distribution file")
	}

	// Read Distribution XML to verify Readme was injected
	content, err := os.ReadFile(distXmlPath)
	if err != nil {
		t.Fatalf("Failed to read Distribution XML: %v", err)
	}

	if !strings.Contains(string(content), `<readme file="readme.txt"/>`) {
		t.Errorf("Distribution XML does not contain the injected readme tag")
	}
}
