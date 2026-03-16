package tools

import (
	"fmt"
	"os"
	"strings"
)

// ConfirmFunc is called before executing sensitive operations.
// Return true to proceed, false to abort.
// Override this to integrate with a different UI (e.g. TUI overlay).
var ConfirmFunc = defaultConfirm

func defaultConfirm(prompt string) bool {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		// Can't open tty — deny by default for safety.
		return false
	}
	defer tty.Close()

	fmt.Fprintf(tty, "\n⚠  %s [y/N]: ", prompt)

	buf := make([]byte, 64)
	n, _ := tty.Read(buf)
	resp := strings.TrimSpace(strings.ToLower(string(buf[:n])))
	return resp == "y" || resp == "yes"
}
