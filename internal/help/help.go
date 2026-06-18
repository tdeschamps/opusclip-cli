// Package help renders the branded landing screen shown for a bare `opusclip`
// invocation and inside `opusclip info`: a framed ASCII wordmark with a tagline
// and version, followed by a grouped command listing. It is chrome — on a
// non-TTY (piped) or with color disabled it degrades to clean plain text.
package help

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/tdeschamps/opusclip-cli/internal/iostreams"
	"github.com/tdeschamps/opusclip-cli/internal/text"
)

//go:embed wordmark.txt
var wordmark string

// tagline is the one-line product pitch shown under the wordmark.
const tagline = "Turn long videos into short, ready-to-post clips — from the terminal."

// Command is one row in the command listing: an invocation and what it does.
type Command struct {
	Invocation string
	Summary    string
}

// Group is a titled set of commands.
type Group struct {
	Title    string
	Commands []Command
}

// Screen is the data the landing screen renders.
type Screen struct {
	Version string
	Groups  []Group
	Footer  string
}

// boxWidth is the inner width of the dashed frame (chars between the borders).
const boxWidth = 70

// RenderBanner writes just the framed wordmark + tagline + version box to
// stdout (no command listing). Used by `info`, which adds its own status rows.
func RenderBanner(io *iostreams.IOStreams, version string) {
	out := io.Out
	top := "┌" + strings.Repeat("╌", boxWidth) + "┐"
	bottom := "└" + strings.Repeat("╌", boxWidth) + "┘"
	fmt.Fprintln(out, io.Gray(top))
	fmt.Fprintln(out, boxRow(io, ""))

	for _, line := range strings.Split(strings.TrimRight(wordmark, "\n"), "\n") {
		fmt.Fprintln(out, boxRow(io, io.Cyan(line)))
	}

	fmt.Fprintln(out, boxRow(io, ""))
	for _, line := range wrap(tagline, boxWidth-4) {
		fmt.Fprintln(out, boxRow(io, io.Bold(line)))
	}
	if version != "" {
		fmt.Fprintln(out, boxRow(io, io.Gray(version)))
	}
	fmt.Fprintln(out, boxRow(io, ""))
	fmt.Fprintln(out, io.Gray(bottom))
}

// Render writes the banner box + command groups to the IOStreams' stdout.
func (sc Screen) Render(io *iostreams.IOStreams) {
	out := io.Out
	RenderBanner(io, sc.Version)
	fmt.Fprintln(out)

	// --- command groups ---
	for _, g := range sc.Groups {
		fmt.Fprintln(out, io.Bold(g.Title))
		for _, c := range g.Commands {
			// Pad the (uncolored) invocation to a fixed column, then colorize, so
			// ANSI escape bytes never throw off the alignment.
			pad := commandColWidth - text.DisplayWidth(c.Invocation)
			if pad < 1 {
				pad = 1
			}
			fmt.Fprintf(out, "  %s%s%s\n", io.Cyan(c.Invocation), strings.Repeat(" ", pad), c.Summary)
		}
		fmt.Fprintln(out)
	}

	if sc.Footer != "" {
		fmt.Fprintln(out, io.Gray(sc.Footer))
	}
}

// commandColWidth is the column at which command summaries start.
const commandColWidth = 26

// boxRow centers content within the dashed frame. content may contain ANSI
// escapes; padding is computed from its visible length.
func boxRow(io *iostreams.IOStreams, content string) string {
	visible := text.DisplayWidth(content)
	if visible > boxWidth {
		visible = boxWidth
	}
	left := (boxWidth - visible) / 2
	right := boxWidth - visible - left
	border := io.Gray("╎")
	return fmt.Sprintf("%s%s%s%s%s", border, strings.Repeat(" ", left), content, strings.Repeat(" ", right), border)
}

// wrap breaks s into lines no wider than width, on word boundaries.
func wrap(s string, width int) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	cur := words[0]
	for _, w := range words[1:] {
		if len(cur)+1+len(w) > width {
			lines = append(lines, cur)
			cur = w
			continue
		}
		cur += " " + w
	}
	return append(lines, cur)
}
