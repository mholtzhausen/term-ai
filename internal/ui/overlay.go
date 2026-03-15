package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// dimLine applies ANSI dim to a string, re-applying dim after any SGR full
// resets so that existing colors are preserved but visually darkened.
func dimLine(s string) string {
	// Replace full SGR resets with reset+dim so lipgloss colour sequences
	// don't inadvertently cancel the dim attribute mid-line.
	s = strings.ReplaceAll(s, "\x1b[0m", "\x1b[0;2m")
	s = strings.ReplaceAll(s, "\x1b[m", "\x1b[0;2m")
	return "\x1b[2m" + s + "\x1b[0m"
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
