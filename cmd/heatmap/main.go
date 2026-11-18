// Command heatmap is the CLI of the motion heatmap project.
//
//	heatmap list        — list methods, pipelines and sequences
//	heatmap run         — run a pipeline on a sequence
//	heatmap experiment  — pipelines × sequences × noise levels
//	heatmap testcases   — check reference test cases
//	heatmap bench       — measure time/FPS, sequential vs parallel
//	heatmap serve       — web UI and HTTP API
//	heatmap version     — print the program version
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// command is a subcommand: it parses its own flags from args.
type command func(args []string, stdout, stderr io.Writer) int

// commands is filled by the files that implement them.
var commands = map[string]command{}

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
	}
	if cmd, ok := commands[args[0]]; ok {
		return cmd(args[1:], stdout, stderr)
	}
	switch args[0] {
	case "experiment", "testcases", "bench", "serve":
		_, _ = fmt.Fprintf(stderr, "command %q is not implemented yet\n", args[0])
		return 1
	}
	_, _ = fmt.Fprintf(stderr, "unknown command %q\n\n", args[0])
	usage(stderr)
	return 2
}

// usage prints help for the subcommands.
func usage(w io.Writer) {
	_, _ = fmt.Fprint(w, `Usage: heatmap <command> [flags]

Commands:
  list         list methods (with categories and parameters), pipelines and sequences
  run          run a pipeline on a sequence (CDnet, a folder of frames or a synthetic scene)
  experiment   run a full experiment from a config
  testcases    check reference test cases
  bench        measure time/FPS, sequential vs parallel
  serve        start the web UI and HTTP API
  version      print the program version

Run "heatmap <command> -h" for the flags of a command.
`)
}

// newFlags returns a FlagSet that prints errors and help to stderr.
func newFlags(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

// parse parses flags and returns an exit code ≥ 0 when the command should stop.
func parse(fs *flag.FlagSet, args []string) (code int, stop bool) {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0, true
		}
		return 2, true
	}
	if fs.NArg() > 0 {
		_, _ = fmt.Fprintf(fs.Output(), "unexpected arguments: %v\n", fs.Args())
		return 2, true
	}
	return 0, false
}

func fail(stderr io.Writer, err error) int {
	_, _ = fmt.Fprintln(stderr, "error:", err)
	return 1
}
