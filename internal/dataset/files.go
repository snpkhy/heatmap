package dataset

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"heatmap/internal/imgx"
)

// CDnet is a CDnet 2014 sequence:
//
//	<dir>/input/in000001.jpg …        frames
//	<dir>/groundtruth/gt000001.png …  labels (0, 50, 85, 170, 255)
//	<dir>/temporalROI.txt             "first last", 1-based: frames with valid labels
//
// Ground truth is only reported for frames inside the temporal ROI; CDnet
// leaves the labels of earlier frames (meant for background learning) empty.
type CDnet struct {
	info  Info
	dir   string
	files []string // input file names, sorted
}

// OpenCDnet opens a CDnet sequence directory. id is reported in Info.
func OpenCDnet(dir, id string) (*CDnet, error) {
	files, err := imageFiles(filepath.Join(dir, "input"))
	if err != nil || len(files) == 0 {
		return nil, fmt.Errorf("%w: %s has no input frames", ErrNotFound, dir)
	}
	s := &CDnet{dir: dir, files: files}
	s.info = Info{
		ID:        id,
		Name:      filepath.Base(dir),
		Category:  filepath.Base(filepath.Dir(dir)),
		Source:    SourceCDnet,
		Path:      dir,
		Frames:    len(files),
		EvalStart: -1,
		EvalEnd:   -1,
	}
	if first, last, ok := readTemporalROI(filepath.Join(dir, "temporalROI.txt")); ok &&
		isDir(filepath.Join(dir, "groundtruth")) {
		// Keep only the part of the ROI that has frames; an ROI entirely past
		// the end means there is no usable ground truth.
		start, end := max(first-1, 0), min(last-1, len(files)-1)
		if start <= end {
			s.info.HasGT, s.info.EvalStart, s.info.EvalEnd = true, start, end
		}
	}
	if err := fillSize(&s.info, s); err != nil {
		return nil, err
	}
	return s, nil
}

// Info implements Sequence.
func (s *CDnet) Info() Info { return s.info }

// Frame implements Sequence.
func (s *CDnet) Frame(i int) (*imgx.RGB, error) {
	if err := checkIndex(i, len(s.files)); err != nil {
		return nil, err
	}
	return imgx.Load(filepath.Join(s.dir, "input", s.files[i]))
}

// GroundTruth implements Sequence. The label file has the number of the input
// file: in000470.jpg ↔ gt000470.png.
func (s *CDnet) GroundTruth(i int) (*imgx.Gray, error) {
	if err := checkIndex(i, len(s.files)); err != nil {
		return nil, err
	}
	if !s.info.HasGT || i < s.info.EvalStart || i > s.info.EvalEnd {
		return nil, nil
	}
	num := strings.TrimPrefix(strings.TrimSuffix(s.files[i], filepath.Ext(s.files[i])), "in")
	gt, err := imgx.LoadGray(filepath.Join(s.dir, "groundtruth", "gt"+num+".png"))
	if err != nil {
		return nil, fmt.Errorf("dataset: ground truth of frame %d: %w", i, err)
	}
	return gt, nil
}

// readTemporalROI parses "first last" from temporalROI.txt.
func readTemporalROI(path string) (first, last int, ok bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, false
	}
	f := strings.Fields(string(data))
	if len(f) < 2 {
		return 0, 0, false
	}
	a, err1 := strconv.Atoi(f[0])
	b, err2 := strconv.Atoi(f[1])
	if err1 != nil || err2 != nil || a < 1 || b < a {
		return 0, 0, false
	}
	return a, b, true
}

// Folder is a directory of frames (jpg, png or bmp) in file-name order,
// e.g. produced by scripts/extract_frames.sh. It has no ground truth.
type Folder struct {
	info  Info
	dir   string
	files []string
}

// OpenFolder opens a folder of images. id is reported in Info.
func OpenFolder(dir, id string) (*Folder, error) {
	files, err := imageFiles(dir)
	if err != nil || len(files) == 0 {
		return nil, fmt.Errorf("%w: %s has no images", ErrNotFound, dir)
	}
	s := &Folder{dir: dir, files: files}
	s.info = Info{
		ID:        id,
		Name:      filepath.Base(dir),
		Category:  "custom",
		Source:    SourceFrames,
		Path:      dir,
		Frames:    len(files),
		EvalStart: -1,
		EvalEnd:   -1,
	}
	if err := fillSize(&s.info, s); err != nil {
		return nil, err
	}
	return s, nil
}

// Info implements Sequence.
func (s *Folder) Info() Info { return s.info }

// Frame implements Sequence.
func (s *Folder) Frame(i int) (*imgx.RGB, error) {
	if err := checkIndex(i, len(s.files)); err != nil {
		return nil, err
	}
	return imgx.Load(filepath.Join(s.dir, s.files[i]))
}

// GroundTruth implements Sequence: a folder never has ground truth.
func (s *Folder) GroundTruth(i int) (*imgx.Gray, error) {
	return nil, checkIndex(i, len(s.files))
}

// fillSize reads the first frame to record the frame size.
func fillSize(info *Info, s Sequence) error {
	f, err := s.Frame(0)
	if err != nil {
		return fmt.Errorf("dataset: %s: first frame: %w", info.ID, err)
	}
	info.Width, info.Height = f.W, f.H
	return nil
}
