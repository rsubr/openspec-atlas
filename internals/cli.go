package internals

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
)

func RunCLI(args []string, stdout, stderr io.Writer) int {
	if err := run(args, stdout, stderr); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 1 && args[0] == "--version" {
		fmt.Fprintln(stdout, "openspec-atlas", Version)
		return nil
	}

	fs := flag.NewFlagSet("openspec-atlas", flag.ContinueOnError)
	fs.SetOutput(stderr)

	outputPath := fs.String("o", "structure.json", "output JSON file")
	allFiles := fs.Bool("all", false, "ignore .gitignore files and process everything")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: openspec-atlas [-o output.json] [-all] <dir> [dir ...]")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	dirs := fs.Args()
	if len(dirs) == 0 {
		fs.Usage()
		return fmt.Errorf("at least one directory is required")
	}

	output := scanProjects(dirs, *allFiles, stdout, stderr)

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal output: %w", err)
	}
	if err := os.WriteFile(*outputPath, data, 0644); err != nil {
		return fmt.Errorf("write error: %w", err)
	}

	fmt.Fprintln(stdout, *outputPath, "generated")
	return nil
}
