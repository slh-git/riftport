// clipboard.go — copy text to the system clipboard via common CLI tools.
package cmd

import (
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
)

// writeClipboard copies text to the system clipboard.
func writeClipboard(text string) error {
	switch runtime.GOOS {
	case "darwin":
		return pipeToClipboard(text, exec.Command("pbcopy"))
	case "linux":
		for _, cmd := range []*exec.Cmd{
			exec.Command("wl-copy"),
			exec.Command("xclip", "-selection", "clipboard"),
			exec.Command("xsel", "--clipboard", "--input"),
		} {
			if err := pipeToClipboard(text, cmd); err == nil {
				return nil
			}
		}
		return fmt.Errorf("clipboard unavailable: install wl-copy, xclip, or xsel")
	case "windows":
		return pipeToClipboard(text, exec.Command("clip"))
	default:
		return fmt.Errorf("clipboard not supported on %s", runtime.GOOS)
	}
}

func pipeToClipboard(text string, cmd *exec.Cmd) error {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	if _, err := io.WriteString(stdin, text); err != nil {
		_ = stdin.Close()
		_ = cmd.Wait()
		return err
	}
	if err := stdin.Close(); err != nil {
		_ = cmd.Wait()
		return err
	}
	return cmd.Wait()
}

func clipboardToolName() string {
	switch runtime.GOOS {
	case "darwin":
		return "pbcopy"
	case "windows":
		return "clip"
	default:
		return "wl-copy, xclip, or xsel"
	}
}

func finishInteractiveHint() string {
	if runtime.GOOS == "windows" {
		return "Ctrl+Z then Enter"
	}
	return "Ctrl+D"
}

func trimInput(text string) string {
	return strings.TrimSpace(text)
}
