// Package doctor implements `opusclip doctor` — connectivity and credential
// diagnostics (mirrors `sentry-cli info` / `gh status`).
package doctor

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/tdeschamps/opusclip-cli/internal/cmd/version"
	"github.com/tdeschamps/opusclip-cli/internal/cmdutil"
	"github.com/tdeschamps/opusclip-cli/internal/config"
)

// NewCmdDoctor returns the doctor command.
func NewCmdDoctor(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:     "doctor",
		Short:   "Check connectivity, credentials, and configuration",
		GroupID: "config",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			io := f.IOStreams

			fmt.Fprintf(io.Out, "%s %s\n", io.Bold("opusclip"), version.String())

			cfgPath, _ := config.DefaultPath()
			fmt.Fprintf(io.Out, "%s config path: %s\n", io.InfoIcon(), cfgPath)

			profile, _ := f.ActiveProfile()
			fmt.Fprintf(io.Out, "%s active profile: %s\n", io.InfoIcon(), profile)

			r, err := f.Resolver()
			if err != nil {
				return err
			}
			baseURL := r.Resolve("base_url", "", cmdutil.OSLookup)

			// Credential present?
			store, err := f.CredentialStore()
			hasCred := false
			if err == nil {
				if _, gerr := store.Get(profile); gerr == nil {
					hasCred = true
				}
			}
			fmt.Fprintf(io.Out, "%s credential stored: %s\n", io.BoolIcon(hasCred), boolText(hasCred))

			// Probe both endpoints behind a spinner (no-op when piped/--quiet).
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()
			sp := io.NewSpinner("Probing endpoints…")
			sp.Start()

			// REST reachability + credential check (Validate probes the API).
			client, err := f.APIClient()
			restOK := false
			var restErr error
			if err == nil {
				if restErr = client.Validate(ctx); restErr == nil {
					restOK = true
				}
			}
			sp.Stop()

			fmt.Fprintf(io.Out, "%s REST %s\n", io.BoolIcon(restOK), baseURL)
			if restErr != nil {
				fmt.Fprintf(io.Out, "    %s\n", io.Gray(restErr.Error()))
			}

			if !restOK {
				return cmdutil.NewSilentError(cmdutil.ExitError, fmt.Errorf("REST endpoint unreachable or unauthenticated"))
			}
			return nil
		},
	}
}

func boolText(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
