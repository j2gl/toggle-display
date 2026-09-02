package completion

import (
	"strings"
	"testing"
)

func TestGenerateZsh(t *testing.T) {
	script, err := Generate(" zsh ")
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"#compdef toggle-display", "_arguments", "_describe", "--complete", "--config"} {
		if !strings.Contains(script, fragment) {
			t.Fatalf("generated zsh script lacks %q", fragment)
		}
	}
	if _, err := Generate("bash"); err == nil {
		t.Fatal("unsupported shell was accepted")
	}
}
