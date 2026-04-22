package render

import (
	"fmt"
	"io"
	"os"
	"time"
)

// Progress prints one-line status updates to stderr while `wait` polls.
// Each Update writes a fresh line — no carriage-return rewriting — so the
// output scrolls naturally both on a TTY and in redirected logs.
type Progress struct {
	w       io.Writer
	started time.Time
}

func NewProgress() *Progress {
	return &Progress{
		w:       os.Stderr,
		started: time.Now(),
	}
}

// Update renders the current state on its own line.
func (p *Progress) Update(state string) {
	elapsed := time.Since(p.started).Round(time.Second)
	fmt.Fprintf(p.w, "[%s] state=%s\n", formatDuration(elapsed), state)
}

func formatDuration(d time.Duration) string {
	m := int(d.Minutes())
	s := int(d.Seconds()) - m*60
	return fmt.Sprintf("%02d:%02d", m, s)
}
