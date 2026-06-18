package clip

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/tdeschamps/opusclip-cli/internal/api"
	"github.com/tdeschamps/opusclip-cli/internal/cmdutil"
)

// createFlags holds the flags for `clip create`.
type createFlags struct {
	url           string
	webhook       string
	email         string
	notifyFailure bool
	genre         string
	lang          string
	keywords      []string
	durations     []string // "min:max" pairs
	start         float64
	end           float64
	brandTemplate string
	wait          bool
	interval      time.Duration
	timeout       time.Duration
	exitStatus    bool
}

func newCreateCmd(f *cmdutil.Factory) *cobra.Command {
	fl := &createFlags{}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Submit a video URL for clipping",
		Long: `Submit a long-form video URL to OpusClip for clipping.

  opusclip clip create --url https://youtube.com/watch?v=...
  opusclip clip create --url <video> --wait        # block until clips are ready
  opusclip clip create --url <video> --webhook https://example.com/hook`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			in, err := buildCreateInput(fl)
			if err != nil {
				return err
			}
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			project, err := client.CreateProject(cmd.Context(), in)
			if err != nil {
				return err
			}
			if fl.wait {
				return waitForProject(cmd.Context(), f, client, project.ProjectID, watchOpts{
					interval:   fl.interval,
					timeout:    fl.timeout,
					exitStatus: fl.exitStatus,
				})
			}
			return cmdutil.RenderSlice(f, []api.Project{project}, projectFields(f.IOStreams))
		},
	}
	pf := cmd.Flags()
	pf.StringVar(&fl.url, "url", "", "Source video URL (required)")
	pf.StringVar(&fl.webhook, "webhook", "", "Webhook URL to call on completion")
	pf.StringVar(&fl.email, "email", "", "Email to notify on completion")
	pf.BoolVar(&fl.notifyFailure, "notify-failure", false, "Also notify on failure (webhook/email)")
	pf.StringVar(&fl.genre, "genre", "", "Curation genre hint")
	pf.StringVar(&fl.lang, "lang", "", "Source spoken language (ISO-639, e.g. en); default auto")
	pf.StringSliceVar(&fl.keywords, "keywords", nil, "Topic keywords to focus on (repeatable/CSV)")
	pf.StringArrayVar(&fl.durations, "duration", nil, "Clip duration range in seconds as min:max (repeatable)")
	pf.Float64Var(&fl.start, "start", 0, "Only curate from this second of the source")
	pf.Float64Var(&fl.end, "end", 0, "Only curate up to this second of the source")
	pf.StringVar(&fl.brandTemplate, "brand-template", "", "Brand template ID to apply")
	pf.BoolVar(&fl.wait, "wait", false, "Poll until the project completes, then exit")
	pf.DurationVar(&fl.interval, "interval", defaultPollInterval, "Poll interval when --wait is set")
	pf.DurationVar(&fl.timeout, "timeout", defaultPollTimeout, "Give up waiting after this duration (0 = no limit)")
	pf.BoolVar(&fl.exitStatus, "exit-status", false, "With --wait, set exit code by outcome and suppress the banner")
	_ = cmd.MarkFlagRequired("url")
	return cmd
}

// buildCreateInput turns flags into a CreateProjectInput. It is pure (no I/O) so
// it can be unit-tested exhaustively.
func buildCreateInput(fl *createFlags) (api.CreateProjectInput, error) {
	in := api.CreateProjectInput{
		VideoURL:        strings.TrimSpace(fl.url),
		BrandTemplateID: fl.brandTemplate,
	}
	if in.VideoURL == "" {
		return in, cmdutil.NewUsageError(fmt.Errorf("--url is required"))
	}

	in.ConclusionActions = conclusionActions(fl)

	curation := api.CurationPreference{Genre: fl.genre, TopicKeywords: fl.keywords}
	durations, err := parseDurations(fl.durations)
	if err != nil {
		return in, err
	}
	curation.ClipDurations = durations
	if fl.start != 0 || fl.end != 0 {
		curation.Range = &api.CurationRange{StartSec: fl.start, EndSec: fl.end}
	}
	if !curationEmpty(curation) {
		in.CurationPref = &curation
	}

	if fl.lang != "" {
		in.ImportPref = &api.ImportPreference{SourceLang: fl.lang}
	}
	return in, nil
}

func conclusionActions(fl *createFlags) []api.ConclusionAction {
	var out []api.ConclusionAction
	if fl.webhook != "" {
		out = append(out, api.ConclusionAction{Type: "WEBHOOK", URL: fl.webhook, NotifyFailure: fl.notifyFailure})
	}
	if fl.email != "" {
		out = append(out, api.ConclusionAction{Type: "EMAIL", Email: fl.email, NotifyFailure: fl.notifyFailure})
	}
	return out
}

// parseDurations parses "min:max" second pairs into [[min,max],…].
func parseDurations(pairs []string) ([][]int, error) {
	if len(pairs) == 0 {
		return nil, nil
	}
	out := make([][]int, 0, len(pairs))
	for _, p := range pairs {
		minStr, maxStr, ok := strings.Cut(p, ":")
		if !ok {
			return nil, cmdutil.NewUsageError(fmt.Errorf("--duration must be min:max, got %q", p))
		}
		min, err := strconv.Atoi(strings.TrimSpace(minStr))
		if err != nil {
			return nil, cmdutil.NewUsageError(fmt.Errorf("--duration min must be an integer, got %q", minStr))
		}
		max, err := strconv.Atoi(strings.TrimSpace(maxStr))
		if err != nil {
			return nil, cmdutil.NewUsageError(fmt.Errorf("--duration max must be an integer, got %q", maxStr))
		}
		if min < 0 || max < min {
			return nil, cmdutil.NewUsageError(fmt.Errorf("--duration range invalid: %q (need 0 <= min <= max)", p))
		}
		out = append(out, []int{min, max})
	}
	return out, nil
}

func curationEmpty(c api.CurationPreference) bool {
	return c.Genre == "" && len(c.TopicKeywords) == 0 && len(c.ClipDurations) == 0 && c.Range == nil
}
