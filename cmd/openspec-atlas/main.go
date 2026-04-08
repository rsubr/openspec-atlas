package main

import (
	"os"

	"openspec-atlas/internal"
)

func main() {
	os.Exit(internal.RunCLI(os.Args[1:], os.Stdout, os.Stderr))
}
