package root_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tdeschamps/opusclip-cli/internal/auth"
	"github.com/tdeschamps/opusclip-cli/internal/cmd/root"
	"github.com/tdeschamps/opusclip-cli/internal/cmdutil"
	"github.com/tdeschamps/opusclip-cli/internal/iostreams"
	"github.com/tdeschamps/opusclip-cli/internal/text"
)

// fullStub serves the slice of the OpusClip API the command tests exercise:
// clip-project create/get (with scripted stage transitions), the bare-array
// exportable-clips list, and a fake GCS route serving a tiny mp4.
func fullStub(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	// Per-project stage counters so successive GETs advance toward COMPLETE.
	var mu sync.Mutex
	polls := map[string]int{}
	stages := []string{"IMPORT", "CURATE", "RENDER", "COMPLETE"}

	mux.HandleFunc("/api/clip-projects", func(w http.ResponseWriter, r *http.Request) {
		// POST create only (GET-by-id is handled by the /{id} route below).
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"P1.run","projectId":"P1","stage":"PENDING","sourcePlatform":"youtube","createdAt":"2026-06-17T10:00:00Z"}`))
	})
	mux.HandleFunc("/api/clip-projects/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/clip-projects/")
		if id == "STALLED" {
			_, _ = w.Write([]byte(`{"projectId":"STALLED","stage":"STALLED","error":"import failed: unreachable url"}`))
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
		_, _ = w.Write([]byte(`[{"id":"P1.c1","projectId":"P1","curationId":"c1","title":"Best Moment","durationMs":30000,"genre":"podcast","uriForExport":"` + gcsURL + `","createdAt":"2026-06-17T10:05:00Z"}]`))
	})

	mux.HandleFunc("/gcs/clip.mp4", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("FAKE-MP4-BYTES"))
	})

	srv := httptest.NewServer(mux)
	gcsURL = srv.URL + "/gcs/clip.mp4"
	t.Cleanup(srv.Close)
	return srv
}

// gcsURL is the stub export URL, filled in once the server is up.
var gcsURL string

func harness(t *testing.T) func(args ...string) (string, string, error) {
	t.Helper()
	srv := fullStub(t)
	t.Setenv("OPUSCLIP_BASE_URL", srv.URL)

	// Stub the browser so `open`/`--web` commands don't launch anything.
	orig := cmdutil.BrowserRunner
	cmdutil.BrowserRunner = func(string, ...string) error { return nil }
	t.Cleanup(func() { cmdutil.BrowserRunner = orig })

	store := auth.NewMemoryStore()
	_ = store.Set("default", auth.Credential{Token: "test-token", Method: auth.MethodAPIKey})
	cfgPath := t.TempDir() + "/config.toml"

	return func(args ...string) (string, string, error) {
		io, _, outBuf, errBuf := iostreams.Test()
		f := &cmdutil.Factory{
			IOStreams:  io,
			Flags:      &cmdutil.GlobalFlags{},
			Clock:      text.FixedClock(time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC)),
			ConfigPath: cfgPath,
			CredStore:  store,
		}
		cmd := root.NewCmdRoot(f)
		cmd.SetArgs(args)
		cmd.SetOut(outBuf)
		cmd.SetErr(errBuf)
		err := cmd.Execute()
		return outBuf.String(), errBuf.String(), err
	}
}

func TestClipCreateAndGet(t *testing.T) {
	run := harness(t)
	if out, errOut, err := run("clip", "create", "--url", "https://youtu.be/x", "--json"); err != nil {
		t.Fatalf("create: %v (%s)", err, errOut)
	} else if !strings.Contains(out, "P1") {
		t.Errorf("create output missing projectId: %s", out)
	}
	if out, errOut, err := run("clip", "get", "P1", "--json"); err != nil {
		t.Fatalf("get: %v (%s)", err, errOut)
	} else if !strings.Contains(out, "P1") {
		t.Errorf("get output missing project: %s", out)
	}
}

func TestClipCreateValidation(t *testing.T) {
	run := harness(t)
	if _, _, err := run("clip", "create"); err == nil {
		t.Error("create without --url should fail")
	}
	if _, _, err := run("clip", "create", "--url", "x", "--duration", "bogus"); err == nil {
		t.Error("create with bad --duration should fail")
	}
}

func TestClipWatchCompletes(t *testing.T) {
	run := harness(t)
	// Fast interval so the stage counter reaches COMPLETE quickly.
	out, errOut, err := run("clip", "watch", "P1", "--interval", "1ms")
	if err != nil {
		t.Fatalf("watch: %v (%s)", err, errOut)
	}
	if !strings.Contains(out+errOut, "ready") {
		t.Errorf("watch should report completion: out=%s err=%s", out, errOut)
	}
}

func TestClipWatchStalled(t *testing.T) {
	run := harness(t)
	_, errOut, err := run("clip", "watch", "STALLED", "--interval", "1ms")
	if err == nil {
		t.Fatal("watch on a stalled project should error")
	}
	if cmdutil.ExitCodeForError(err) != cmdutil.ExitUpstream {
		t.Errorf("stalled exit code = %d, want %d", cmdutil.ExitCodeForError(err), cmdutil.ExitUpstream)
	}
	_ = errOut
}

func TestClipCreateWait(t *testing.T) {
	run := harness(t)
	out, errOut, err := run("clip", "create", "--url", "https://youtu.be/x", "--wait", "--interval", "1ms")
	if err != nil {
		t.Fatalf("create --wait: %v (%s)", err, errOut)
	}
	if !strings.Contains(out+errOut, "ready") {
		t.Errorf("create --wait should report completion: out=%s err=%s", out, errOut)
	}
}

func TestClipsListAndDownload(t *testing.T) {
	run := harness(t)
	if out, errOut, err := run("clips", "list", "--project", "P1", "--json"); err != nil {
		t.Fatalf("list: %v (%s)", err, errOut)
	} else if !strings.Contains(out, "Best Moment") {
		t.Errorf("list missing clip: %s", out)
	}
	dir := t.TempDir()
	if _, errOut, err := run("clips", "download", "--project", "P1", "--out", dir); err != nil {
		t.Fatalf("download: %v (%s)", err, errOut)
	}
}

func TestClipsListTable(t *testing.T) {
	run := harness(t)
	if out, errOut, err := run("clips", "list", "--project", "P1", "-o", "table"); err != nil {
		t.Fatalf("list table: %v (%s)", err, errOut)
	} else if !strings.Contains(out, "Best Moment") {
		t.Errorf("table missing clip: %s", out)
	}
	if _, _, err := run("clips", "list", "--project", "P1", "-o", "bogus"); err == nil {
		t.Error("bad format should error")
	}
}

func TestClipsDownloadDryRun(t *testing.T) {
	run := harness(t)
	dir := t.TempDir()
	if _, errOut, err := run("clips", "download", "--project", "P1", "--out", dir, "--dry-run"); err != nil {
		t.Fatalf("download dry-run: %v (%s)", err, errOut)
	} else if !strings.Contains(errOut, "would download") {
		t.Errorf("dry-run should announce, not download: %s", errOut)
	}
}

func TestAPICommand(t *testing.T) {
	run := harness(t)
	out, errOut, err := run("api", "GET", "/api/clip-projects/P1")
	if err != nil || !strings.Contains(out, "P1") {
		t.Fatalf("api GET: %v %s (%s)", err, out, errOut)
	}
	if _, _, err := run("api", "/api/clip-projects/P1"); err != nil {
		t.Fatalf("api shorthand: %v", err)
	}
	if _, _, err := run("api", "GET", "/api/clip-projects/P1", "--param", "novalue"); err == nil {
		t.Error("expected bad --param error")
	}
	// --paginate walks the bare-array exportable-clips list.
	if out, _, err := run("api", "GET", "/api/exportable-clips", "--param", "projectId=P1", "--paginate"); err != nil || !strings.Contains(out, "Best Moment") {
		t.Fatalf("api paginate: %v %s", err, out)
	}
}

func TestConfigAndProfileCommands(t *testing.T) {
	run := harness(t)
	if _, _, err := run("config", "set", "output", "json"); err != nil {
		t.Fatalf("config set: %v", err)
	}
	if out, _, err := run("config", "get", "output"); err != nil || !strings.Contains(out, "json") {
		t.Fatalf("config get: %v %s", err, out)
	}
	if out, _, err := run("config", "list"); err != nil || !strings.Contains(out, "base_url") {
		t.Fatalf("config list: %v %s", err, out)
	}
	if _, _, err := run("config", "set", "bogus", "x"); err == nil {
		t.Error("expected unknown-key error")
	}
	if _, _, err := run("config", "set", "default_limit", "notint"); err == nil {
		t.Error("expected int parse error")
	}
	if _, _, err := run("profiles", "use", "work"); err != nil {
		t.Fatalf("profiles use: %v", err)
	}
	if out, _, err := run("profiles", "list"); err != nil || !strings.Contains(out, "work") {
		t.Fatalf("profiles list: %v %s", err, out)
	}
}

func TestAuthCommands(t *testing.T) {
	run := harness(t)
	if out, _, err := run("auth", "status"); err != nil || !strings.Contains(out, "Method") {
		t.Fatalf("auth status: %v %s", err, out)
	}
	if _, _, err := run("auth", "token"); err == nil {
		t.Error("auth token without --confirm should fail")
	}
	if out, _, err := run("auth", "token", "--confirm"); err != nil || !strings.Contains(out, "test-token") {
		t.Fatalf("auth token --confirm: %v %s", err, out)
	}
	if _, errOut, err := run("auth", "logout"); err != nil || !strings.Contains(errOut, "Logged out") {
		t.Fatalf("auth logout: %v %s", err, errOut)
	}
	if _, errOut, err := run("auth", "logout"); err != nil || !strings.Contains(errOut, "No credentials") {
		t.Fatalf("auth logout (repeat): %v %s", err, errOut)
	}
	if _, _, err := run("auth", "switch", "other"); err != nil {
		t.Fatalf("auth switch: %v", err)
	}
}

func TestAuthLoginWithToken(t *testing.T) {
	srv := fullStub(t)
	t.Setenv("OPUSCLIP_BASE_URL", srv.URL)
	store := auth.NewMemoryStore()
	io, in, out, errBuf := iostreams.Test()
	in.WriteString("sk_test_key\n")
	f := &cmdutil.Factory{
		IOStreams:  io,
		Flags:      &cmdutil.GlobalFlags{},
		Clock:      text.FixedClock(time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC)),
		ConfigPath: t.TempDir() + "/config.toml",
		CredStore:  store,
	}
	cmd := root.NewCmdRoot(f)
	cmd.SetArgs([]string{"auth", "login", "--with-token"})
	cmd.SetOut(out)
	cmd.SetErr(errBuf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("login: %v (%s)", err, errBuf.String())
	}
	if cred, err := store.Get("default"); err != nil || cred.Token != "sk_test_key" {
		t.Errorf("stored cred = %+v, err %v", cred, err)
	}
}

func TestMiscCommands(t *testing.T) {
	run := harness(t)
	for _, args := range [][]string{
		{"version"},
		{"docs"},
		{"docs", "--web"},
		{"update"},
		{"completion", "bash"},
		{"completion", "zsh"},
		{"completion", "fish"},
		{"completion", "powershell"},
	} {
		if _, errOut, err := run(args...); err != nil {
			t.Errorf("%v: %v (%s)", args, err, errOut)
		}
	}
}
