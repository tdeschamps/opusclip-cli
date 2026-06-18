package clip

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/tdeschamps/opusclip-cli/internal/api"
	"github.com/tdeschamps/opusclip-cli/internal/cmdutil"
	"github.com/tdeschamps/opusclip-cli/internal/iostreams"
	"github.com/tdeschamps/opusclip-cli/internal/text"
)

func TestBuildCreateInput(t *testing.T) {
	fl := &createFlags{
		url:           "https://youtu.be/x",
		webhook:       "https://hook",
		email:         "me@x.com",
		notifyFailure: true,
		genre:         "podcast",
		lang:          "en",
		keywords:      []string{"ai", "go"},
		durations:     []string{"0:60", "30:90"},
		start:         10,
		end:           120,
		brandTemplate: "bt-1",
	}
	in, err := buildCreateInput(fl)
	if err != nil {
		t.Fatal(err)
	}
	if in.VideoURL != "https://youtu.be/x" || in.BrandTemplateID != "bt-1" {
		t.Errorf("in = %+v", in)
	}
	if len(in.ConclusionActions) != 2 {
		t.Fatalf("conclusion actions = %+v", in.ConclusionActions)
	}
	if in.ConclusionActions[0].Type != "WEBHOOK" || !in.ConclusionActions[0].NotifyFailure {
		t.Errorf("webhook action = %+v", in.ConclusionActions[0])
	}
	if in.CurationPref == nil || !reflect.DeepEqual(in.CurationPref.ClipDurations, [][]int{{0, 60}, {30, 90}}) {
		t.Errorf("durations = %+v", in.CurationPref)
	}
	if in.CurationPref.Range == nil || in.CurationPref.Range.StartSec != 10 {
		t.Errorf("range = %+v", in.CurationPref.Range)
	}
	if in.ImportPref == nil || in.ImportPref.SourceLang != "en" {
		t.Errorf("importPref = %+v", in.ImportPref)
	}
}

func TestBuildCreateInputMinimal(t *testing.T) {
	in, err := buildCreateInput(&createFlags{url: "https://x"})
	if err != nil {
		t.Fatal(err)
	}
	if in.CurationPref != nil || in.ImportPref != nil || in.ConclusionActions != nil {
		t.Errorf("minimal input should omit optional prefs: %+v", in)
	}
}

func TestBuildCreateInputErrors(t *testing.T) {
	if _, err := buildCreateInput(&createFlags{}); err == nil {
		t.Error("missing url should error")
	}
	if _, err := buildCreateInput(&createFlags{url: "x", durations: []string{"bad"}}); err == nil {
		t.Error("bad duration should error")
	}
	if _, err := buildCreateInput(&createFlags{url: "x", durations: []string{"90:10"}}); err == nil {
		t.Error("inverted duration range should error")
	}
}

func TestStepperString(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	for _, s := range append(api.StageOrder, api.StageStalled, api.Stage("WAT")) {
		if got := stepperString(io, s); got == "" {
			t.Errorf("stepper(%q) empty", s)
		}
	}
}

// fakeGetter scripts a sequence of stages for the poller.
type fakeGetter struct {
	stages []api.Stage
	errs   []error
	i      int
}

func (f *fakeGetter) GetProject(_ context.Context, id string) (api.Project, error) {
	idx := f.i
	if idx >= len(f.stages) {
		idx = len(f.stages) - 1
	}
	f.i++
	var err error
	if idx < len(f.errs) {
		err = f.errs[idx]
	}
	p := api.Project{ProjectID: id, Stage: f.stages[idx]}
	if f.stages[idx] == api.StageStalled {
		p.Error = "boom"
	}
	return p, err
}

func testFactory(t *testing.T) *cmdutil.Factory {
	io, _, _, _ := iostreams.Test()
	return &cmdutil.Factory{
		IOStreams: io,
		Flags:     &cmdutil.GlobalFlags{},
		Clock:     text.FixedClock(time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC)),
	}
}

func withInstantTicks(t *testing.T) {
	t.Helper()
	orig := after
	after = func(time.Duration) <-chan time.Time {
		ch := make(chan time.Time, 1)
		ch <- time.Time{}
		return ch
	}
	t.Cleanup(func() { after = orig })
}

func TestWaitForProjectCompletes(t *testing.T) {
	withInstantTicks(t)
	g := &fakeGetter{stages: []api.Stage{api.StageImport, api.StageCurate, api.StageRender, api.StageComplete}}
	if err := waitForProject(context.Background(), testFactory(t), g, "P1", watchOpts{interval: time.Millisecond}); err != nil {
		t.Fatalf("complete should be nil err: %v", err)
	}
}

func TestWaitForProjectStalls(t *testing.T) {
	withInstantTicks(t)
	g := &fakeGetter{stages: []api.Stage{api.StageImport, api.StageStalled}}
	err := waitForProject(context.Background(), testFactory(t), g, "P1", watchOpts{interval: time.Millisecond})
	if err == nil {
		t.Fatal("stall should error")
	}
	if cmdutil.ExitCodeForError(err) != cmdutil.ExitUpstream {
		t.Errorf("stall exit code = %d, want %d", cmdutil.ExitCodeForError(err), cmdutil.ExitUpstream)
	}
}

func TestWaitForProjectGetError(t *testing.T) {
	withInstantTicks(t)
	g := &fakeGetter{stages: []api.Stage{api.StageImport}, errs: []error{errors.New("net down")}}
	if err := waitForProject(context.Background(), testFactory(t), g, "P1", watchOpts{interval: time.Millisecond}); err == nil {
		t.Fatal("get error should propagate")
	}
}

func TestWaitForProjectContextCancel(t *testing.T) {
	// Never reaches a terminal stage; a cancelled context ends the loop.
	orig := after
	after = func(time.Duration) <-chan time.Time { return make(chan time.Time) } // never ticks
	t.Cleanup(func() { after = orig })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	g := &fakeGetter{stages: []api.Stage{api.StageImport}}
	err := waitForProject(ctx, testFactory(t), g, "P1", watchOpts{interval: time.Hour})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("want context.Canceled, got %v", err)
	}
}
