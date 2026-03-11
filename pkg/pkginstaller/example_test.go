package pkginstaller_test

import (
	"fmt"
	"os"
	"al.essio.dev/cmd/bakepkg/pkg/pkginstaller"
)

func ExampleBuilder_Build_flat() {
	// Create dummy files to bundle for this example
	_ = os.MkdirAll("testdata", 0755)
	_ = os.WriteFile("testdata/hello", []byte("hello world"), 0755)
	defer os.RemoveAll("testdata")

	// Configure the builder — no explicit install location or scripts needed
	builder := pkginstaller.New().
		WithIdentifier("com.example.hello").
		WithName("Hello").
		WithVersion("1.0.0").
		AddFile("testdata/hello", "bin/hello").
		WithSimulate(true).
		WithLogger(func(format string, args ...any) {
			fmt.Printf(format+"\n", args...)
		})

	// Build the package — install location and scripts are auto-generated
	if err := builder.Build("hello.pkg"); err != nil {
		fmt.Printf("Build failed: %v\n", err)
	}
}

func ExampleBuilder_Build_distribution() {
	// Create dummy files and resources for this example
	_ = os.MkdirAll("testdata/resources", 0755)
	_ = os.WriteFile("testdata/app", []byte("binary data"), 0755)
	_ = os.WriteFile("testdata/resources/readme.txt", []byte("readme content"), 0644)
	defer os.RemoveAll("testdata")

	// Configure the builder
	builder := pkginstaller.New().
		WithIdentifier("com.example.app").
		WithName("MyApp").
		WithVersion("2.5.0").
		AddFile("testdata/app", "bin/myapp").
		WithDistributionUI(pkginstaller.Distribution{
			Readme: "testdata/resources/readme.txt",
		}).
		WithSimulate(true).
		WithLogger(func(format string, args ...any) {
			fmt.Printf(format+"\n", args...)
		})

	// Build the package
	if err := builder.Build("awesome.pkg"); err != nil {
		fmt.Printf("Build failed: %v\n", err)
	}
}
