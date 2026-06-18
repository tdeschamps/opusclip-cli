package root_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tdeschamps/opusclip-cli/internal/auth"
	"github.com/tdeschamps/opusclip-cli/internal/cmd/root"
	"github.com/tdeschamps/opusclip-cli/internal/cmdutil"
	"github.com/tdeschamps/opusclip-cli/internal/iostreams"
	"github.com/tdeschamps/opusclip-cli/internal/text"
)

// newTestFactory wires a Factory with in-memory streams, a memory credential
// store seeded with a token, and a transport that rewrites requests to srv while
// preserving the path — so tests can assert exact request paths.
func newTestFactory(t *testing.T, srv *httptest.Server) (*cmdutil.Factory, *strings.Builder) {
	t.Helper()
	t.Setenv("OPUSCLIP_BASE_URL", "http://opusclip.local")

	io, _, _, _ := iostreams.Test()
	out := &strings.Builder{}
	io.Out = out
	io.SetStdoutTTY(false)

	store := auth.NewMemoryStore()
	_ = store.Set("default", auth.Credential{Token: "test-token", Method: auth.MethodAPIKey})

	cfgPath := t.TempDir() + "/config.toml"
	f := &cmdutil.Factory{
		IOStreams:  io,
		Flags:      &cmdutil.GlobalFlags{},
		Clock:      text.FixedClock(time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC)),
		ConfigPath: cfgPath,
		CredStore:  store,
		Transport:  rewriteTransport{base: srv.URL},
	}
	return f, out
}

// rewriteTransport sends every request to the test server, preserving the path.
type rewriteTransport struct{ base string }

func (rt rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	u := rt.base + req.URL.Path
	if req.URL.RawQuery != "" {
		u += "?" + req.URL.RawQuery
	}
	nr, err := http.NewRequest(req.Method, u, req.Body)
	if err != nil {
		return nil, err
	}
	nr.Header = req.Header
	return http.DefaultTransport.RoundTrip(nr)
}

func runCmd(t *testing.T, f *cmdutil.Factory, args ...string) error {
	t.Helper()
	cmd := root.NewCmdRoot(f)
	cmd.SetArgs(args)
	return cmd.Execute()
}

func TestClipGetSendsAuthAndPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/clip-projects/P1" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("auth header = %q", got)
		}
		_, _ = w.Write([]byte(`{"projectId":"P1","stage":"COMPLETE"}`))
	}))
	defer srv.Close()

	f, out := newTestFactory(t, srv)
	if err := runCmd(t, f, "clip", "get", "P1", "--json"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"projectId": "P1"`) {
		t.Errorf("output missing project:\n%s", out.String())
	}
}

func TestClipCreateSendsOrgHeader(t *testing.T) {
	var sawOrg string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawOrg = r.Header.Get("x-opus-org-id")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"projectId":"P1","stage":"PENDING"}`))
	}))
	defer srv.Close()

	f, _ := newTestFactory(t, srv)
	if err := runCmd(t, f, "--org", "org_123", "clip", "create", "--url", "https://x", "--json"); err != nil {
		t.Fatal(err)
	}
	if sawOrg != "org_123" {
		t.Errorf("x-opus-org-id = %q, want org_123", sawOrg)
	}
}

func TestClipsListCSVColumns(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"P1.c1","projectId":"P1","title":"Hello","durationMs":30000}]`))
	}))
	defer srv.Close()

	f, out := newTestFactory(t, srv)
	if err := runCmd(t, f, "clips", "list", "--project", "P1", "-o", "csv", "--columns", "id,title"); err != nil {
		t.Fatal(err)
	}
	want := "ID,TITLE\nP1.c1,Hello\n"
	if out.String() != want {
		t.Errorf("csv output:\n%q\nwant\n%q", out.String(), want)
	}
}

func TestVersionCommand(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	f, out := newTestFactory(t, srv)
	if err := runCmd(t, f, "version"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "opusclip ") {
		t.Errorf("version output = %q", out.String())
	}
}
