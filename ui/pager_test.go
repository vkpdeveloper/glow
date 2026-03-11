package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/glamour/styles"
	"github.com/muesli/reflow/ansi"
)

func TestGlamourRenderFitsViewportWidth(t *testing.T) {
	previousConfig := config
	t.Cleanup(func() {
		config = previousConfig
	})

	config = Config{GlamourEnabled: true}

	m := pagerModel{
		common: &commonModel{
			cfg: Config{
				GlamourEnabled:  true,
				GlamourMaxWidth: 80,
				GlamourStyle:    styles.DarkStyle,
			},
		},
		currentDocument: markdown{
			Note: "architecture.md",
		},
	}
	m.viewport.Width = 80

	out, err := glamourRender(m, "Design a photo-to-calories system that performs materially better than the current single-pass meal analyzer, fits the existing Ledger app/server architecture, and creates the data foundation required to keep improving accuracy.")
	if err != nil {
		t.Fatalf("glamourRender returned error: %v", err)
	}

	for _, line := range strings.Split(out, "\n") {
		if got := ansi.PrintableRuneWidth(line); got > m.viewport.Width {
			t.Fatalf("rendered line width %d exceeds viewport width %d: %q", got, m.viewport.Width, line)
		}
	}
}

func TestGlamourRenderUsesConfiguredMaxWidth(t *testing.T) {
	previousConfig := config
	t.Cleanup(func() {
		config = previousConfig
	})

	config = Config{GlamourEnabled: true}

	m := pagerModel{
		common: &commonModel{
			cfg: Config{
				GlamourEnabled:  true,
				GlamourMaxWidth: 150,
				GlamourStyle:    styles.DarkStyle,
			},
		},
		currentDocument: markdown{
			Note: "architecture.md",
		},
	}
	m.viewport.Width = 200

	out, err := glamourRender(m, strings.Repeat("abcdefghij ", 40))
	if err != nil {
		t.Fatalf("glamourRender returned error: %v", err)
	}

	maxWidth := 0
	for _, line := range strings.Split(out, "\n") {
		maxWidth = max(maxWidth, ansi.PrintableRuneWidth(line))
	}

	if maxWidth > 150 {
		t.Fatalf("expected TUI render width <= 150, got %d", maxWidth)
	}
	if maxWidth < 130 {
		t.Fatalf("expected TUI render to use a width near 150, got max line width %d", maxWidth)
	}
}
