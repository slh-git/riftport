package cmd

import (
	"strings"
	"testing"
)

func TestTrimInput(t *testing.T) {
	if got := trimInput("  hello\n"); got != "hello" {
		t.Fatalf("trimInput() = %q, want %q", got, "hello")
	}
	if got := trimInput("\n\n"); got != "" {
		t.Fatalf("trimInput() = %q, want empty", got)
	}
}

func TestFinishInteractiveHint(t *testing.T) {
	if !strings.Contains(finishInteractiveHint(), "Ctrl") {
		t.Fatalf("finishInteractiveHint() = %q", finishInteractiveHint())
	}
}
