package root_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tdeschamps/opusclip-cli/internal/auth"
	"github.com/tdeschamps/opusclip-cli/internal/cmd/root"
	"github.com/tdeschamps/opusclip-cli/internal/cmdutil"
	"github.com/tdeschamps/opusclip-cli/internal/iostreams"
	"github.com/tdeschamps/opusclip-cli/internal/text"
)

// errorHarness runs commands against a server that always returns a non-retryable
// 400, with a valid stored credential — so the API client error branches execute.
func errorHarness(t *testing.T) func(args ...string) error {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"boom"}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("OPUSCLIP_BASE_URL", srv.URL)

	orig := cmdutil.BrowserRunner
	cmdutil.BrowserRunner = func(string, ...string) error { return nil }
	t.Cleanup(func() { cmdutil.BrowserRunner = orig })

	store := auth.NewMemoryStore()
	_ = store.Set("default", auth.Credential{Token: "t", Method: auth.MethodAPIKey})
	cfgPath := t.TempDir() + "/config.toml"

	return func(args ...string) error {
		io, _, out, errOut := iostreams.Test()
		f := &cmdutil.Factory{
			IOStreams:  io,
			Flags:      &cmdutil.GlobalFlags{MaxRetries: 1},
			Clock:      text.FixedClock(time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC)),
			ConfigPath: cfgPath,
			CredStore:  store,
		}
		cmd := root.NewCmdRoot(f)
		cmd.SetArgs(args)
		cmd.SetOut(out)
		cmd.SetErr(errOut)
		return cmd.Execute()
	}
}

func TestCommandsSurfaceServerErrors(t *testing.T) {
	run := errorHarness(t)
	cmds := [][]string{
		{"clip", "create", "--url", "https://x"},
		{"clip", "get", "P1"},
		{"clips", "list", "--project", "P1"},
		{"clips", "download", "--project", "P1"},
		{"api", "GET", "/api/clip-projects/P1"},
	}
	for _, args := range cmds {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			if err := run(args...); err == nil {
				t.Errorf("%v should surface the server error", args)
			}
		})
	}
}

// brokenConfigHarness points commands at an unparseable config so the early
// f.Config()/f.APIClient() error branches execute.
func brokenConfigHarness(t *testing.T) func(args ...string) error {
	t.Helper()
	path := t.TempDir() + "/config.toml"
	if err := os.WriteFile(path, []byte("== not toml ]["), 0o600); err != nil {
		t.Fatal(err)
	}
	return func(args ...string) error {
		io, _, out, errOut := iostreams.Test()
		f := &cmdutil.Factory{
			IOStreams:  io,
			Flags:      &cmdutil.GlobalFlags{},
			Clock:      text.FixedClock(time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC)),
			ConfigPath: path,
			CredStore:  auth.NewMemoryStore(),
		}
		cmd := root.NewCmdRoot(f)
		cmd.SetArgs(args)
		cmd.SetOut(out)
		cmd.SetErr(errOut)
		return cmd.Execute()
	}
}

func TestCommandsSurfaceConfigErrors(t *testing.T) {
	run := brokenConfigHarness(t)
	cmds := [][]string{
		{"clip", "create", "--url", "https://x"},
		{"clip", "get", "P1"},
		{"clips", "list", "--project", "P1"},
		{"config", "get", "output"},
		{"config", "set", "output", "json"},
		{"config", "list"},
		{"profiles", "list"},
		{"profiles", "use", "x"},
		{"auth", "switch", "x"},
		{"api", "GET", "/x"},
		{"doctor"},
	}
	for _, args := range cmds {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			if err := run(args...); err == nil {
				t.Errorf("%v should surface the config error", args)
			}
		})
	}
}
