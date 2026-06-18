package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

// TestMain registers the real opusclip entrypoint as a testscript command so the
// .txtar scripts in test/script drive the actual binary behavior (the gh/go
// approach to CLI e2e testing).
func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"opusclip": func() { os.Exit(run()) },
	})
}

// stubHandler emulates the slice of the OpusClip API the e2e scripts exercise:
// clip-project create/get (with scripted stage transitions), the bare-array
// exportable-clips list, and a fake GCS export route.
func stubHandler(gcsURL func() string) http.Handler {
	mux := http.NewServeMux()

	var mu sync.Mutex
	polls := map[string]int{}
	stages := []string{"IMPORT", "CURATE", "RENDER", "COMPLETE"}

	mux.HandleFunc("/api/clip-projects", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"projectId":"P1","stage":"PENDING","sourcePlatform":"youtube","createdAt":"2026-06-17T10:00:00Z"}`))
	})
	mux.HandleFunc("/api/clip-projects/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/clip-projects/")
		if id == "STALLED" {
			_, _ = w.Write([]byte(`{"projectId":"STALLED","stage":"STALLED","error":"import failed"}`))
			return
		}
		mu.Lock()
		n := polls[id]
		polls[id]++
		mu.Unlock()
		stage := stages[min(n, len(stages)-1)]
		_, _ = w.Write([]byte(`{"projectId":"` + id + `","stage":"` + stage + `","sourcePlatform":"youtube","createdAt":"2026-06-17T10:00:00Z"}`))
	})
	mux.HandleFunc("/api/exportable-clips", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("projectId") == "validate-probe" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		_, _ = w.Write([]byte(`[{"id":"P1.c1","projectId":"P1","title":"Best Moment","durationMs":30000,"genre":"podcast","uriForExport":"` + gcsURL() + `","createdAt":"2026-06-17T10:05:00Z"}]`))
	})
	mux.HandleFunc("/gcs/clip.mp4", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("FAKE-MP4-BYTES"))
	})
	return mux
}

func TestScripts(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(stubHandler(func() string { return srv.URL + "/gcs/clip.mp4" }))
	t.Cleanup(srv.Close)

	testscript.Run(t, testscript.Params{
		Dir: "../../test/script",
		Setup: func(e *testscript.Env) error {
			e.Setenv("OPUSCLIP_BASE_URL", srv.URL)
			e.Setenv("OPUSCLIP_API_KEY", "test-key")
			e.Setenv("HOME", e.WorkDir)
			e.Setenv("XDG_CONFIG_HOME", e.WorkDir+"/.config")
			// Force the file-backed credential store so the auth scripts stay
			// hermetic: without this, `auth login` writes to the real OS keychain
			// (which the redirected HOME can't isolate), and on macOS it can pop
			// an interactive keychain prompt.
			e.Setenv("OPUSCLIP_NO_KEYRING", "1")
			return nil
		},
	})
}
