// Command heatmap is the CLI of the motion heatmap project.
//
// Subcommands (implemented stage by stage, see README):
//
//	heatmap list        — list methods and pipelines
//	heatmap run         — run a pipeline or a single method
//	heatmap experiment  — pipelines × sequences × noise levels
//	heatmap testcases   — check reference test cases
//	heatmap bench       — measure time/FPS
//	heatmap version     — print the program version
package main

import (
	"fmt"
	"io"
	"os"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run dispatches the subcommand and returns the process exit code.
// It is separate from main so the CLI can be tested.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}

	switch args[0] {
	case "version":
		_, _ = fmt.Fprintln(stdout, "heatmap", version)
		return 0
	case "help", "-h", "--help":
		usage(stdout)
		return 0
	case "list", "run", "experiment", "testcases", "bench":
		_, _ = fmt.Fprintf(stderr, "command %q is not implemented yet\n", args[0])
		return 1
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command %q\n\n", args[0])
		usage(stderr)
		return 2
	}
}

// usage prints help for the subcommands.
func usage(w io.Writer) {
	_, _ = fmt.Fprint(w, `Usage: heatmap <command> [flags]

Commands:
  list         list methods (with categories) and pipelines
  run          run a pipeline (--pipeline) or a single method (--method)
  experiment   run a full experiment from a config
  testcases    check reference test cases
  bench        measure time/FPS, sequential vs parallel
  version      print the program version
`)
}
