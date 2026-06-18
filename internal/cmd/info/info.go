// Package info implements `opusclip info` — a friendly banner showing the CLI
// version, the active configuration, and auth status. On a terminal it prints
// the OpusClip logo plus a readable summary; piped or with --json it emits a plain
// structured object, so it stays scriptable.
package info

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/tdeschamps/opusclip-cli/internal/auth"
	"github.com/tdeschamps/opusclip-cli/internal/cmd/version"
	"github.com/tdeschamps/opusclip-cli/internal/cmdutil"
	"github.com/tdeschamps/opusclip-cli/internal/config"
	"github.com/tdeschamps/opusclip-cli/internal/iostreams"
)

const docsURL = "https://github.com/tdeschamps/opusclip-cli#readme"

// data is the structured form of `info`, used for json/piped output.
type data struct {
	Version          string `json:"version"`
	Commit           string `json:"commit"`
	BuildDate        string `json:"buildDate"`
	Profile          string `json:"profile"`
	Workspace        string `json:"workspace,omitempty"`
	BaseURL          string `json:"baseUrl"`
	ConfigPath       string `json:"configPath"`
	Authenticated    bool   `json:"authenticated"`
	AuthMethod       string `json:"authMethod,omitempty"`
	TokenFingerprint string `json:"tokenFingerprint,omitempty"`
	RESTReachable    *bool  `json:"restReachable,omitempty"`
}

// NewCmdInfo returns the info command.
func NewCmdInfo(f *cmdutil.Factory) *cobra.Command {
	var check bool
	cmd := &cobra.Command{
		Use:     "info",
		Short:   "Show the CLI version, configuration, and status",
		GroupID: "config",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := gather(f)
			if err != nil {
				return err
			}

			if check {
				rest := probe(cmd.Context(), f)
				d.RESTReachable = &rest
			}

			if handled, err := cmdutil.PrintStructured(f, d); handled || err != nil {
				return err
			}
			renderHuman(f.IOStreams, d)
			return nil
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "Also probe REST connectivity")
	return cmd
}

func gather(f *cmdutil.Factory) (data, error) {
	r, err := f.Resolver()
	if err != nil {
		return data{}, err
	}
	profile, _ := f.ActiveProfile()
	cfgPath, _ := config.DefaultPath()
	if f.ConfigPath != "" {
		cfgPath = f.ConfigPath
	}

	d := data{
		Version:    version.Version,
		Commit:     version.Commit,
		BuildDate:  version.Date,
		Profile:    profile,
		BaseURL:    r.Resolve("base_url", "", cmdutil.OSLookup),
		ConfigPath: cfgPath,
		Workspace:  r.Resolve("workspace", "", cmdutil.OSLookup),
	}

	if store, serr := f.CredentialStore(); serr == nil {
		if cred, cerr := store.Get(profile); cerr == nil {
			d.Authenticated = true
			d.AuthMethod = cred.Method
			d.TokenFingerprint = auth.Fingerprint(cred.Token)
			if d.Workspace == "" {
				d.Workspace = cred.Workspace
			}
		}
	}
	return d, nil
}

// probe checks REST reachability, showing a spinner while it waits.
func probe(ctx context.Context, f *cmdutil.Factory) (rest bool) {
	sp := f.IOStreams.NewSpinner("Checking connectivity…")
	sp.Start()
	defer sp.Stop()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if c, err := f.APIClient(); err == nil {
		rest = c.Validate(ctx) == nil
	}
	return rest
}

func renderHuman(io *iostreams.IOStreams, d data) {
	out := io.Out
	fmt.Fprintln(out, io.Bold("opusclip"), io.Gray(d.Version))
	fmt.Fprintln(out)

	// Pad the label to a fixed width *before* colorizing so the ANSI escape
	// bytes don't throw off the alignment.
	row := func(label, value string) {
		fmt.Fprintf(out, "  %s %s\n", io.Bold(fmt.Sprintf("%-7s", label)), value)
	}
	row("Version", fmt.Sprintf("%s (%s)", d.Version, d.Commit))
	if d.Workspace != "" {
		row("Profile", fmt.Sprintf("%s (%s)", d.Profile, d.Workspace))
	} else {
		row("Profile", d.Profile)
	}
	if d.Authenticated {
		row("Auth", fmt.Sprintf("%s %s (%s)", io.SuccessIcon(), d.TokenFingerprint, d.AuthMethod))
	} else {
		row("Auth", fmt.Sprintf("%s not logged in", io.ErrorIcon()))
	}
	row("REST", d.BaseURL)
	row("Config", d.ConfigPath)

	if d.RESTReachable != nil {
		row("Check", fmt.Sprintf("REST %s", io.BoolIcon(*d.RESTReachable)))
	}

	fmt.Fprintln(out)
	if d.Authenticated {
		fmt.Fprintf(out, "  %s Try %s or %s\n", io.Gray("›"), io.Cyan("opusclip clip create --url <video> --wait"), io.Cyan("opusclip clips list --project <id>"))
	} else {
		fmt.Fprintf(out, "  %s Run %s to get started\n", io.Gray("›"), io.Cyan("opusclip auth login"))
	}
	fmt.Fprintf(out, "  %s Docs: %s\n", io.Gray("›"), io.Hyperlink(docsURL, docsURL))
}
