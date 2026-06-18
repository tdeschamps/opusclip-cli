package clip

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tdeschamps/opusclip-cli/internal/api"
	"github.com/tdeschamps/opusclip-cli/internal/cmdutil"
	"github.com/tdeschamps/opusclip-cli/internal/iostreams"
)

const (
	// defaultPollInterval keeps polling under the 30 req/min API budget (~10/min).
	defaultPollInterval = 6 * time.Second
	// defaultPollTimeout bounds how long we wait for a project to finish.
	defaultPollTimeout = 30 * time.Minute
)

// watchOpts configures a poll loop.
type watchOpts struct {
	interval   time.Duration
	timeout    time.Duration
	exitStatus bool
}

// projectGetter is the slice of the API client the poller needs (injected so
// tests can script stage transitions without a server).
type projectGetter interface {
	GetProject(ctx context.Context, projectID string) (api.Project, error)
}

// after is the tick seam, overridable in tests to avoid real sleeps.
var after = time.After

// waitForProject polls a project until it reaches a terminal stage. It renders a
// stage stepper on stderr (when interactive) and returns:
//   - nil on COMPLETE,
//   - a SilentError(ExitUpstream) on STALLED,
//   - the context error on cancel/deadline (deadline → exit 124).
func waitForProject(ctx context.Context, f *cmdutil.Factory, client projectGetter, projectID string, opt watchOpts) error {
	if opt.interval <= 0 {
		opt.interval = defaultPollInterval
	}
	if opt.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opt.timeout)
		defer cancel()
	}

	io := f.IOStreams
	sp := io.NewSpinner("Waiting for project " + projectID + "…")
	sp.Start()
	defer sp.Stop()

	var lastStage api.Stage
	for {
		p, err := client.GetProject(ctx, projectID)
		if err != nil {
			sp.Stop()
			return err
		}

		if p.Stage != lastStage {
			lastStage = p.Stage
			sp.Update(stepperString(io, p.Stage))
			// On a non-interactive stderr the spinner is silent; emit one terse
			// line per stage change so logs/CI stay readable but not noisy.
			if !io.ProgressEnabled() && !opt.exitStatus {
				io.Errf("%s %s\n", projectID, p.Stage)
			}
		}

		switch p.Stage {
		case api.StageComplete:
			sp.Stop()
			if !opt.exitStatus {
				renderComplete(io, projectID)
			}
			return nil
		case api.StageStalled:
			sp.Stop()
			msg := p.Error
			if msg == "" {
				msg = "no error detail provided"
			}
			return cmdutil.NewSilentError(cmdutil.ExitUpstream,
				fmt.Errorf("project %s stalled: %s", projectID, msg))
		}

		select {
		case <-ctx.Done():
			sp.Stop()
			return ctx.Err()
		case <-after(opt.interval):
		}
	}
}

// stepperString renders the happy-path stages with the current one highlighted:
// done stages get a check, the current a dot, pending stages stay dim.
func stepperString(io *iostreams.IOStreams, current api.Stage) string {
	cur := current.Index()
	parts := make([]string, 0, len(api.StageOrder))
	for i, st := range api.StageOrder {
		label := string(st)
		switch {
		case cur >= 0 && i < cur:
			parts = append(parts, io.Green("✓"+label))
		case i == cur:
			parts = append(parts, io.Cyan("●"+label))
		default:
			parts = append(parts, io.Gray(label))
		}
	}
	return strings.Join(parts, " › ")
}

func renderComplete(io *iostreams.IOStreams, projectID string) {
	io.RenderBanner(iostreams.Banner{
		Kind:     iostreams.BannerSuccess,
		Headline: "Project " + projectID + " is ready",
		Body:     "Clips have finished rendering.",
		NextSteps: []string{
			"opusclip clips list --project " + projectID,
			"opusclip clips download --project " + projectID,
		},
	})
}
