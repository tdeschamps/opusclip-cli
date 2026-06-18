package root_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tdeschamps/opusclip-cli/internal/auth"
	"github.com/tdeschamps/opusclip-cli/internal/cmd/root"
	"github.com/tdeschamps/opusclip-cli/internal/cmdutil"
	"github.com/tdeschamps/opusclip-cli/internal/iostreams"
)

func TestPresentationFlags(t *testing.T) {
	run := harness(t)
	for _, args := range [][]string{
		{"version", "--color", "always"},
		{"version", "--color", "never"},
		{"version", "--no-color"},
		{"version", "--quiet"},
	} {
		if _, _, err := run(args...); err != nil {
			t.Errorf("%v: %v", args, err)
		}
	}
}

func TestVersionFlag(t *testing.T) {
	run := harness(t)
	out, _, err := run("--version")
	if err != nil || !strings.Contains(out, "opusclip ") {
		t.Fatalf("--version: %v %s", err, out)
	}
}

func TestNoArgsShowsHelp(t *testing.T) {
	run := harness(t)
	out, _, err := run()
	if err != nil || !strings.Contains(out, "Core commands") {
		t.Fatalf("no args (non-TTY) should show cobra help: %v %s", err, out)
	}
}

func TestNoArgsOnTTYShowsBrandedScreen(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	t.Setenv("OPUSCLIP_BASE_URL", srv.URL)

	io, _, out, errBuf := iostreams.Test()
	io.SetStdoutTTY(true)
	f := &cmdutil.Factory{
		IOStreams:  io,
		Flags:      &cmdutil.GlobalFlags{},
		ConfigPath: t.TempDir() + "/c.toml",
		CredStore:  auth.NewMemoryStore(),
	}
	cmd := root.NewCmdRoot(f)
	cmd.SetArgs([]string{})
	cmd.SetOut(out)
	cmd.SetErr(errBuf)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "█") || !strings.Contains(s, "Commands") || !strings.Contains(s, "opusclip clip create") {
		t.Errorf("TTY no-args should render the branded screen:\n%s", s)
	}
}

func TestHelpBoldStandardTitlesWithColorEnabled(t *testing.T) {
	run := harness(t)
	out, _, err := run("clip", "--help", "--color", "always")
	if err != nil {
		t.Fatalf("clip --help --color always: %v", err)
	}
	for _, want := range []string{
		"\x1b[1mUsage:\x1b[0m",
		"\x1b[1mAvailable Commands:\x1b[0m",
		"\x1b[1mFlags:\x1b[0m",
		"\x1b[1mGlobal Flags:\x1b[0m",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected bold heading %q in help output:\n%s", want, out)
		}
	}
}

func TestHelpTitlesRemainPlainWhenColorDisabled(t *testing.T) {
	run := harness(t)
	out, _, err := run("clip", "--help", "--color", "never")
	if err != nil {
		t.Fatalf("clip --help --color never: %v", err)
	}
	for _, want := range []string{"Usage:", "Available Commands:", "Flags:", "Global Flags:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected plain heading %q in help output:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b[1m") {
		t.Fatalf("did not expect ANSI bold sequences with color disabled:\n%s", out)
	}
}

func TestCompletionBadShell(t *testing.T) {
	run := harness(t)
	if _, _, err := run("completion", "tcsh"); err == nil {
		t.Error("unsupported shell should error")
	}
}

func TestDoctorNoCredential(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	t.Setenv("OPUSCLIP_BASE_URL", srv.URL)
	io, _, out, _ := iostreams.Test()
	// Empty store → no-credential branch in doctor.
	f := &cmdutil.Factory{IOStreams: io, Flags: &cmdutil.GlobalFlags{}, ConfigPath: t.TempDir() + "/c.toml", CredStore: auth.NewMemoryStore()}
	cmd := root.NewCmdRoot(f)
	cmd.SetArgs([]string{"doctor"})
	cmd.SetOut(out)
	cmd.SetErr(out)
	_ = cmd.Execute()
	if !strings.Contains(out.String(), "credential stored") {
		t.Errorf("doctor output: %s", out.String())
	}
}
