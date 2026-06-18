package clip

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/tdeschamps/opusclip-cli/internal/cmdutil"
)

func newWatchCmd(f *cmdutil.Factory) *cobra.Command {
	var interval, timeout time.Duration
	var exitStatus bool
	cmd := &cobra.Command{
		Use:   "watch <projectId>",
		Short: "Poll a clip project until it completes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			return waitForProject(cmd.Context(), f, client, args[0], watchOpts{
				interval:   interval,
				timeout:    timeout,
				exitStatus: exitStatus,
			})
		},
	}
	cmd.Flags().DurationVar(&interval, "interval", defaultPollInterval, "Poll interval")
	cmd.Flags().DurationVar(&timeout, "timeout", defaultPollTimeout, "Give up after this duration (0 = no limit)")
	cmd.Flags().BoolVar(&exitStatus, "exit-status", false, "Set exit code by outcome and suppress the banner")
	return cmd
}
