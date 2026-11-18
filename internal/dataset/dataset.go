package dataset

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"heatmap/internal/imgx"
)

// Sources of sequences, also the first component of a sequence ID.
const (
	SourceCDnet     = "cdnet"
	SourceFrames    = "frames"
	SourceSynthetic = "synthetic"
)

// ErrNotFound is returned when a sequence ID does not name an existing sequence.
var ErrNotFound = errors.New("dataset: sequence not found")

// Info describes a sequence without loading its frames.
type Info struct {
	// ID identifies the sequence for Open: "synthetic/street",
	// "cdnet/baseline/highway", "frames/my_video", or a filesystem path.
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Source   string `json:"source"`
	Path     string `json:"path,omitempty"`
	Frames   int    `json:"frames"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	// HasGT tells whether ground truth exists for frames EvalStart…EvalEnd
	// (0-based, inclusive). Without ground truth both are −1.
	HasGT     bool `json:"has_gt"`
	EvalStart int  `json:"eval_start"`
	EvalEnd   int  `json:"eval_end"`
}

// Sequence is an ordered set of color frames with optional ground truth.
// Implementations load frames lazily and are safe for concurrent reads.
type Sequence interface {
	Info() Info
	// Frame returns frame i (0-based) as an RGB image.
	Frame(i int) (*imgx.RGB, error)
	// GroundTruth returns the CDnet-style label image of frame i
	// (0 static, 50 shadow, 85 outside ROI, 170 unknown, 255 motion),
	// or nil without an error when frame i has no ground truth.
	GroundTruth(i int) (*imgx.Gray, error)
}

// Open opens a sequence by ID. IDs starting with "synthetic/" name built-in
// scenes; "cdnet/<category>/<sequence>" and "frames/<name>" are looked up in
// dataRoot/cdnet and dataRoot/frames. Anything else is treated as a path to a
// CDnet sequence directory (one with input/) or to a folder of images.
//
// IDs may not contain ".." components, so an ID received from the web UI
// cannot reach files outside dataRoot.
func Open(dataRoot, id string) (Sequence, error) {
	if id == "" {
		return nil, fmt.Errorf("%w: empty id", ErrNotFound)
	}
	parts := strings.Split(filepath.ToSlash(id), "/")
	for _, p := range parts {
		if p == ".." {
			return nil, fmt.Errorf("dataset: id %q must not contain a parent-directory component", id)
		}
	}
	switch parts[0] {
	case SourceSynthetic:
		if len(parts) != 2 {
			return nil, fmt.Errorf("%w: %q", ErrNotFound, id)
		}
		return NewSynthetic(parts[1])
	case SourceCDnet:
		if len(parts) != 3 {
			return nil, fmt.Errorf("%w: %q (want cdnet/<category>/<sequence>)", ErrNotFound, id)
		}
		return OpenCDnet(filepath.Join(dataRoot, SourceCDnet, parts[1], parts[2]), id)
	case SourceFrames:
		if len(parts) != 2 {
			return nil, fmt.Errorf("%w: %q (want frames/<name>)", ErrNotFound, id)
		}
		return OpenFolder(filepath.Join(dataRoot, SourceFrames, parts[1]), id)
	}
	return OpenPath(id)
}

// OpenPath opens a directory as a CDnet sequence if it has an input/
// subdirectory, and as a plain folder of images otherwise.
func OpenPath(path string) (Sequence, error) {
	st, err := os.Stat(path)
	if err != nil || !st.IsDir() {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, path)
	}
	if isDir(filepath.Join(path, "input")) {
		return OpenCDnet(path, path)
	}
	return OpenFolder(path, path)
}

// Discover lists the built-in synthetic scenes and every sequence found under
// dataRoot/cdnet/<category>/<sequence> and dataRoot/frames/<name>. A missing
// dataRoot simply yields only the synthetic scenes.
func Discover(dataRoot string) ([]Info, error) {
	var out []Info
	for _, name := range SyntheticNames() {
		s, err := NewSynthetic(name)
		if err != nil {
			return nil, err
		}
		out = append(out, s.Info())
	}

	cdnet := filepath.Join(dataRoot, SourceCDnet)
	for _, cat := range subdirs(cdnet) {
		for _, seq := range subdirs(filepath.Join(cdnet, cat)) {
			s, err := OpenCDnet(filepath.Join(cdnet, cat, seq), SourceCDnet+"/"+cat+"/"+seq)
			if err != nil {
				continue // not a sequence (e.g. a stray folder); skip it
			}
			out = append(out, s.Info())
		}
	}
	frames := filepath.Join(dataRoot, SourceFrames)
	for _, name := range subdirs(frames) {
		s, err := OpenFolder(filepath.Join(frames, name), SourceFrames+"/"+name)
		if err != nil {
			continue
		}
		out = append(out, s.Info())
	}
	return out, nil
}

func isDir(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

// subdirs returns the sorted names of the directories inside dir.
func subdirs(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// imageFiles returns the sorted names of image files (jpg, jpeg, png, bmp) in dir.
func imageFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		switch strings.ToLower(filepath.Ext(e.Name())) {
		case ".jpg", ".jpeg", ".png", ".bmp":
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

func checkIndex(i, n int) error {
	if i < 0 || i >= n {
		return fmt.Errorf("dataset: frame %d out of range [0, %d)", i, n)
	}
	return nil
}
