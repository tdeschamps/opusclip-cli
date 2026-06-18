package clips

import (
	"github.com/spf13/cobra"

	"github.com/tdeschamps/opusclip-cli/internal/cmdutil"
)

func newListCmd(f *cmdutil.Factory) *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List a project's generated clips",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			limit, err := f.EffectiveLimit()
			if err != nil {
				return err
			}
			items, err := client.ListExportableClips(cmd.Context(), project, limit)
			if err != nil {
				return err
			}
			return cmdutil.RenderSlice(f, items, clipFields(f.IOStreams))
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "Project ID (required)")
	_ = cmd.MarkFlagRequired("project")
	return cmd
}
