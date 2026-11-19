package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func newTestServer(t *testing.T) (*Server, *httptest.Server, string) {
	t.Helper()
	results := t.TempDir()
	s := New(Config{
		PipelinesDir: "../../configs/pipelines",
		DataDir:      t.TempDir(),
		ResultsDir:   results,
		MaxFrames:    40,
		Version:      "test",
		Static: fstest.MapFS{
			"index.html":    {Data: []byte("<html>ui</html>")},
			"assets/app.js": {Data: []byte("js")},
		},
	})
	ts := httptest.NewServer(s)
	t.Cleanup(func() {
		ts.Close()
		s.Close()
	})
	return s, ts, results
}

func getJSON(t *testing.T, url string, v any) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if v != nil {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			t.Fatalf("%s: %v", url, err)
		}
	}
	return resp.StatusCode
}

func post(t *testing.T, url, body string) (int, map[string]string) {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestCatalogEndpoints(t *testing.T) {
	_, ts, _ := newTestServer(t)
	var health map[string]string
	if code := getJSON(t, ts.URL+"/api/health", &health); code != 200 || health["version"] != "test" {
		t.Errorf("health %d %v", code, health)
	}
	var methods []map[string]any
	if getJSON(t, ts.URL+"/api/methods", &methods); len(methods) < 25 || methods[0]["params"] == nil {
		t.Errorf("methods: %d", len(methods))
	}
	var pipes []map[string]any
	if getJSON(t, ts.URL+"/api/pipelines", &pipes); len(pipes) != 8 || pipes[0]["stage_info"] == nil {
		t.Errorf("pipelines: %v", pipes)
	}
	var seqs []map[string]any
	if getJSON(t, ts.URL+"/api/sequences", &seqs); len(seqs) != 4 {
		t.Errorf("sequences: %d", len(seqs))
	}
	resp, err := http.Get(ts.URL + "/api/sequences/preview?id=synthetic/street&frame=40")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "image/png" || !bytes.HasPrefix(body, []byte("\x89PNG")) {
		t.Errorf("preview %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	for _, q := range []string{"id=synthetic/ocean", "id=/etc", "id=synthetic/street&frame=x", "id=synthetic/street&frame=999"} {
		if code := getJSON(t, ts.URL+"/api/sequences/preview?"+q, nil); code < 400 {
			t.Errorf("preview %s: %d", q, code)
		}
	}
	if code := getJSON(t, ts.URL+"/api/nope", nil); code != 404 {
		t.Errorf("unknown API path: %d", code)
	}
}

func TestStaticUI(t *testing.T) {
	_, ts, _ := newTestServer(t)
	for path, want := range map[string]string{"/": "ui", "/runs/abc": "ui", "/assets/app.js": "js"} {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 || !strings.Contains(string(body), want) {
			t.Errorf("%s: %d %q", path, resp.StatusCode, body)
		}
	}
	bare := httptest.NewServer(New(Config{}))
	defer bare.Close()
	if code := getJSON(t, bare.URL+"/", nil); code != 404 {
		t.Errorf("no UI: %d", code)
	}
	empty := httptest.NewServer(New(Config{Static: fstest.MapFS{}}))
	defer empty.Close()
	if code := getJSON(t, empty.URL+"/", nil); code != 404 {
		t.Errorf("unbuilt UI: %d", code)
	}
}

// readEvents reads SSE events until "done" and returns their names.
func readEvents(t *testing.T, url string) []string {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content type %q", ct)
	}
	var names []string
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		if name, ok := strings.CutPrefix(sc.Text(), "event: "); ok {
			names = append(names, name)
			if name == "done" {
				break
			}
		}
	}
	return names
}

func waitStatus(t *testing.T, url, want string) RunDetail {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for {
		var d RunDetail
		getJSON(t, url, &d)
		if d.Status == want {
			return d
		}
		if time.Now().After(deadline) {
			t.Fatalf("status %q, want %q (%s)", d.Status, want, d.Error)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestRunLifecycle(t *testing.T) {
	_, ts, results := newTestServer(t)
	code, out := post(t, ts.URL+"/api/runs", `{"pipeline":"p5","sequence":"synthetic/street","noise":"gaussian:5","seed":3,"frames":25}`)
	if code != http.StatusAccepted || out["run_id"] == "" {
		t.Fatalf("start: %d %v", code, out)
	}
	id := out["run_id"]

	events := readEvents(t, ts.URL+"/api/runs/"+id+"/events")
	if len(events) == 0 || events[len(events)-1] != "done" {
		t.Errorf("events %v", events)
	}
	d := waitStatus(t, ts.URL+"/api/runs/"+id, StatusDone)
	if d.Run == nil || d.Run.Summary.Frames != 25 || !d.Run.SavedFrames || len(d.Run.StageKeys) != 3 {
		t.Fatalf("detail %+v", d)
	}
	// A run of this server also says what it is processing, for the live view.
	if d.Live == nil || d.Live.Scene.ID != "synthetic/street" || d.Live.Scene.Width != 160 ||
		d.Live.Start != d.Run.Summary.Start || d.Live.Request.Pipeline != "p5" {
		t.Errorf("live info %+v", d.Live)
	}

	// A finished run streams a single done event.
	if ev := readEvents(t, ts.URL+"/api/runs/"+id+"/events"); len(ev) != 1 || ev[0] != "done" {
		t.Errorf("events after finish %v", ev)
	}

	for _, f := range []string{"heatmap.png", "overlay.png", "plan_heatmap.png", "frames/20/mask.png", "frames/20/1-fft_filter.png", "metrics.csv"} {
		resp, err := http.Get(ts.URL + "/api/runs/" + id + "/files/" + f)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("file %s: %d", f, resp.StatusCode)
		}
	}
	for _, f := range []string{"../../etc/passwd", "frames", "missing.png", "..%2f..%2frun.json"} {
		resp, err := http.Get(ts.URL + "/api/runs/" + id + "/files/" + f)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode == 200 {
			t.Errorf("file %s must not be served", f)
		}
	}

	var list []RunListItem
	getJSON(t, ts.URL+"/api/runs", &list)
	if len(list) != 1 || list[0].ID != id || list[0].Status != StatusDone || list[0].Summary == nil {
		t.Errorf("list %+v", list)
	}

	// Runs found only on disk (e.g. after a restart) are listed and readable.
	fresh := New(Config{PipelinesDir: "../../configs/pipelines", ResultsDir: results})
	ts2 := httptest.NewServer(fresh)
	defer ts2.Close()
	var list2 []RunListItem
	getJSON(t, ts2.URL+"/api/runs", &list2)
	if len(list2) != 1 || list2[0].Summary == nil {
		t.Errorf("list after restart %+v", list2)
	}
	if code := getJSON(t, ts2.URL+"/api/runs/"+id, nil); code != 200 {
		t.Errorf("detail after restart %d", code)
	}
	if ev := readEvents(t, ts2.URL+"/api/runs/"+id+"/events"); len(ev) != 1 {
		t.Errorf("events after restart %v", ev)
	}

	// Delete removes the files.
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/runs/"+id, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("delete %d", resp.StatusCode)
	}
	if _, err := os.Stat(filepath.Join(results, id)); !os.IsNotExist(err) {
		t.Error("run directory must be gone")
	}
	if code := getJSON(t, ts.URL+"/api/runs/"+id, nil); code != 404 {
		t.Errorf("deleted run: %d", code)
	}
}

func TestCancelRun(t *testing.T) {
	_, ts, results := newTestServer(t)
	_, out := post(t, ts.URL+"/api/runs", `{"pipeline":"p3","sequence":"synthetic/street","frames":40}`)
	id := out["run_id"]
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/runs/"+id, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	deadline := time.Now().Add(30 * time.Second)
	for {
		var det RunDetail
		getJSON(t, ts.URL+"/api/runs/"+id, &det)
		if det.Status == StatusCancelled || det.Status == StatusDone {
			if det.Status == StatusCancelled {
				if _, err := os.Stat(filepath.Join(results, id)); !os.IsNotExist(err) {
					t.Error("a cancelled run must leave no files")
				}
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("status %s", det.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestStartRunValidation(t *testing.T) {
	_, ts, _ := newTestServer(t)
	for _, body := range []string{
		``, `{`, `{"pipeline":"p1"}`, `{"sequence":"synthetic/street"}`,
		`{"pipeline":"p1","sequence":"synthetic/street","bogus":1}`,
		`{"pipeline":"p99","sequence":"synthetic/street"}`,
		`{"pipeline":"../configs/pipelines/p1.yaml","sequence":"synthetic/street"}`,
		`{"pipeline":"p1","sequence":"/etc"}`,
		`{"pipeline":"p1","sequence":"frames/../../x"}`,
		`{"pipeline":"p1","sequence":"synthetic/street","noise":"fog:1"}`,
		`{"pipeline":"p1","sequence":"synthetic/street","frames":41}`,
		`{"pipeline":"p1","sequence":"synthetic/street","frames":-1}`,
		`{"pipeline":"p1","sequence":"synthetic/street","start":500}`,
		`{"pipeline":"p1","sequence":"synthetic/street","start":-2}`,
		`{"pipeline":"p1","sequence":"synthetic/street","decay":2}`,
	} {
		if code, _ := post(t, ts.URL+"/api/runs", body); code != http.StatusBadRequest {
			t.Errorf("%s: code %d, want 400", body, code)
		}
	}
	for _, path := range []string{"/api/runs/bad%20id", "/api/runs/missing", "/api/runs/missing/events", "/api/runs/missing/files/x.png"} {
		if code := getJSON(t, ts.URL+path, nil); code < 400 {
			t.Errorf("%s: %d", path, code)
		}
	}
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/runs/missing", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("delete missing: %d", resp.StatusCode)
	}
}

func TestFailedRunIsReported(t *testing.T) {
	s, ts, _ := newTestServer(t)
	// Make the results directory unwritable by replacing it with a file.
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.cfg.ResultsDir = blocker
	_, out := post(t, ts.URL+"/api/runs", `{"pipeline":"p8","sequence":"synthetic/night","frames":5}`)
	d := waitStatus(t, ts.URL+"/api/runs/"+out["run_id"], StatusFailed)
	if d.Error == "" {
		t.Error("a failed run must carry its error")
	}
	var list []RunListItem
	getJSON(t, ts.URL+"/api/runs", &list)
	if len(list) != 1 || list[0].Status != StatusFailed {
		t.Errorf("list %+v", list)
	}
}
