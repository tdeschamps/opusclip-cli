// Package clip implements `opusclip clip`: create, get, and watch clip projects.
package clip

import (
	"github.com/spf13/cobra"

	"github.com/tdeschamps/opusclip-cli/internal/cmdutil"
)

// NewCmdClip returns the clip command group.
func NewCmdClip(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "clip <command>",
		Short:   "Create, inspect, and watch clip projects",
		GroupID: "core",
	}
	cmd.AddCommand(
		newCreateCmd(f),
		newGetCmd(f),
		newWatchCmd(f),
	)
	return cmd
}
