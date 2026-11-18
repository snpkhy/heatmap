package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantOut  string
		wantErr  string
	}{
		{name: "no args", args: nil, wantCode: 2, wantErr: "Usage"},
		{name: "version", args: []string{"version"}, wantCode: 0, wantOut: "heatmap dev"},
		{name: "help", args: []string{"help"}, wantCode: 0, wantOut: "Commands:"},
		{name: "unknown command", args: []string{"foo"}, wantCode: 2, wantErr: `unknown command "foo"`},
		{name: "not implemented", args: []string{"serve"}, wantCode: 1, wantErr: "not implemented"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := run(tt.args, &out, &errOut)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d", code, tt.wantCode)
			}
			if !strings.Contains(out.String(), tt.wantOut) {
				t.Errorf("stdout = %q, want it to contain %q", out.String(), tt.wantOut)
			}
			if !strings.Contains(errOut.String(), tt.wantErr) {
				t.Errorf("stderr = %q, want it to contain %q", errOut.String(), tt.wantErr)
			}
		})
	}
}

const testPipelines = "../../configs/pipelines"

func runCLI(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestList(t *testing.T) {
	code, out, errOut := runCLI(t, "list", "-pipelines", testPipelines, "-data", t.TempDir())
	if code != 0 {
		t.Fatalf("code %d: %s", code, errOut)
	}
	for _, want := range []string{"METHODS", "[filtering]", "gaussian", "k=5", "PIPELINES", "p8", "SEQUENCES", "synthetic/street"} {
		if !strings.Contains(out, want) {
			t.Errorf("list output lacks %q", want)
		}
	}
	code, out, _ = runCLI(t, "list", "-json", "-pipelines", testPipelines, "-data", t.TempDir())
	if code != 0 || !strings.Contains(out, `"methods"`) || !strings.Contains(out, `"params"`) {
		t.Errorf("json list: code %d", code)
	}
	if code, _, _ := runCLI(t, "list", "-pipelines", "missing-dir"); code != 1 {
		t.Errorf("missing pipelines dir: code %d", code)
	}
}

func TestRunCommand(t *testing.T) {
	out := t.TempDir()
	code, stdout, stderr := runCLI(t, "run", "-pipeline", "p1", "-input", "synthetic/night", "-pipelines", testPipelines,
		"-out", out, "-frames", "25", "-noise", "gaussian:5", "-save-frames")
	if code != 0 {
		t.Fatalf("code %d: %s", code, stderr)
	}
	for _, want := range []string{"pipeline  p1", "F1 0.", "fps", "output"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("summary lacks %q:\n%s", want, stdout)
		}
	}
	entries, err := os.ReadDir(out)
	if err != nil || len(entries) != 1 {
		t.Fatalf("want one run directory, got %v %v", entries, err)
	}
	if _, err := os.Stat(filepath.Join(out, entries[0].Name(), "run.json")); err != nil {
		t.Error("run.json missing")
	}

	code, stdout, _ = runCLI(t, "run", "-pipeline", "p6", "-input", "synthetic/street", "-pipelines", testPipelines,
		"-out", out, "-start", "15", "-frames", "30")
	if code != 0 || !strings.Contains(stdout, "direction dominant") {
		t.Errorf("p6 run: code %d\n%s", code, stdout)
	}
}

func TestRunCommandErrors(t *testing.T) {
	for _, args := range [][]string{
		{"run", "-pipeline", "p99", "-pipelines", testPipelines},
		{"run", "-pipelines", testPipelines, "-input", "synthetic/ocean"},
		{"run", "-pipelines", testPipelines, "-noise", "fog:1"},
		{"run", "-pipelines", testPipelines, "-start", "9999", "-out", t.TempDir()},
	} {
		if code, _, _ := runCLI(t, args...); code != 1 {
			t.Errorf("%v: code %d, want 1", args, code)
		}
	}
	if code, _, _ := runCLI(t, "run", "-bogus"); code != 2 {
		t.Error("unknown flag must give code 2")
	}
	if code, _, _ := runCLI(t, "run", "extra"); code != 2 {
		t.Error("positional argument must give code 2")
	}
	if code, _, _ := runCLI(t, "run", "-h"); code != 0 {
		t.Error("-h must give code 0")
	}
}

func TestExperimentTestcasesBenchCommands(t *testing.T) {
	dir := t.TempDir()
	exp := filepath.Join(dir, "e.yaml")
	if err := os.WriteFile(exp, []byte("name: e\npipelines: [p1, p8]\nsequences: [synthetic/night]\nframes: 15\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runCLI(t, "experiment", "-config", exp, "-pipelines", testPipelines, "-out", dir, "-workers", "2")
	if code != 0 || !strings.Contains(stdout, "2 runs in") || !strings.Contains(stderr, "[2/2]") {
		t.Errorf("experiment: code %d\n%s\n%s", code, stdout, stderr)
	}

	tc := filepath.Join(dir, "tc.yaml")
	if err := os.WriteFile(tc, []byte(`cases:
  - {id: A, pipeline: p1, sequence: synthetic/street, frames: 30, expect: {min_f1: 0.5}}
  - {id: B, pipeline: p1, sequence: synthetic/street, frames: 30, expect: {min_f1: 0.999}}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ = runCLI(t, "testcases", "-config", tc, "-pipelines", testPipelines)
	if code != 1 || !strings.Contains(stdout, "1/2 test cases passed") || !strings.Contains(stdout, "FAIL") {
		t.Errorf("testcases: code %d\n%s", code, stdout)
	}
	code, stdout, _ = runCLI(t, "testcases", "-config", tc, "-pipelines", testPipelines, "-json")
	if code != 1 || !strings.Contains(stdout, `"passed": false`) {
		t.Errorf("testcases -json: code %d", code)
	}

	code, stdout, _ = runCLI(t, "bench", "-pipelines", testPipelines, "-frames", "5", "-input", "synthetic/night", "-workers", "2")
	if code != 0 || !strings.Contains(stdout, "speedup") || !strings.Contains(stdout, "p8") {
		t.Errorf("bench: code %d\n%s", code, stdout)
	}

	for _, args := range [][]string{
		{"experiment", "-config", filepath.Join(dir, "missing.yaml")},
		{"testcases", "-config", filepath.Join(dir, "missing.yaml")},
		{"bench", "-pipelines", testPipelines, "-input", "synthetic/ocean"},
	} {
		if code, _, _ := runCLI(t, args...); code != 1 {
			t.Errorf("%v: code %d, want 1", args, code)
		}
	}
}
