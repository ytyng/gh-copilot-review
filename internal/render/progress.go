package render

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mattn/go-isatty"
)

// Progress prints one-line status updates to stderr while `wait` polls.
// On a TTY, updates rewrite the current line with a carriage return; on
// non-TTY output (CI logs, redirects) each update is a fresh line so the
// progress stays scannable.
type Progress struct {
	w       io.Writer
	isTTY   bool
	started time.Time
}

func NewProgress() *Progress {
	return &Progress{
		w:       os.Stderr,
		isTTY:   isatty.IsTerminal(os.Stderr.Fd()),
		started: time.Now(),
	}
}

// Update renders the current state. Safe to call repeatedly.
func (p *Progress) Update(state string) {
	elapsed := time.Since(p.started).Round(time.Second)
	if p.isTTY {
		fmt.Fprintf(p.w, "\r[%s] state=%s                    ", formatDuration(elapsed), state)
		return
	}
	fmt.Fprintf(p.w, "[%s] state=%s\n", formatDuration(elapsed), state)
}

// Done clears the progress line (TTY only) so subsequent output doesn't
// collide with the last rewrite.
func (p *Progress) Done() {
	if p.isTTY {
		fmt.Fprint(p.w, "\r\x1b[2K") // CR + clear line
	}
}

func formatDuration(d time.Duration) string {
	m := int(d.Minutes())
	s := int(d.Seconds()) - m*60
	return fmt.Sprintf("%02d:%02d", m, s)
}
