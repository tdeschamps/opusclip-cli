package clip

import (
	"github.com/spf13/cobra"

	"github.com/tdeschamps/opusclip-cli/internal/cmdutil"
)

func newGetCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "get <projectId>...",
		Short: "Get one or more clip projects by ID",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			return cmdutil.GetAndRender(cmd.Context(), f, args, client.GetProject, projectFields(f.IOStreams))
		},
	}
}
