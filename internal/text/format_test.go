package text

import "testing"

func TestOrDash(t *testing.T) {
	if OrDash("") != "-" {
		t.Error("empty should be dash")
	}
	if OrDash("x") != "x" {
		t.Error("non-empty should pass through")
	}
}

func TestDuration(t *testing.T) {
	cases := map[float64]string{
		0:       "00:00",
		30000:   "00:30",
		90000:   "01:30",
		3661000: "01:01:01",
	}
	for ms, want := range cases {
		if got := Duration(ms); got != want {
			t.Errorf("Duration(%v) = %q, want %q", ms, got, want)
		}
	}
}

func TestHumanBytes(t *testing.T) {
	cases := map[int64]string{
		512:     "512 B",
		2048:    "2.0 KB",
		5242880: "5.0 MB",
	}
	for n, want := range cases {
		if got := HumanBytes(n); got != want {
			t.Errorf("HumanBytes(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestDisplayWidth(t *testing.T) {
	if got := DisplayWidth("plain"); got != 5 {
		t.Errorf("plain = %d", got)
	}
	if got := DisplayWidth("\x1b[36mhi\x1b[0m"); got != 2 {
		t.Errorf("CSI color = %d, want 2", got)
	}
	// OSC 8 hyperlink wrapping "X" → visible width 1.
	link := "\x1b]8;;https://example.com\x07X\x1b]8;;\x07"
	if got := DisplayWidth(link); got != 1 {
		t.Errorf("OSC 8 hyperlink = %d, want 1", got)
	}
}
