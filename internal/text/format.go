package text

import (
	"fmt"
)

// OrDash returns s, or "-" when s is empty. Used for table/detail cells that
// shouldn't render a blank.
func OrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// Duration renders a millisecond count as mm:ss (or hh:mm:ss past an hour).
func Duration(ms float64) string {
	total := int(ms / 1000)
	h, m, s := total/3600, (total%3600)/60, total%60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// HumanBytes renders a byte count with a binary (1024-based) unit suffix.
func HumanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n/div >= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// DisplayWidth returns the visible width of s, ignoring ANSI escape sequences
// (both CSI color codes and OSC 8 hyperlinks) so padding/centering math isn't
// thrown off by escape bytes.
func DisplayWidth(s string) int {
	n := 0
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] != 0x1b { // not ESC → a visible rune
			n++
			continue
		}
		// Skip an escape sequence.
		i++
		if i >= len(runes) {
			break
		}
		switch runes[i] {
		case '[': // CSI: ESC [ ... <final byte 0x40–0x7e>
			for i++; i < len(runes); i++ {
				if runes[i] >= 0x40 && runes[i] <= 0x7e {
					break
				}
			}
		case ']': // OSC: ESC ] ... terminated by BEL or ESC \
			for i++; i < len(runes); i++ {
				if runes[i] == 0x07 {
					break
				}
				if runes[i] == 0x1b && i+1 < len(runes) && runes[i+1] == '\\' {
					i++
					break
				}
			}
		}
	}
	return n
}
