package ui

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// dimFactor controls background dimming (0 = black, 1 = original brightness).
const dimFactor = 0.35

// ansiSGRRe matches any ANSI SGR escape sequence: ESC [ <params> m
var ansiSGRRe = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

// darkenSGR receives a full SGR escape sequence and returns a copy with any
// 24-bit color components (both foreground 38;2;r;g;b and background 48;2;r;g;b)
// multiplied by dimFactor. Non-RGB sequences are returned unchanged.
func darkenSGR(seq string) string {
	inner := seq[2 : len(seq)-1] // strip ESC[ and m
	parts := strings.Split(inner, ";")

	i := 0
	for i < len(parts) {
		// Detect 24-bit color intro: "38" (fg) or "48" (bg) followed by "2" + R;G;B
		if (parts[i] == "38" || parts[i] == "48") && i+4 < len(parts) && parts[i+1] == "2" {
			r, errR := strconv.Atoi(parts[i+2])
			g, errG := strconv.Atoi(parts[i+3])
			b, errB := strconv.Atoi(parts[i+4])
			if errR == nil && errG == nil && errB == nil {
				parts[i+2] = strconv.Itoa(int(float64(r) * dimFactor))
				parts[i+3] = strconv.Itoa(int(float64(g) * dimFactor))
				parts[i+4] = strconv.Itoa(int(float64(b) * dimFactor))
			}
			i += 5
			continue
		}
		i++
	}
	return "\x1b[" + strings.Join(parts, ";") + "m"
}

// dimLine darkens all 24-bit foreground and background colors in s and also
// applies ANSI faint (SGR 2) as a fallback for any non-24-bit foreground colors.
func dimLine(s string) string {
	// Translate full SGR resets so the dim wrapper survives them mid-line.
	s = strings.ReplaceAll(s, "\x1b[0m", "\x1b[0;2m")
	s = strings.ReplaceAll(s, "\x1b[m", "\x1b[0;2m")

	// Darken 24-bit RGB values directly in every SGR sequence (covers both fg and bg).
	s = ansiSGRRe.ReplaceAllStringFunc(s, darkenSGR)

	// Wrap with ANSI faint as a fallback for non-24-bit foreground colors.
	return "\x1b[2m" + s + "\x1b[0m"
}

// placeOverlayBottomRight composites fg over bg at the bottom-right corner,
// leaving the rest of the background undimmed. margin controls the gap from
// the edges (in terminal cells). bgW and bgH are the terminal dimensions.
func placeOverlayBottomRight(bg, fg string, bgW, bgH int) string {
	const margin = 2

	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")

	fgH := len(fgLines)
	fgW := 0
	for _, l := range fgLines {
		if w := lipgloss.Width(l); w > fgW {
			fgW = w
		}
	}

	startY := bgH - fgH - margin
	startX := bgW - fgW - margin
	if startX < 0 {
		startX = 0
	}

	for i, bgLine := range bgLines {
		fgIdx := i - startY
		if fgIdx < 0 || fgIdx >= fgH {
			continue
		}
		fgLine := fgLines[fgIdx]
		left := ansi.Truncate(bgLine, startX, "")
		right := ansi.TruncateLeft(bgLine, startX+fgW, "")
		bgLines[i] = left + fgLine + right
	}

	return strings.Join(bgLines, "\n")
}

// placeOverlayCenter composites fg centred over bg, dimming all background
// content. bgW and bgH are the terminal's visual column and row counts.
func placeOverlayCenter(bg, fg string, bgW, bgH int) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")

	fgH := len(fgLines)
	fgW := 0
	for _, l := range fgLines {
		if w := lipgloss.Width(l); w > fgW {
			fgW = w
		}
	}

	startY := (bgH - fgH) / 2
	startX := (bgW - fgW) / 2
	if startX < 0 {
		startX = 0
	}

	for i, bgLine := range bgLines {
		fgIdx := i - startY
		if fgIdx < 0 || fgIdx >= fgH {
			// Row is entirely behind the background — dim the whole line.
			bgLines[i] = dimLine(bgLine)
			continue
		}

		// Row intersects the modal — dim the left and right flanking portions
		// and leave the modal's own line untouched (full brightness).
		fgLine := fgLines[fgIdx]
		left := dimLine(ansi.Truncate(bgLine, startX, ""))
		right := dimLine(ansi.TruncateLeft(bgLine, startX+fgW, ""))
		bgLines[i] = left + fgLine + right
	}

	return strings.Join(bgLines, "\n")
}
