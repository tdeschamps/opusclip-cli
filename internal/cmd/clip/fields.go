package clip

import (
	"github.com/tdeschamps/opusclip-cli/internal/api"
	"github.com/tdeschamps/opusclip-cli/internal/iostreams"
	"github.com/tdeschamps/opusclip-cli/internal/output"
	"github.com/tdeschamps/opusclip-cli/internal/text"
)

// projectFields describes the table columns for a Project.
func projectFields(io *iostreams.IOStreams) []output.Field {
	return []output.Field{
		{Name: "PROJECT", Extract: func(v any) string { return v.(api.Project).ProjectID }},
		{Name: "STAGE", Extract: func(v any) string { return stageCell(io, v.(api.Project).Stage) }},
		{Name: "SOURCE", Extract: func(v any) string { return text.OrDash(v.(api.Project).SourcePlatform) }},
		{Name: "CREATED", Extract: func(v any) string { return text.OrDash(v.(api.Project).CreatedAt) }},
		{Name: "ERROR", Extract: func(v any) string { return errorCell(v.(api.Project)) }},
	}
}

// stageCell renders a stage, colorized: green for COMPLETE, red for STALLED,
// cyan for everything in progress.
func stageCell(io *iostreams.IOStreams, s api.Stage) string {
	switch s {
	case api.StageComplete:
		return io.Green(string(s))
	case api.StageStalled:
		return io.Red(string(s))
	default:
		return io.Cyan(string(s))
	}
}

// errorCell shows the project error only for the failure state.
func errorCell(p api.Project) string {
	if p.Stage == api.StageStalled && p.Error != "" {
		return p.Error
	}
	return "-"
}
