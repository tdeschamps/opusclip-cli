package clip

import (
	"strings"
	"testing"

	"github.com/tdeschamps/opusclip-cli/internal/api"
	"github.com/tdeschamps/opusclip-cli/internal/iostreams"
)

func TestProjectFields(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	fields := projectFields(io)
	p := api.Project{
		ProjectID:      "P1",
		Stage:          api.StageComplete,
		SourcePlatform: "youtube",
		CreatedAt:      "2026-06-17",
	}
	cells := map[string]string{}
	for _, f := range fields {
		cells[f.Name] = f.Extract(p)
	}
	if cells["PROJECT"] != "P1" || cells["STAGE"] != "COMPLETE" {
		t.Errorf("cells = %+v", cells)
	}
	if cells["SOURCE"] != "youtube" || cells["CREATED"] != "2026-06-17" {
		t.Errorf("cells = %+v", cells)
	}
	if cells["ERROR"] != "-" {
		t.Errorf("non-stalled error cell = %q", cells["ERROR"])
	}
}

func TestStageCellColors(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	io.SetColorEnabled(true)
	// Just assert the stage text is present for each branch.
	for _, s := range []api.Stage{api.StageComplete, api.StageStalled, api.StageRender} {
		if got := stageCell(io, s); !strings.Contains(got, string(s)) {
			t.Errorf("stageCell(%q) = %q", s, got)
		}
	}
}

func TestErrorCellStalled(t *testing.T) {
	p := api.Project{Stage: api.StageStalled, Error: "boom"}
	if got := errorCell(p); got != "boom" {
		t.Errorf("stalled error cell = %q", got)
	}
	if got := errorCell(api.Project{Stage: api.StageStalled}); got != "-" {
		t.Errorf("stalled-no-detail cell = %q", got)
	}
}

func TestStageIsTerminal(t *testing.T) {
	if !api.StageComplete.IsTerminal() || !api.StageStalled.IsTerminal() {
		t.Error("complete/stalled should be terminal")
	}
	if api.StageRender.IsTerminal() {
		t.Error("render is not terminal")
	}
}
