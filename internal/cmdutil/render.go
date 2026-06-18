package cmdutil

import (
	"context"
	"fmt"
	"iter"

	"github.com/tdeschamps/opusclip-cli/internal/output"
)

// CollectAndRender drains a paginating iterator into a slice, then renders it
// through the factory's Printer using the supplied table fields. noun names the
// resource ("deals", "calls", …) for the progress indicator. When --all is set
// and progress is enabled (interactive stderr, not --quiet/--hide-spinner), a
// spinner reports a live count on stderr; it never touches stdout. It returns
// the first error encountered.
func CollectAndRender[T any](ctx context.Context, f *Factory, seq iter.Seq2[T, error], fields []output.Field, noun string) error {
	items := make([]T, 0)

	// The spinner is a no-op unless started, and Start itself is gated on an
	// interactive stderr — so this only animates for a real `--all` sweep.
	sp := f.IOStreams.NewSpinner("Fetching " + noun + "…")
	if f.Flags.All {
		sp.Start()
	}

	for item, err := range seq {
		if err != nil {
			sp.Stop()
			return err
		}
		items = append(items, item)
		sp.Update(fmt.Sprintf("Fetched %d %s…", len(items), noun))
	}
	sp.Stop()
	return RenderSlice(f, items, fields)
}

// GetAndRender fetches one object per id (in order), then renders the lot. It
// centralizes the "get one-or-more IDs → render" flow shared by every resource
// command's `get` subcommand.
func GetAndRender[T any](ctx context.Context, f *Factory, args []string, get func(context.Context, string) (T, error), fields []output.Field) error {
	items := make([]T, 0, len(args))
	for _, id := range args {
		item, err := get(ctx, id)
		if err != nil {
			return err
		}
		items = append(items, item)
	}
	return RenderSlice(f, items, fields)
}

// RenderSlice renders an already-collected slice of items.
func RenderSlice[T any](f *Factory, items []T, fields []output.Field) error {
	p, err := f.Printer()
	if err != nil {
		return err
	}
	return output.Render(p, items, fields)
}

// PrintStructured handles the machine-output path for commands that have a
// bespoke human layout. When the effective format is non-interactive (json/csv/
// yaml or a piped stdout) it prints v as JSON and returns handled=true; when the
// format is the interactive table it returns handled=false so the caller renders
// its human view. It centralizes the "JSON when piped, else human" dispatch
// shared by info and clips download.
func PrintStructured(f *Factory, v any) (handled bool, err error) {
	format, err := f.OutputFormat()
	if err != nil {
		return false, err
	}
	if format.IsInteractive() {
		return false, nil
	}
	p, err := f.Printer()
	if err != nil {
		return true, err
	}
	return true, p.PrintJSON(v)
}
