package main

import (
	"os"

	"openspec-atlas/internals"
)

func main() {
	os.Exit(internals.RunCLI(os.Args[1:], os.Stdout, os.Stderr))
}
