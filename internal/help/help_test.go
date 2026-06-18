package help

import (
	"strings"
	"testing"

	"github.com/tdeschamps/opusclip-cli/internal/iostreams"
	"github.com/tdeschamps/opusclip-cli/internal/text"
)

func TestScreenRenderPlain(t *testing.T) {
	io, _, out, _ := iostreams.Test() // color off by default
	OpusClip("v1.0.0").Render(io)
	s := out.String()

	for _, want := range []string{"█", "Commands", "opusclip clip create", "v1.0.0", tagline[:10]} {
		if !strings.Contains(s, want) {
			t.Errorf("screen missing %q:\n%s", want, s)
		}
	}
	// Plain (no color) output must carry no ANSI escapes.
	if strings.Contains(s, "\x1b[") {
		t.Errorf("plain screen should have no ANSI escapes:\n%q", s)
	}
}

func TestBannerBoxAligned(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	RenderBanner(io, "dev")
	for _, line := range strings.Split(strings.TrimRight(out.String(), "\n"), "\n") {
		if w := text.DisplayWidth(line); w != boxWidth+2 {
			t.Errorf("box row width = %d, want %d: %q", w, boxWidth+2, line)
		}
	}
}

func TestWrap(t *testing.T) {
	got := wrap("one two three four", 8)
	if len(got) < 2 {
		t.Errorf("expected wrapping, got %v", got)
	}
	for _, line := range got {
		if len(line) > 8 && !strings.Contains(line, " ") {
			continue // a single over-long word is allowed to overflow
		}
		if len(line) > 8 {
			t.Errorf("line exceeds width: %q", line)
		}
	}
	if wrap("", 8)[0] != "" {
		t.Error("empty input should yield one empty line")
	}
}
