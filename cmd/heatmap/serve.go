package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"heatmap/internal/server"
	"heatmap/web"
)

func init() {
	commands["serve"] = cmdServe
}

func cmdServe(args []string, stdout, stderr io.Writer) int {
	fs := newFlags("serve", stderr)
	addr := fs.String("addr", "127.0.0.1:8080", "listen address (keep it on localhost unless you mean to share it)")
	pipelinesDir := fs.String("pipelines", defaultPipelines, "directory with pipeline YAML files")
	dataDir := fs.String("data", defaultData, "data directory (cdnet/, frames/)")
	results := fs.String("results", defaultResults, "results directory")
	maxFrames := fs.Int("max-frames", 500, "largest number of frames per run started from the UI")
	parallel := fs.Int("parallel", 2, "runs processed at the same time")
	if code, stop := parse(fs, args); stop {
		return code
	}
	logger := log.New(stderr, "serve: ", log.LstdFlags)
	srv := server.New(server.Config{
		PipelinesDir: *pipelinesDir, DataDir: *dataDir, ResultsDir: *results,
		MaxFrames: *maxFrames, Parallel: *parallel, Static: web.Dist(), Version: version, Logger: logger,
	})
	defer srv.Close()

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return fail(stderr, err)
	}
	httpSrv := &http.Server{Handler: srv, ReadHeaderTimeout: 10 * time.Second, ErrorLog: logger}
	_, _ = fmt.Fprintf(stdout, "heatmap UI on http://%s (Ctrl+C to stop)\n", ln.Addr())

	ctx, cancel := signalContext()
	defer cancel()
	errc := make(chan error, 1)
	go func() { errc <- httpSrv.Serve(ln) }()
	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return fail(stderr, err)
		}
	case <-ctx.Done():
		shutdown, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = httpSrv.Shutdown(shutdown)
	}
	return 0
}
