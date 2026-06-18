// Package auth implements the `opusclip auth` command group: login (API key),
// logout, status, switch, and token. OpusClip authenticates with an API key
// only (no OAuth), so login simply validates and stores the key.
package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/tdeschamps/opusclip-cli/internal/api"
	"github.com/tdeschamps/opusclip-cli/internal/auth"
	"github.com/tdeschamps/opusclip-cli/internal/cmdutil"
	"github.com/tdeschamps/opusclip-cli/internal/config"
	"github.com/tdeschamps/opusclip-cli/internal/httpclient"
	"github.com/tdeschamps/opusclip-cli/internal/iostreams"
	"github.com/tdeschamps/opusclip-cli/internal/text"
)

// NewCmdAuth returns the auth command group.
func NewCmdAuth(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "auth <command>",
		Short:   "Authenticate opusclip and manage credentials",
		GroupID: "config",
	}
	cmd.AddCommand(
		newLoginCmd(f),
		newLogoutCmd(f),
		newStatusCmd(f),
		newSwitchCmd(f),
		newTokenCmd(f),
	)
	return cmd
}

func newLoginCmd(f *cmdutil.Factory) *cobra.Command {
	var withToken, web, skipValidation bool
	var org string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with OpusClip using an API key",
		Long: `Authenticate with OpusClip using an API key from the dashboard. With
--with-token the key is read from stdin, which is CI-friendly:

  echo $OPUSCLIP_API_KEY | opusclip auth login --with-token`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			io := f.IOStreams
			key, err := readKey(f, withToken, web)
			if err != nil {
				return err
			}
			if key == "" {
				return cmdutil.NewUsageError(fmt.Errorf("no API key provided"))
			}

			profileName, err := f.ActiveProfile()
			if err != nil {
				return err
			}

			if !skipValidation {
				warn, hardErr := validateKey(cmd.Context(), f, key, org)
				if hardErr != nil {
					return hardErr
				}
				if warn != "" {
					io.Errf("%s %s\n", io.WarnIcon(), warn)
				}
			}

			store, err := f.CredentialStore()
			if err != nil {
				return err
			}
			if err := store.Set(profileName, auth.Credential{Token: key, Method: auth.MethodAPIKey}); err != nil {
				return err
			}

			// Record the auth method (and org) on the profile.
			if cfg, _ := f.Config(); cfg != nil {
				p := cfg.ProfileOrDefault(profileName)
				p.AuthMethod = auth.MethodAPIKey
				if org != "" {
					p.OrgID = org
				}
				_ = f.SaveConfig(cfg)
			}

			io.RenderBanner(iostreams.Banner{
				Kind:     iostreams.BannerSuccess,
				Headline: fmt.Sprintf("Logged in to profile %q", profileName),
				Body:     fmt.Sprintf("Key %s stored.", auth.Fingerprint(key)),
				NextSteps: []string{
					"opusclip clip create --url <video> --wait",
					"opusclip clips download --project <id>",
					"opusclip doctor — check connectivity",
				},
			})
			return nil
		},
	}
	cmd.Flags().BoolVar(&withToken, "with-token", false, "Read the API key from stdin")
	cmd.Flags().BoolVar(&web, "web", false, "Open the dashboard in a browser")
	cmd.Flags().StringVar(&org, "org", "", "Organization ID (x-opus-org-id) to store for this profile")
	cmd.Flags().BoolVar(&skipValidation, "skip-validation", false, "Store the key without validating it")
	return cmd
}

// readKey obtains the API key from stdin (--with-token) or an interactive prompt.
func readKey(f *cmdutil.Factory, withToken, web bool) (string, error) {
	io := f.IOStreams
	if withToken {
		b, err := io.ReadAllStdin()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(b)), nil
	}
	if web {
		io.Errf("Opening %s\n", config.DashboardURL)
		_ = cmdutil.OpenBrowser(config.DashboardURL)
	} else {
		io.Errf("Create or copy an API key at %s\n", config.DashboardURL)
	}
	if !io.CanPrompt() {
		return "", fmt.Errorf("cannot prompt for a key in a non-interactive session; use --with-token")
	}
	k, err := cmdutil.PromptSecret(io, "Paste your OpusClip API key: ")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(k), nil
}

// validateKey probes the API with the candidate key.
//   - A genuine 401 returns a hard error (login aborts; nothing is stored).
//   - A network/5xx/uncertain error returns a non-empty warning and nil error:
//     the key is stored anyway so CI and flaky networks never block login.
//   - Success returns ("", nil).
func validateKey(ctx context.Context, f *cmdutil.Factory, key, org string) (warn string, hardErr error) {
	client := api.New(api.Options{
		BaseURL:    mustResolve(f, "base_url"),
		OrgID:      org,
		HTTPClient: validationClient(f, key),
	})
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	sp := f.IOStreams.NewSpinner("Validating API key…")
	sp.Start()
	err := client.Validate(ctx)
	sp.Stop()
	if err == nil {
		return "", nil
	}
	var apiErr *api.Error
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusUnauthorized {
		return "", cmdutil.NewSilentError(cmdutil.ExitAuth,
			fmt.Errorf("the API rejected this key (401 Unauthorized); nothing was stored"))
	}
	return "could not verify the key against the API; stored anyway — run `opusclip doctor` to check", nil
}

func newLogoutCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove credentials for the active profile",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName, err := f.ActiveProfile()
			if err != nil {
				return err
			}
			store, err := f.CredentialStore()
			if err != nil {
				return err
			}
			if err := store.Delete(profileName); err != nil {
				if errors.Is(err, auth.ErrNotFound) {
					f.IOStreams.Errf("No credentials stored for profile %q\n", profileName)
					return nil
				}
				return err
			}
			f.IOStreams.Errf("%s Logged out of profile %q\n", f.IOStreams.SuccessIcon(), profileName)
			return nil
		},
	}
}

func newStatusCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the active profile and stored credential",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName, err := f.ActiveProfile()
			if err != nil {
				return err
			}
			store, err := f.CredentialStore()
			if err != nil {
				return err
			}
			cred, err := store.Get(profileName)
			if err != nil {
				return cmdutil.NewSilentError(cmdutil.ExitAuth,
					fmt.Errorf("not logged in to profile %q (run `opusclip auth login`)", profileName))
			}
			io := f.IOStreams
			io.Errf("%s Profile %s\n", io.SuccessIcon(), io.Bold(profileName))
			fmt.Fprintf(io.Out, "  Token:   %s\n", auth.Fingerprint(cred.Token))
			fmt.Fprintf(io.Out, "  Method:  %s\n", text.OrDash(cred.Method))
			return nil
		},
	}
}

func newSwitchCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "switch <profile>",
		Short: "Change the active profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := f.Config()
			if err != nil {
				return err
			}
			cfg.ProfileOrDefault(args[0])
			cfg.ActiveProfile = args[0]
			if err := f.SaveConfig(cfg); err != nil {
				return err
			}
			f.IOStreams.Errf("%s Active profile is now %q\n", f.IOStreams.SuccessIcon(), args[0])
			return nil
		},
	}
}

func newTokenCmd(f *cmdutil.Factory) *cobra.Command {
	var confirm bool
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Print the current API key (guarded)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !confirm {
				return cmdutil.NewUsageError(fmt.Errorf(
					"printing the key exposes full-workspace access; re-run with --confirm"))
			}
			tok, err := f.TokenSource()()
			if err != nil {
				return err
			}
			fmt.Fprintln(f.IOStreams.Out, tok)
			return nil
		},
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Confirm exposing the full key")
	return cmd
}

// --- helpers ---

func mustResolve(f *cmdutil.Factory, key string) string {
	r, err := f.Resolver()
	if err != nil {
		return config.DefaultBaseURL
	}
	return r.Resolve(key, "", cmdutil.OSLookup)
}

// validationClient builds an http.Client that injects the candidate key so we
// can validate it before persisting.
func validationClient(f *cmdutil.Factory, key string) *http.Client {
	return httpclient.New(httpclient.Options{
		Token:      func() (string, error) { return key, nil },
		MaxRetries: 1,
		Insecure:   f.Flags.Insecure,
	})
}
