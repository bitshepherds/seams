// Command seams finds a system's contract boundaries, and keeps a registry of
// each one's provider, consumers, contract and test world.
package main

import (
	"fmt"
	"io"
	"os"
)

// version is stamped in at build, by the release: -ldflags "-X main.version=…".
var version = "dev"

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) } // coverage-ignore: os.Exit ends the test run; the scenarios cover it

func run(args []string, out, errs io.Writer) int {
	if len(args) == 1 && args[0] == "--version" {
		_, _ = fmt.Fprintln(out, "seams", version)
		return 0
	}
	_, _ = fmt.Fprintln(errs, "usage: seams --version")
	return 2
}
