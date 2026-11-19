// Package server implements `heatmap serve`: a JSON + Server-Sent Events API
// over the pipeline runner, plus the embedded web UI.
//
// All image processing happens here, in Go; the UI only starts runs and shows
// what the API returns.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"heatmap/internal/config"
	"heatmap/internal/dataset"
	"heatmap/internal/imgx"
	"heatmap/internal/noise"
	"heatmap/internal/pipeline"
	"heatmap/internal/report"
)

// Config configures a Server.
type Config struct {
	PipelinesDir string
	DataDir      string
	ResultsDir   string
	// MaxFrames caps the frames of a run started from the UI (they save
	// every stage image of every frame). 0 means 500.
	MaxFrames int
	// Parallel is the number of runs processed at the same time; others
	// wait in a queue. 0 means 2.
	Parallel int
	// Static is the built web UI; nil serves only the API.
	Static fs.FS
	// Version is reported by /api/health.
	Version string
	// Logger receives request errors; nil uses the standard logger.
	Logger *log.Logger
}

// Run states.
const (
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusDone      = "done"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

// Server serves the API and the UI. Create it with New.
type Server struct {
	cfg  Config
	mux  *http.ServeMux
	slot chan struct{}
	ctx  context.Context
	stop context.CancelFunc
	wg   sync.WaitGroup

	mu   sync.Mutex
	runs map[string]*liveRun
}

// liveRun is a run started by this server process.
type liveRun struct {
	ID        string            `json:"run_id"`
	Status    string            `json:"status"`
	Error     string            `json:"error,omitempty"`
	Progress  pipeline.Progress `json:"progress"`
	Request   RunRequest        `json:"request"`
	CreatedAt time.Time         `json:"created_at"`
	scene     dataset.Info
	start     int
	cancel    context.CancelFunc `json:"-"`
	subs      map[chan event]struct{}
}

type event struct {
	name string
	data any
}

// New creates a server. Call Close to stop running jobs.
func New(cfg Config) *Server {
	if cfg.MaxFrames <= 0 {
		cfg.MaxFrames = 500
	}
	if cfg.Parallel <= 0 {
		cfg.Parallel = 2
	}
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}
	ctx, stop := context.WithCancel(context.Background())
	s := &Server{cfg: cfg, mux: http.NewServeMux(), slot: make(chan struct{}, cfg.Parallel),
		runs: map[string]*liveRun{}, ctx: ctx, stop: stop}
	s.routes()
	return s
}

// Close cancels all runs and waits for them to stop.
func (s *Server) Close() {
	s.stop()
	s.wg.Wait()
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.health)
	s.mux.HandleFunc("GET /api/methods", s.methods)
	s.mux.HandleFunc("GET /api/pipelines", s.pipelines)
	s.mux.HandleFunc("GET /api/sequences", s.sequences)
	s.mux.HandleFunc("GET /api/sequences/preview", s.preview)
	s.mux.HandleFunc("GET /api/runs", s.listRuns)
	s.mux.HandleFunc("POST /api/runs", s.startRun)
	s.mux.HandleFunc("GET /api/runs/{id}", s.getRun)
	s.mux.HandleFunc("DELETE /api/runs/{id}", s.deleteRun)
	s.mux.HandleFunc("GET /api/runs/{id}/events", s.events)
	s.mux.HandleFunc("GET /api/runs/{id}/files/{file...}", s.runFile)
	s.mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, errors.New("no such API endpoint"))
	})
	s.mux.HandleFunc("/", s.static)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "version": s.cfg.Version})
}

// methodInfo is a method with its parameters, for the UI.
type methodInfo struct {
	pipeline.Method
	Params []pipeline.ParamSpec `json:"params"`
}

func (s *Server) methods(w http.ResponseWriter, _ *http.Request) {
	var out []methodInfo
	for _, m := range pipeline.Methods() {
		out = append(out, methodInfo{m, m.Params()})
	}
	writeJSON(w, http.StatusOK, out)
}

// pipelineInfo is a pipeline config with the category of each stage.
type pipelineInfo struct {
	*config.Pipeline
	StageInfo []pipeline.StageInfo `json:"stage_info"`
	Error     string               `json:"error,omitempty"`
}

func (s *Server) pipelines(w http.ResponseWriter, _ *http.Request) {
	all, err := config.LoadPipelines(s.cfg.PipelinesDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]pipelineInfo, 0, len(all))
	for _, p := range all {
		info := pipelineInfo{Pipeline: p}
		if built, err := pipeline.Build(p); err != nil {
			info.Error = err.Error()
		} else {
			info.StageInfo = built.Stages()
		}
		out = append(out, info)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) sequences(w http.ResponseWriter, _ *http.Request) {
	infos, err := dataset.Discover(s.cfg.DataDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, infos)
}

// preview returns one frame of a sequence as PNG: ?id=<sequence>&frame=<n>.
func (s *Server) preview(w http.ResponseWriter, r *http.Request) {
	seq, err := dataset.Open(s.cfg.DataDir, r.URL.Query().Get("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	n := 0
	if f := r.URL.Query().Get("frame"); f != "" {
		if _, err := fmt.Sscanf(f, "%d", &n); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("bad frame %q", f))
			return
		}
	}
	img, err := seq.Frame(n)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "max-age=3600")
	if err := encodePNG(w, img); err != nil {
		s.cfg.Logger.Printf("preview: %v", err)
	}
}

// RunRequest is the body of POST /api/runs.
type RunRequest struct {
	Pipeline string `json:"pipeline"`
	Sequence string `json:"sequence"`
	// Noise is "none", "gaussian:<sigma>" or "salt_pepper:<fraction>".
	Noise string `json:"noise"`
	Seed  uint64 `json:"seed"`
	// Start is the first frame, −1 (default when omitted) for automatic.
	Start *int `json:"start,omitempty"`
	// Frames is the number of frames; 0 means MaxFrames.
	Frames int     `json:"frames"`
	Decay  float32 `json:"decay,omitempty"`
}

func (s *Server) startRun(w http.ResponseWriter, r *http.Request) {
	var req RunRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("bad request body: %w", err))
		return
	}
	opts, err := s.prepare(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	ctx, cancel := context.WithCancel(s.ctx)
	lr := &liveRun{ID: opts.RunID, Status: StatusQueued, Request: req, CreatedAt: time.Now().UTC(),
		cancel: cancel, subs: map[chan event]struct{}{}, scene: opts.Sequence.Info(), start: opts.Start,
		Progress: pipeline.Progress{RunID: opts.RunID, Total: opts.Frames}}
	s.mu.Lock()
	s.runs[lr.ID] = lr
	s.mu.Unlock()

	s.wg.Go(func() { s.execute(ctx, lr, opts) })
	writeJSON(w, http.StatusAccepted, map[string]string{"run_id": lr.ID, "status": StatusQueued})
}

// prepare validates a request and turns it into runner options.
func (s *Server) prepare(req RunRequest) (pipeline.Options, error) {
	var opts pipeline.Options
	if req.Pipeline == "" || req.Sequence == "" {
		return opts, errors.New("pipeline and sequence are required")
	}
	// Only ids from the pipelines directory, never arbitrary file paths.
	if strings.ContainsAny(req.Pipeline, `/\`) {
		return opts, fmt.Errorf("bad pipeline id %q", req.Pipeline)
	}
	cfg, err := config.FindPipeline(s.cfg.PipelinesDir, req.Pipeline)
	if err != nil {
		return opts, err
	}
	if _, err := pipeline.Build(cfg); err != nil {
		return opts, err
	}
	seq, err := dataset.Open(s.cfg.DataDir, req.Sequence)
	if err != nil {
		return opts, err
	}
	ns, err := noise.Parse(req.Noise)
	if err != nil {
		return opts, err
	}
	ns.Seed = req.Seed
	if req.Frames < 0 || req.Frames > s.cfg.MaxFrames {
		return opts, fmt.Errorf("frames %d must be in 0..%d", req.Frames, s.cfg.MaxFrames)
	}
	if !(req.Decay >= 0 && req.Decay <= 1) {
		return opts, fmt.Errorf("decay %v must be in (0, 1]", req.Decay)
	}
	info := seq.Info()
	start := -1
	if req.Start != nil {
		start = *req.Start
	}
	if start < -1 || start >= info.Frames {
		return opts, fmt.Errorf("start %d must be -1 or a frame in 0..%d", start, info.Frames-1)
	}
	first := start
	if first < 0 {
		first = pipeline.AutoStart(info)
	}
	frames := req.Frames
	if frames == 0 {
		frames = s.cfg.MaxFrames
	}
	frames = min(frames, info.Frames-first)
	id := pipeline.NewRunID(cfg.ID, info.Name, time.Now())
	return pipeline.Options{
		Pipeline: cfg, Sequence: seq, Noise: ns, Start: first, Frames: frames,
		OutDir: filepath.Join(s.cfg.ResultsDir, id), RunID: id, SaveFrames: true, Decay: req.Decay,
	}, nil
}

func (s *Server) execute(ctx context.Context, lr *liveRun, opts pipeline.Options) {
	defer lr.cancel()
	select {
	case s.slot <- struct{}{}:
		defer func() { <-s.slot }()
	case <-ctx.Done():
		s.finish(lr, StatusCancelled, "")
		return
	}
	s.update(func() { lr.Status = StatusRunning })
	opts.Progress = func(p pipeline.Progress) {
		s.update(func() { lr.Progress = p })
		s.publish(lr, event{"progress", p})
	}
	_, err := pipeline.Run(ctx, opts)
	switch {
	case err == nil:
		s.finish(lr, StatusDone, "")
	case errors.Is(err, context.Canceled):
		_ = os.RemoveAll(opts.OutDir) // a partial run is useless
		s.finish(lr, StatusCancelled, "")
	default:
		s.finish(lr, StatusFailed, err.Error())
	}
}

func (s *Server) update(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn()
}

func (s *Server) finish(lr *liveRun, status, msg string) {
	s.mu.Lock()
	lr.Status, lr.Error = status, msg
	subs := lr.subs
	lr.subs = nil
	s.mu.Unlock()
	for ch := range subs {
		// Non-blocking: a client that went away may have a full buffer. The
		// close still tells a reader that the stream is over.
		select {
		case ch <- event{"done", map[string]string{"status": status, "error": msg}}:
		default:
		}
		close(ch)
	}
}

// publish sends an event to subscribers without blocking the run: a slow
// client simply misses intermediate progress events.
func (s *Server) publish(lr *liveRun, e event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range lr.subs {
		select {
		case ch <- e:
		default:
		}
	}
}

// RunListItem is one entry of GET /api/runs.
type RunListItem struct {
	ID        string             `json:"run_id"`
	Status    string             `json:"status"`
	Error     string             `json:"error,omitempty"`
	Progress  *pipeline.Progress `json:"progress,omitempty"`
	Summary   *report.Summary    `json:"summary,omitempty"`
	Request   *RunRequest        `json:"request,omitempty"`
	CreatedAt time.Time          `json:"created_at"`
}

func (s *Server) listRuns(w http.ResponseWriter, _ *http.Request) {
	items := map[string]RunListItem{}
	entries, _ := os.ReadDir(s.cfg.ResultsDir)
	for _, e := range entries {
		if !e.IsDir() || !pipeline.ValidRunID(e.Name()) {
			continue
		}
		run, err := report.ReadRunJSON(filepath.Join(s.cfg.ResultsDir, e.Name(), "run.json"))
		if err != nil {
			continue // experiment directories, partial runs
		}
		sum := run.Summary
		items[e.Name()] = RunListItem{ID: e.Name(), Status: StatusDone, Summary: &sum, CreatedAt: sum.CreatedAt}
	}
	s.mu.Lock()
	for id, lr := range s.runs {
		if lr.Status == StatusDone {
			if _, onDisk := items[id]; onDisk {
				continue
			}
		}
		p, req := lr.Progress, lr.Request
		items[id] = RunListItem{ID: id, Status: lr.Status, Error: lr.Error, Progress: &p, Request: &req, CreatedAt: lr.CreatedAt}
	}
	s.mu.Unlock()
	out := make([]RunListItem, 0, len(items))
	for _, it := range items {
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	writeJSON(w, http.StatusOK, out)
}

// RunDetail is the body of GET /api/runs/{id}.
type RunDetail struct {
	ID       string             `json:"run_id"`
	Status   string             `json:"status"`
	Error    string             `json:"error,omitempty"`
	Progress *pipeline.Progress `json:"progress,omitempty"`
	// Live describes a run started by this server, so the UI can show its
	// frames while it is still being processed.
	Live *LiveInfo   `json:"live,omitempty"`
	Run  *report.Run `json:"run,omitempty"`
}

// LiveInfo is what the UI needs to follow a run before run.json exists.
type LiveInfo struct {
	Request RunRequest   `json:"request"`
	Scene   dataset.Info `json:"scene"`
	// Start is the first frame of the run; frames are saved under frames/<n>/.
	Start int `json:"start"`
}

func (s *Server) runID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if !pipeline.ValidRunID(id) {
		writeError(w, http.StatusBadRequest, fmt.Errorf("bad run id %q", id))
		return "", false
	}
	return id, true
}

func (s *Server) getRun(w http.ResponseWriter, r *http.Request) {
	id, ok := s.runID(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	lr := s.runs[id]
	var detail RunDetail
	if lr != nil {
		p := lr.Progress
		detail = RunDetail{ID: id, Status: lr.Status, Error: lr.Error, Progress: &p,
			Live: &LiveInfo{Request: lr.Request, Scene: lr.scene, Start: lr.start}}
	}
	s.mu.Unlock()
	if lr == nil || detail.Status == StatusDone {
		run, err := report.ReadRunJSON(filepath.Join(s.cfg.ResultsDir, id, "run.json"))
		if err != nil {
			if lr == nil {
				writeError(w, http.StatusNotFound, fmt.Errorf("no run %q", id))
				return
			}
		} else {
			detail.ID, detail.Status, detail.Run = id, StatusDone, run
		}
	}
	writeJSON(w, http.StatusOK, detail)
}

// deleteRun cancels a running run, or deletes the files of a finished one.
func (s *Server) deleteRun(w http.ResponseWriter, r *http.Request) {
	id, ok := s.runID(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	lr := s.runs[id]
	active := lr != nil && (lr.Status == StatusQueued || lr.Status == StatusRunning)
	if !active && lr != nil {
		delete(s.runs, id)
	}
	s.mu.Unlock()
	if active {
		lr.cancel()
		writeJSON(w, http.StatusAccepted, map[string]string{"run_id": id, "status": "cancelling"})
		return
	}
	dir := filepath.Join(s.cfg.ResultsDir, id)
	if _, err := os.Stat(filepath.Join(dir, "run.json")); err != nil && lr == nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("no run %q", id))
		return
	}
	if err := os.RemoveAll(dir); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"run_id": id, "status": "deleted"})
}

// events streams progress as Server-Sent Events until the run ends.
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	id, ok := s.runID(w, r)
	if !ok {
		return
	}
	flusher, canFlush := w.(http.Flusher)
	s.mu.Lock()
	lr := s.runs[id]
	var ch chan event
	var snapshot pipeline.Progress
	status, msg := StatusDone, ""
	if lr != nil {
		status, msg, snapshot = lr.Status, lr.Error, lr.Progress
		if lr.subs != nil {
			ch = make(chan event, 16)
			lr.subs[ch] = struct{}{}
		}
	}
	s.mu.Unlock()
	if lr == nil {
		if _, err := os.Stat(filepath.Join(s.cfg.ResultsDir, id, "run.json")); err != nil {
			writeError(w, http.StatusNotFound, fmt.Errorf("no run %q", id))
			return
		}
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	send := func(e event) {
		data, _ := json.Marshal(e.data)
		_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.name, data)
		if canFlush {
			flusher.Flush()
		}
	}
	if ch == nil { // already finished
		send(event{"done", map[string]string{"status": status, "error": msg}})
		return
	}
	send(event{"progress", snapshot})
	defer func() {
		s.mu.Lock()
		if lr.subs != nil {
			delete(lr.subs, ch)
		}
		s.mu.Unlock()
	}()
	for {
		select {
		case e, open := <-ch:
			if !open {
				return
			}
			send(e)
			if e.name == "done" {
				return
			}
		case <-r.Context().Done():
			return
		}
	}
}

// runFile serves a file from a run directory (heatmap.png, frames/12/mask.png, metrics.csv, …).
// os.Root confines the lookup to the run directory, so "../" cannot escape it.
func (s *Server) runFile(w http.ResponseWriter, r *http.Request) {
	id, ok := s.runID(w, r)
	if !ok {
		return
	}
	root, err := os.OpenRoot(filepath.Join(s.cfg.ResultsDir, id))
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("no run %q", id))
		return
	}
	defer func() { _ = root.Close() }()
	name := path.Clean(r.PathValue("file"))
	f, err := root.Open(filepath.FromSlash(name))
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("no file %q in run %s", name, id))
		return
	}
	defer func() { _ = f.Close() }()
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		writeError(w, http.StatusNotFound, fmt.Errorf("no file %q in run %s", name, id))
		return
	}
	if s.isFinished(id) {
		w.Header().Set("Cache-Control", "max-age=86400, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	http.ServeContent(w, r, name, st.ModTime(), f)
}

func (s *Server) isFinished(id string) bool {
	s.mu.Lock()
	lr := s.runs[id]
	s.mu.Unlock()
	return lr == nil || lr.Status == StatusDone
}

// static serves the embedded UI. Unknown paths get index.html, so client-side
// routes such as /runs/<id> survive a page reload.
func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Static == nil {
		writeError(w, http.StatusNotFound, errors.New("the web UI is not built into this binary (run make build)"))
		return
	}
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "" {
		name = "index.html"
	}
	if st, err := fs.Stat(s.cfg.Static, name); err != nil || st.IsDir() {
		name = "index.html"
	}
	if _, err := fs.Stat(s.cfg.Static, "index.html"); err != nil {
		writeError(w, http.StatusNotFound, errors.New("the web UI is not built (run make web-build)"))
		return
	}
	if strings.HasPrefix(name, "assets/") {
		w.Header().Set("Cache-Control", "max-age=31536000, immutable") // hashed file names
	}
	http.ServeFileFS(w, r, s.cfg.Static, name)
}

func encodePNG(w http.ResponseWriter, img *imgx.RGB) error {
	return png.Encode(w, img.ToImage())
}
