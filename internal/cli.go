package internal

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
)

// subcommands maps subcommand names to their handler functions. Adding a new
// subcommand is a one-line change here.
var subcommands = map[string]func([]string, io.Writer, io.Writer) error{
	"drift": runDrift,
}

type runOptions struct {
	outputPath string
	allFiles   bool
	version    bool
	dirs       []string
}

func RunCLI(args []string, stdout, stderr io.Writer) int {
	if err := run(args, stdout, stderr); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) > 0 {
		if fn, ok := subcommands[args[0]]; ok {
			return fn(args[1:], stdout, stderr)
		}
	}

	opts, err := parseRunOptions(args, stderr)
	if err != nil {
		return err
	}
	if opts.version {
		fmt.Fprintln(stdout, "openspec-atlas", Version)
		return nil
	}

	output := scanProjects(opts.dirs, opts.allFiles, stdout, stderr)
	return writeOutputFile(opts.outputPath, output, stdout)
}

func parseRunOptions(args []string, stderr io.Writer) (runOptions, error) {
	fs := flag.NewFlagSet("openspec-atlas", flag.ContinueOnError)
	fs.SetOutput(stderr)

	outputPath := fs.String("o", "structure.json", "output JSON file")
	allFiles := fs.Bool("all", false, "ignore .gitignore files and process everything")
	version := fs.Bool("version", false, "print version and exit")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: openspec-atlas [-o output.json] [-all] <dir> [dir ...]")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return runOptions{}, err
	}

	if *version {
		return runOptions{version: true}, nil
	}

	dirs := fs.Args()
	if len(dirs) == 0 {
		fs.Usage()
		return runOptions{}, fmt.Errorf("at least one directory is required")
	}

	return runOptions{
		outputPath: *outputPath,
		allFiles:   *allFiles,
		version:    *version,
		dirs:       dirs,
	}, nil
}

func writeOutputFile(path string, output Output, stdout io.Writer) error {
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal output: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write error: %w", err)
	}

	fmt.Fprintln(stdout, path, "generated")
	return nil
}
