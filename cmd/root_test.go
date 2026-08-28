package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestDispatchWithoutArgs(t *testing.T) {
	err := dispatch(nil)

	if err == nil {
		t.Fatal("want an error when no command is given")
	}
	if !strings.Contains(err.Error(), "no command specified") {
		t.Errorf("err = %v", err)
	}
}

func TestDispatchHelp(t *testing.T) {
	for _, arg := range []string{"-h", "--help", "help"} {
		if err := dispatch([]string{arg}); err != nil {
			t.Errorf("dispatch(%q) = %v, want nil", arg, err)
		}
	}
}

func TestDispatchUnknownCommand(t *testing.T) {
	err := dispatch([]string{"nope"})

	if err == nil {
		t.Fatal("want an error for an unknown command")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("err = %v, want it to name the unknown command", err)
	}
}

func TestReportError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantCode    int
		wantMessage string
	}{
		{
			// `wait` のタイムアウトが 124 で返るのは、この経路が生きているから。
			// 呼び出し側のスクリプトが 1 と 124 を区別できなくなると壊れる。
			name:        "ExitError carries its own code",
			err:         &ExitError{Code: 124, Err: errors.New("timed out")},
			wantCode:    124,
			wantMessage: "timed out",
		},
		{
			name:     "ExitError without a message prints nothing",
			err:      &ExitError{Code: 0},
			wantCode: 0,
		},
		{
			name:        "plain errors become exit code 1",
			err:         errors.New("boom"),
			wantCode:    1,
			wantMessage: "boom",
		},
		{
			name:        "a wrapped ExitError is still detected",
			err:         fmt.Errorf("while waiting: %w", &ExitError{Code: 124, Err: errors.New("timed out")}),
			wantCode:    124,
			wantMessage: "timed out",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			if got := reportError(&buf, tt.err); got != tt.wantCode {
				t.Errorf("reportError = %d, want %d", got, tt.wantCode)
			}
			if tt.wantMessage == "" {
				if buf.Len() != 0 {
					t.Errorf("output = %q, want nothing", buf.String())
				}
				return
			}
			if !strings.Contains(buf.String(), tt.wantMessage) {
				t.Errorf("output = %q, want it to contain %q", buf.String(), tt.wantMessage)
			}
		})
	}
}

func TestRunUnknownCommandExitsWithOne(t *testing.T) {
	if code := Run([]string{"nope"}); code != 1 {
		t.Errorf("Run = %d, want 1", code)
	}
}

func TestRunHelpExitsWithZero(t *testing.T) {
	if code := Run([]string{"--help"}); code != 0 {
		t.Errorf("Run = %d, want 0", code)
	}
}

func TestExitErrorMessage(t *testing.T) {
	t.Run("with a wrapped error", func(t *testing.T) {
		inner := errors.New("boom")
		e := &ExitError{Code: 2, Err: inner}

		if e.Error() != "boom" {
			t.Errorf("Error() = %q", e.Error())
		}
		if !errors.Is(e, inner) {
			t.Error("errors.Is should unwrap to the inner error")
		}
	})

	t.Run("without an error", func(t *testing.T) {
		e := &ExitError{Code: 0}

		if e.Error() != "exit code 0" {
			t.Errorf("Error() = %q", e.Error())
		}
		if errors.Unwrap(e) != nil {
			t.Error("Unwrap should be nil")
		}
	})
}

// 引数の検証は API を叩く前に終わること。
// ここが崩れると、明らかに壊れた引数でも GitHub へ要求が飛ぶ。
func TestArgumentValidationHappensBeforeAnyAPICall(t *testing.T) {
	tests := []struct {
		name string
		run  func([]string) error
		args []string
		want string
	}{
		{"request without a PR number", RunRequest, nil, "expected 1 positional argument"},
		{"request with too many arguments", RunRequest, []string{"1", "2"}, "expected 1 positional argument"},
		{"request with a non-numeric PR", RunRequest, []string{"abc"}, "parse PR number"},
		{"wait without a PR number", RunWait, nil, "expected 1 positional argument"},
		{"wait with a non-numeric PR", RunWait, []string{"abc"}, "parse PR number"},
		{"wait with a zero interval", RunWait, []string{"--interval", "0s", "1"}, "--interval must be positive"},
		{"wait with a negative interval", RunWait, []string{"--interval", "-1s", "1"}, "--interval must be positive"},
		{"wait with a zero timeout", RunWait, []string{"--timeout", "0s", "1"}, "--timeout must be positive"},
		{"status without a PR number", RunStatus, nil, "expected 1 positional argument"},
		{"status with a non-numeric PR", RunStatus, []string{"abc"}, "parse PR number"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(tt.args)

			if err == nil {
				t.Fatalf("want an error containing %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestHelpFlagReturnsExitZero(t *testing.T) {
	for _, run := range []func([]string) error{RunRequest, RunWait, RunStatus} {
		err := run([]string{"-h"})

		var exit *ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("want an ExitError, got %v", err)
		}
		if exit.Code != 0 {
			t.Errorf("code = %d, want 0", exit.Code)
		}
	}
}

func TestUsageMentionsEverySubcommand(t *testing.T) {
	for _, name := range []string{"request", "wait", "status"} {
		if !strings.Contains(rootUsage, name) {
			t.Errorf("root usage does not mention %q", name)
		}
	}
	// wait の終了コードは呼び出し側 (スクリプト) が分岐に使うので、
	// usage に書いてあることを保つ
	if !strings.Contains(waitUsage, "124") {
		t.Error("wait usage should document exit code 124")
	}
}
