package main

import (
	"os"

	"openspec-atlas/internal"
)

// main delegates process execution to the reusable CLI entrypoint so command
// behavior can be exercised from tests as well as from the compiled binary.
func main() {
	os.Exit(internal.RunCLI(os.Args[1:], os.Stdout, os.Stderr))
}
