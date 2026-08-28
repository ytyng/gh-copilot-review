package render

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "00:00"},
		{5 * time.Second, "00:05"},
		{59 * time.Second, "00:59"},
		{time.Minute, "01:00"},
		{90 * time.Second, "01:30"},
		{10 * time.Minute, "10:00"},
		// 1 時間を超えても分で数え続ける (途中経過の表示なので桁溢れさせない)
		{75 * time.Minute, "75:00"},
	}

	for _, tt := range tests {
		if got := formatDuration(tt.d); got != tt.want {
			t.Errorf("formatDuration(%s) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestProgressUpdateWritesOneLinePerCall(t *testing.T) {
	var buf bytes.Buffer
	p := &Progress{w: &buf, started: time.Now()}

	p.Update("reviewing")
	p.Update("completed")

	// 経過時間は実時間なので、桁の書式と状態だけを見る
	// (formatDuration の値そのものは TestFormatDuration で固定している)。
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d (%q), want 2", len(lines), buf.String())
	}
	for i, want := range []string{"reviewing", "completed"} {
		re := regexp.MustCompile(`^\[\d{2}:\d{2}\] state=` + want + `$`)
		if !re.MatchString(lines[i]) {
			t.Errorf("line %d = %q, want it to match %s", i, lines[i], re)
		}
	}
}

func TestNewProgressStartsNow(t *testing.T) {
	p := NewProgress()

	if time.Since(p.started) > time.Second {
		t.Errorf("started = %s, want ~now", p.started)
	}
	if p.w == nil {
		t.Error("writer is nil")
	}
}
