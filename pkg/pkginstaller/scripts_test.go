package pkginstaller

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectBinaries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		files    map[string]string
		expected []string
	}{
		{
			name:     "no binaries",
			files:    map[string]string{"file.txt": "/Library/MyTool/1.0.0/data/file.txt"},
			expected: nil,
		},
		{
			name:     "single binary",
			files:    map[string]string{"mytool": "/Library/MyTool/1.0.0/bin/mytool"},
			expected: []string{"mytool"},
		},
		{
			name: "multiple binaries",
			files: map[string]string{
				"tool1": "/Library/MyTool/1.0.0/bin/tool1",
				"tool2": "/Library/MyTool/1.0.0/bin/tool2",
			},
			expected: []string{"tool1", "tool2"},
		},
		{
			name:     "nested bin is not detected",
			files:    map[string]string{"file": "/Library/MyTool/1.0.0/bin/subdir/file"},
			expected: nil,
		},
		{
			name:     "bin directory itself is not a binary",
			files:    map[string]string{"file": "/Library/MyTool/1.0.0/bin/"},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectBinaries(tt.files)
			if tt.expected == nil && got != nil {
				t.Errorf("detectBinaries() = %v, want nil", got)
				return
			}
			if len(got) != len(tt.expected) {
				t.Errorf("detectBinaries() returned %d items, want %d: got %v", len(got), len(tt.expected), got)
				return
			}
			// Check all expected are present (order may vary due to map iteration)
			expectedSet := make(map[string]bool)
			for _, e := range tt.expected {
				expectedSet[e] = true
			}
			for _, g := range got {
				if !expectedSet[g] {
					t.Errorf("detectBinaries() returned unexpected %q", g)
				}
			}
		})
	}
}

func TestDetectManPages(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		files    map[string]string
		expected bool
	}{
		{
			name:     "no man pages",
			files:    map[string]string{"file": "/Library/MyTool/1.0.0/bin/mytool"},
			expected: false,
		},
		{
			name:     "has man pages",
			files:    map[string]string{"mytool.1": "/Library/MyTool/1.0.0/share/man/man1/mytool.1"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := detectManPages(tt.files); got != tt.expected {
				t.Errorf("detectManPages() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDetectConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		files    map[string]string
		expected bool
	}{
		{
			name:     "no config",
			files:    map[string]string{"file": "/Library/MyTool/1.0.0/bin/mytool"},
			expected: false,
		},
		{
			name:     "has config dir",
			files:    map[string]string{"cfg": "/Library/MyTool/1.0.0/etc/config.yaml"},
			expected: true,
		},
		{
			name:     "has etc suffix",
			files:    map[string]string{"cfg": "/Library/MyTool/1.0.0/etc"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := detectConfig(tt.files); got != tt.expected {
				t.Errorf("detectConfig() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGenerateScripts(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "bakepkg-scripts-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	scriptsDir := filepath.Join(tmpDir, "scripts")
	buildDir := filepath.Join(tmpDir, "build")

	opts := Options{
		Identifier: "com.example.tool",
		Name:       "MyTool",
		Version:    "1.0.0",
		Files: map[string]string{
			"mytool": "/Library/MyTool/1.0.0/bin/mytool",
		},
		Infof: func(format string, args ...any) {},
	}

	if err := generateScripts(opts, scriptsDir, buildDir); err != nil {
		t.Fatalf("generateScripts failed: %v", err)
	}

	// Verify postinstall exists and is executable
	postinstall := filepath.Join(scriptsDir, "postinstall")
	info, err := os.Stat(postinstall)
	if err != nil {
		t.Fatalf("postinstall not found: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Errorf("postinstall is not executable")
	}

	// Verify postinstall content
	data, err := os.ReadFile(postinstall)
	if err != nil {
		t.Fatalf("Failed to read postinstall: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "paths.d") {
		t.Errorf("postinstall missing paths.d reference")
	}
	if !strings.Contains(content, "MyTool") {
		t.Errorf("postinstall missing tool name")
	}
	if !strings.Contains(content, "/Library/MyTool/1.0.0") {
		t.Errorf("postinstall missing install prefix")
	}

	// Verify uninstall.sh exists in build payload
	uninstall := filepath.Join(buildDir, "MyTool", "1.0.0", "bin", "uninstall.sh")
	info, err = os.Stat(uninstall)
	if err != nil {
		t.Fatalf("uninstall.sh not found: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Errorf("uninstall.sh is not executable")
	}

	data, err = os.ReadFile(uninstall)
	if err != nil {
		t.Fatalf("Failed to read uninstall.sh: %v", err)
	}
	content = string(data)
	if !strings.Contains(content, "pkgutil --forget") {
		t.Errorf("uninstall.sh missing pkgutil --forget")
	}
	if !strings.Contains(content, "com.example.tool") {
		t.Errorf("uninstall.sh missing identifier")
	}
}

func TestGenerateScripts_WithSymlinks(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "bakepkg-scripts-symlink-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	scriptsDir := filepath.Join(tmpDir, "scripts")
	buildDir := filepath.Join(tmpDir, "build")

	opts := Options{
		Identifier:      "com.example.tool",
		Name:            "MyTool",
		Version:         "1.0.0",
		SymlinkBinaries: true,
		Files: map[string]string{
			"mytool": "/Library/MyTool/1.0.0/bin/mytool",
		},
		Infof: func(format string, args ...any) {},
	}

	if err := generateScripts(opts, scriptsDir, buildDir); err != nil {
		t.Fatalf("generateScripts failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(scriptsDir, "postinstall"))
	if err != nil {
		t.Fatalf("Failed to read postinstall: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "ln -sf") {
		t.Errorf("postinstall missing symlink command")
	}
	if !strings.Contains(content, "/usr/local/bin/mytool") {
		t.Errorf("postinstall missing /usr/local/bin/mytool symlink target")
	}
}

func TestGenerateUninstall(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "bakepkg-uninstall-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	scriptsDir := filepath.Join(tmpDir, "scripts")
	buildDir := filepath.Join(tmpDir, "build")

	opts := Options{
		Identifier:      "dev.example.cli",
		Name:            "MyCLI",
		Version:         "2.0.0",
		SymlinkBinaries: true,
		Files: map[string]string{
			"mycli":   "/Library/MyCLI/2.0.0/bin/mycli",
			"man.1":   "/Library/MyCLI/2.0.0/share/man/man1/mycli.1",
			"cfg.yml": "/Library/MyCLI/2.0.0/etc/config.yml",
		},
		Infof: func(format string, args ...any) {},
	}

	if err := generateScripts(opts, scriptsDir, buildDir); err != nil {
		t.Fatalf("generateScripts failed: %v", err)
	}

	uninstall := filepath.Join(buildDir, "MyCLI", "2.0.0", "bin", "uninstall.sh")
	data, err := os.ReadFile(uninstall)
	if err != nil {
		t.Fatalf("Failed to read uninstall.sh: %v", err)
	}
	content := string(data)

	checks := []string{
		`rm -f "/etc/paths.d/MyCLI"`,
		`rm -f "/etc/manpaths.d/MyCLI"`,
		`rm -f "/usr/local/bin/mycli"`,
		`rm -rf "/Library/MyCLI/2.0.0"`,
		`pkgutil --forget "dev.example.cli"`,
	}
	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("uninstall.sh missing: %s", check)
		}
	}

	// Verify upgrade scripts exist (because config is detected)
	if _, err := os.Stat(filepath.Join(scriptsDir, "preupgrade")); err != nil {
		t.Errorf("preupgrade script not generated despite config detection: %v", err)
	}
	if _, err := os.Stat(filepath.Join(scriptsDir, "postupgrade")); err != nil {
		t.Errorf("postupgrade script not generated despite config detection: %v", err)
	}

	// Verify preupgrade backs up config
	preupgrade, err := os.ReadFile(filepath.Join(scriptsDir, "preupgrade"))
	if err != nil {
		t.Fatalf("Failed to read preupgrade: %v", err)
	}
	if !strings.Contains(string(preupgrade), "/Library/MyCLI/2.0.0/etc") {
		t.Errorf("preupgrade missing config backup path")
	}
}
