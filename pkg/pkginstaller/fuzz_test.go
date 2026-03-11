package pkginstaller

import (
	"os"
	"path/filepath"
	"testing"
)

func FuzzBuilder_Identifier(f *testing.F) {
	f.Add("com.example.app")
	f.Add("com.example.")
	f.Add("  ")
	f.Add("!!!invalid!!!")
	f.Add("")
	f.Add("a.b.c.d.e.f.g.h")

	f.Fuzz(func(t *testing.T, id string) {
		builder := New().WithIdentifier(id)

		if builder.opts.Identifier != id {
			t.Errorf("Expected identifier %q, got %q", id, builder.opts.Identifier)
		}
	})
}

func FuzzBuilder_Version(f *testing.F) {
	f.Add("1.0.0")
	f.Add("2.0.0-beta.1")
	f.Add("latest")
	f.Add("!!!")
	f.Add("")

	f.Fuzz(func(t *testing.T, version string) {
		builder := New().WithVersion(version)

		if builder.opts.Version != version {
			t.Errorf("Expected version %q, got %q", version, builder.opts.Version)
		}
	})
}

func FuzzBuild_InvalidPaths(f *testing.F) {
	f.Add("/tmp/nonexistent123")
	f.Add("  ")
	f.Add("*\x00*") // Null bytes
	f.Add("../../../../etc/passwd")

	f.Fuzz(func(t *testing.T, path string) {
		tmpDir, err := os.MkdirTemp("", "bakepkg-fuzz-*")
		if err != nil {
			t.Skipf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		outputPkg := filepath.Join(tmpDir, "out.pkg")

		opts := Options{
			Identifier:      "com.test.fuzz",
			Name:            "FuzzTool",
			Version:         "1.0.0",
			InstallLocation: InstallLocationLibrary,
			Files: map[string]string{
				path: "/Library/fuzz",
			},
			Infof: func(format string, args ...any) {},
		}

		// It should fail gracefully without panicking when given garbage paths
		err = build(opts, outputPkg)
		if err == nil {
			t.Logf("Unexpectedly succeeded with path: %q", path)
		}
	})
}
