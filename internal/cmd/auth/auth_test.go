package auth

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/tdeschamps/opusclip-cli/internal/auth"
	"github.com/tdeschamps/opusclip-cli/internal/cmdutil"
	"github.com/tdeschamps/opusclip-cli/internal/iostreams"
)

// probeServer serves the validation endpoint (/api/exportable-clips) with the
// given status. 200 returns an empty array; other statuses return the raw code
// (optionally with a body for the cap case).
func probeServer(t *testing.T, status int, body string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status == http.StatusOK {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		w.WriteHeader(status)
		if body != "" {
			_, _ = w.Write([]byte(body))
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func authFactory(t *testing.T, baseURL string) (*cmdutil.Factory, *iostreams.IOStreams) {
	t.Helper()
	t.Setenv("OPUSCLIP_BASE_URL", baseURL)
	io, _, _, _ := iostreams.Test()
	return &cmdutil.Factory{
		IOStreams:  io,
		Flags:      &cmdutil.GlobalFlags{},
		ConfigPath: t.TempDir() + "/c.toml",
		CredStore:  auth.NewMemoryStore(),
	}, io
}

func run(t *testing.T, f *cmdutil.Factory, args ...string) error {
	t.Helper()
	cmd := NewCmdAuth(f)
	cmd.SetArgs(args)
	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)
	return cmd.Execute()
}

func TestLoginWithTokenValid(t *testing.T) {
	f, io := authFactory(t, probeServer(t, http.StatusOK, ""))
	writeStdin(io, "sk_live_token_abcd1234\n")
	if err := run(t, f, "login", "--with-token"); err != nil {
		t.Fatalf("login --with-token: %v", err)
	}
	store, _ := f.CredentialStore()
	cred, err := store.Get("default")
	if err != nil || cred.Token != "sk_live_token_abcd1234" {
		t.Errorf("credential not stored: %+v %v", cred, err)
	}
}

func TestLoginInvalidKeyAborts(t *testing.T) {
	f, io := authFactory(t, probeServer(t, http.StatusUnauthorized, ""))
	writeStdin(io, "badkey\n")
	err := run(t, f, "login", "--with-token")
	if err == nil {
		t.Fatal("a 401 should abort login")
	}
	if cmdutil.ExitCodeForError(err) != cmdutil.ExitAuth {
		t.Errorf("exit code = %d, want %d", cmdutil.ExitCodeForError(err), cmdutil.ExitAuth)
	}
	// Nothing should have been stored.
	store, _ := f.CredentialStore()
	if _, gerr := store.Get("default"); gerr == nil {
		t.Error("a rejected key must not be stored")
	}
}

func TestLoginCapIsValid(t *testing.T) {
	// A 403 monthly-cap means the key is valid (auth passed) — login proceeds.
	body := `{"code":"API_MONTHLY_CAP_REACHED","reset_at":"2026-07-01"}`
	f, io := authFactory(t, probeServer(t, http.StatusForbidden, body))
	writeStdin(io, "sk_capped\n")
	if err := run(t, f, "login", "--with-token"); err != nil {
		t.Fatalf("a capped (403) key should still log in: %v", err)
	}
	store, _ := f.CredentialStore()
	if cred, _ := store.Get("default"); cred.Token != "sk_capped" {
		t.Errorf("capped key not stored: %+v", cred)
	}
}

func TestLoginStoresOnUnverifiable(t *testing.T) {
	// A 5xx means we can't verify; store anyway with a warning.
	f, io := authFactory(t, probeServer(t, http.StatusBadGateway, ""))
	writeStdin(io, "sk_unverified\n")
	if err := run(t, f, "login", "--with-token"); err != nil {
		t.Fatalf("unverifiable key should still store: %v", err)
	}
	store, _ := f.CredentialStore()
	if cred, _ := store.Get("default"); cred.Token != "sk_unverified" {
		t.Errorf("unverified key not stored: %+v", cred)
	}
}

func TestLoginSkipValidation(t *testing.T) {
	// Point at a server that would 401, but --skip-validation never probes it.
	f, io := authFactory(t, probeServer(t, http.StatusUnauthorized, ""))
	writeStdin(io, "sk_unchecked\n")
	if err := run(t, f, "login", "--with-token", "--skip-validation"); err != nil {
		t.Fatalf("--skip-validation should store without probing: %v", err)
	}
	store, _ := f.CredentialStore()
	if cred, _ := store.Get("default"); cred.Token != "sk_unchecked" {
		t.Errorf("unchecked key not stored: %+v", cred)
	}
}

func TestLoginStoresOrg(t *testing.T) {
	f, io := authFactory(t, probeServer(t, http.StatusOK, ""))
	writeStdin(io, "sk_key\n")
	if err := run(t, f, "login", "--with-token", "--org", "org_42"); err != nil {
		t.Fatalf("login --org: %v", err)
	}
	cfg, _ := f.Config()
	if cfg.ProfileOrDefault("default").OrgID != "org_42" {
		t.Errorf("org not persisted: %+v", cfg.ProfileOrDefault("default"))
	}
}

func TestLoginEmptyKey(t *testing.T) {
	f, io := authFactory(t, probeServer(t, http.StatusOK, ""))
	writeStdin(io, "\n")
	if err := run(t, f, "login", "--with-token"); err == nil {
		t.Error("empty key should be a usage error")
	}
}

func TestLoginNoPrompt(t *testing.T) {
	f, _ := authFactory(t, probeServer(t, http.StatusOK, ""))
	// neverPrompt is true for Test() streams → interactive paste path errors.
	if err := run(t, f, "login"); err == nil {
		t.Error("non-interactive login without --with-token should error")
	}
}

func TestLoginInteractivePaste(t *testing.T) {
	f, io := authFactory(t, probeServer(t, http.StatusOK, ""))
	io.SetNeverPrompt(false)
	writeStdin(io, "pasted-key-value\n")
	if err := run(t, f, "login"); err != nil {
		t.Fatalf("interactive paste login: %v", err)
	}
	store, _ := f.CredentialStore()
	if cred, _ := store.Get("default"); cred.Token != "pasted-key-value" {
		t.Errorf("paste flow stored %q", cred.Token)
	}
}

func TestLoginWebOpensBrowser(t *testing.T) {
	opened := false
	orig := cmdutil.BrowserRunner
	cmdutil.BrowserRunner = func(string, ...string) error { opened = true; return nil }
	defer func() { cmdutil.BrowserRunner = orig }()

	f, _ := authFactory(t, probeServer(t, http.StatusOK, ""))
	// --web opens the dashboard, then (non-interactively) fails to prompt.
	if err := run(t, f, "login", "--web"); err == nil {
		t.Error("non-interactive --web login should still fail to prompt")
	}
	if !opened {
		t.Error("--web should open the browser")
	}
}

func TestStatusLoggedOut(t *testing.T) {
	f, _ := authFactory(t, probeServer(t, http.StatusOK, ""))
	if err := run(t, f, "status"); err == nil {
		t.Error("status should fail when not logged in")
	}
}

func TestStatusLoggedIn(t *testing.T) {
	f, _ := authFactory(t, probeServer(t, http.StatusOK, ""))
	store, _ := f.CredentialStore()
	_ = store.Set("default", auth.Credential{Token: "sk_abcd1234", Method: auth.MethodAPIKey})
	if err := run(t, f, "status"); err != nil {
		t.Fatalf("status: %v", err)
	}
}

func TestMustResolve(t *testing.T) {
	f, _ := authFactory(t, "http://example.test")
	if got := mustResolve(f, "base_url"); got != "http://example.test" {
		t.Errorf("mustResolve = %q", got)
	}
}

func TestMustResolveBadConfigFallsBack(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	path := t.TempDir() + "/c.toml"
	_ = os.WriteFile(path, []byte("== bad ]["), 0o600)
	f := &cmdutil.Factory{IOStreams: io, Flags: &cmdutil.GlobalFlags{}, ConfigPath: path}
	if got := mustResolve(f, "base_url"); got != "https://api.opus.pro" {
		t.Errorf("fallback = %q", got)
	}
}

// writeStdin replaces the stream's stdin with a reader over s.
func writeStdin(io *iostreams.IOStreams, s string) {
	io.In = strings.NewReader(s)
}
