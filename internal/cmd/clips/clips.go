// Package clips implements `opusclip clips`: list and download a project's
// generated clips.
package clips

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tdeschamps/opusclip-cli/internal/api"
	"github.com/tdeschamps/opusclip-cli/internal/cmdutil"
	"github.com/tdeschamps/opusclip-cli/internal/iostreams"
	"github.com/tdeschamps/opusclip-cli/internal/output"
	"github.com/tdeschamps/opusclip-cli/internal/text"
)

// NewCmdClips returns the clips command group.
func NewCmdClips(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "clips <command>",
		Short:   "List and download generated clips",
		GroupID: "core",
	}
	cmd.AddCommand(
		newListCmd(f),
		newDownloadCmd(f),
	)
	return cmd
}

// clipFields describes the table columns for an ExportableClip.
func clipFields(io *iostreams.IOStreams) []output.Field {
	return []output.Field{
		{Name: "ID", Extract: func(v any) string { return v.(api.ExportableClip).ID }},
		{Name: "TITLE", Extract: func(v any) string { return io.Bold(v.(api.ExportableClip).Title) }},
		{Name: "DURATION", Extract: func(v any) string { return text.Duration(v.(api.ExportableClip).DurationMs) }},
		{Name: "GENRE", Extract: func(v any) string { return text.OrDash(v.(api.ExportableClip).Genre) }},
		{Name: "SCORE", Extract: func(v any) string { return scoreCell(v.(api.ExportableClip)) }},
		{Name: "CREATED", Extract: func(v any) string { return text.OrDash(v.(api.ExportableClip).CreatedAt) }},
	}
}

// scoreCell shows the (unofficial) virality/judge score when present.
func scoreCell(c api.ExportableClip) string {
	switch {
	case c.Score != nil:
		return fmt.Sprintf("%.0f", *c.Score)
	case c.ViralityScore != nil:
		return fmt.Sprintf("%.0f", *c.ViralityScore)
	default:
		return "-"
	}
}
