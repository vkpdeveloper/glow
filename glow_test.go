package main

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/glamour/styles"
	"github.com/muesli/reflow/ansi"
	"github.com/spf13/cobra"
)

func TestGlowFlags(t *testing.T) {
	tt := []struct {
		args  []string
		check func() bool
	}{
		{
			args: []string{"-p"},
			check: func() bool {
				return pager
			},
		},
		{
			args: []string{"-s", "light"},
			check: func() bool {
				return style == "light"
			},
		},
		{
			args: []string{"-w", "40"},
			check: func() bool {
				return width == 40
			},
		},
	}

	for _, v := range tt {
		err := rootCmd.ParseFlags(v.args)
		if err != nil {
			t.Fatal(err)
		}
		if !v.check() {
			t.Errorf("Parsing flag failed: %s", v.args)
		}
	}
}

func TestWidthFlagDefault(t *testing.T) {
	flag := rootCmd.Flags().Lookup("width")
	if flag == nil {
		t.Fatal("width flag not registered")
	}

	if flag.DefValue != "150" {
		t.Fatalf("expected width flag default to be 150, got %s", flag.DefValue)
	}
}

func TestExecuteCLIUsesConfiguredWrapWidth(t *testing.T) {
	previousWidth := width
	previousStyle := style
	previousPager := pager
	previousTUI := tui
	previousPreserveNewLines := preserveNewLines

	t.Cleanup(func() {
		width = previousWidth
		style = previousStyle
		pager = previousPager
		tui = previousTUI
		preserveNewLines = previousPreserveNewLines
	})

	width = 150
	style = styles.NoTTYStyle
	pager = false
	tui = false
	preserveNewLines = false

	src := &source{
		reader: io.NopCloser(strings.NewReader(strings.Repeat("abcdefghij ", 40))),
		URL:    "/tmp/test.md",
	}

	cmd := &cobra.Command{}
	var out bytes.Buffer
	if err := executeCLI(cmd, src, &out); err != nil {
		t.Fatalf("executeCLI returned error: %v", err)
	}

	maxWidth := 0
	for _, line := range strings.Split(out.String(), "\n") {
		maxWidth = max(maxWidth, ansi.PrintableRuneWidth(line))
	}

	if maxWidth > 150 {
		t.Fatalf("expected CLI render width <= 150, got %d", maxWidth)
	}
	if maxWidth < 130 {
		t.Fatalf("expected CLI render to use a width near 150, got max line width %d", maxWidth)
	}
}
