package clips

import (
	"testing"

	"github.com/tdeschamps/opusclip-cli/internal/api"
)

func TestScoreCell(t *testing.T) {
	score := 87.0
	vir := 70.0
	if got := scoreCell(api.ExportableClip{Score: &score}); got != "87" {
		t.Errorf("score cell = %q", got)
	}
	if got := scoreCell(api.ExportableClip{ViralityScore: &vir}); got != "70" {
		t.Errorf("virality cell = %q", got)
	}
	if got := scoreCell(api.ExportableClip{}); got != "-" {
		t.Errorf("absent score = %q", got)
	}
}

func TestClipFilename(t *testing.T) {
	cases := []struct {
		clip api.ExportableClip
		want string
	}{
		{api.ExportableClip{ID: "P1.c1", Title: "Best Moment!"}, "Best-Moment.mp4"},
		{api.ExportableClip{ID: "P1.c2", Title: ""}, "P1.c2.mp4"},
		{api.ExportableClip{ID: "P1.c3", Title: "  ///  "}, "P1.c3.mp4"},
		{api.ExportableClip{ID: "P1.c4", Title: "Uber Clip 2026"}, "Uber-Clip-2026.mp4"},
	}
	for _, tc := range cases {
		if got := clipFilename(tc.clip); got != tc.want {
			t.Errorf("clipFilename(%q) = %q, want %q", tc.clip.Title, got, tc.want)
		}
	}
}

func TestSelectClips(t *testing.T) {
	items := []api.ExportableClip{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	if got := selectClips(items, ""); len(got) != 3 {
		t.Errorf("empty filter should return all, got %d", len(got))
	}
	if got := selectClips(items, "b"); len(got) != 1 || got[0].ID != "b" {
		t.Errorf("filter b = %+v", got)
	}
	if got := selectClips(items, "missing"); got != nil {
		t.Errorf("missing filter should return nil, got %+v", got)
	}
}
