package mermaid

import (
	"strings"
	"testing"

	mermaidascii "github.com/AlexanderGrooff/mermaid-ascii/cmd"
	diagramconfig "github.com/AlexanderGrooff/mermaid-ascii/pkg/diagram"
)

func TestRenderMarkdownReplacesMermaidFence(t *testing.T) {
	input := strings.Join([]string{
		"# Diagram",
		"",
		"```mermaid",
		"graph LR",
		"A --> B",
		"```",
		"",
		"After",
	}, "\n")

	output := RenderMarkdown(input, Options{})

	if strings.Contains(output, "```mermaid") {
		t.Fatalf("expected mermaid fence to be replaced, got %q", output)
	}
	if !strings.Contains(output, "┌") || !strings.Contains(output, "►") {
		t.Fatalf("expected rendered diagram output, got %q", output)
	}
	if !strings.Contains(output, "# Diagram") || !strings.Contains(output, "After") {
		t.Fatalf("expected surrounding markdown to be preserved, got %q", output)
	}
}

func TestRenderMarkdownPreservesUnsupportedMermaid(t *testing.T) {
	input := strings.Join([]string{
		"```mermaid",
		"classDiagram",
		"Animal <|-- Duck",
		"```",
	}, "\n")

	output := RenderMarkdown(input, Options{})

	if output != input {
		t.Fatalf("expected unsupported diagram to remain unchanged, got %q", output)
	}
}

func TestRenderMarkdownKeepsIndentedFenceStructure(t *testing.T) {
	input := strings.Join([]string{
		"- Item",
		"  ```mermaid",
		"  sequenceDiagram",
		"  Alice->>Bob: Hello",
		"  ```",
	}, "\n")

	output := RenderMarkdown(input, Options{})

	if !strings.Contains(output, "  ```") {
		t.Fatalf("expected rendered block to preserve indentation, got %q", output)
	}
	if !strings.Contains(output, "Alice") || !strings.Contains(output, "Bob") {
		t.Fatalf("expected rendered sequence diagram, got %q", output)
	}
}

func TestNormalizeMermaidFlowchartSyntax(t *testing.T) {
	input := strings.Join([]string{
		"flowchart TD",
		`A["Flutter capture flow"] --> B{No data?}`,
		`B -->|No| C[Retry capture]`,
		`B -->|Yes| D[Process frame]`,
		`D --> E[Send to API]`,
	}, "\n")

	output := normalizeMermaid(input)

	for _, unwanted := range []string{`A["`, "B{", "C[", "D[", "E["} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("expected normalized output to remove Mermaid node syntax, got %q", output)
		}
	}
	for _, wanted := range []string{
		"Flutter capture flow --> No data?",
		"No data? -->|No| Retry capture",
		"No data? -->|Yes| Process frame",
		"Process frame --> Send to API",
	} {
		if !strings.Contains(output, wanted) {
			t.Fatalf("expected normalized output to contain %q, got %q", wanted, output)
		}
	}
}

func TestRenderMarkdownHandlesCommonFlowchartLabels(t *testing.T) {
	input := strings.Join([]string{
		"```mermaid",
		"flowchart TD",
		`A["Flutter capture flow"] --> B{No data?}`,
		`B -->|No| C[Retry capture]`,
		`B -->|Yes| D[Process frame]`,
		`D --> E[Send to API]`,
		"```",
	}, "\n")

	output := RenderMarkdown(input, Options{})

	for _, bad := range []string{`A["Flutter capture flow"]`, "B{No data?}", "C[Retry capture]"} {
		if strings.Contains(output, bad) {
			t.Fatalf("expected rendered diagram to avoid raw Mermaid node syntax, got %q", output)
		}
	}
	for _, wanted := range []string{"Flutter capture flow", "No data?", "Retry capture", "Process frame", "Send to API"} {
		if !strings.Contains(output, wanted) {
			t.Fatalf("expected rendered diagram to contain %q, got %q", wanted, output)
		}
	}
}

func TestRenderDiagramChoosesMoreCompactLayoutThanDefault(t *testing.T) {
	source := strings.Join([]string{
		"flowchart TD",
		`A["Flutter capture flow"] --> B{No data?}`,
		`B -->|No| C[Retry capture]`,
		`B -->|Yes| D[Process frame]`,
		`D --> E[Send to API]`,
	}, "\n")

	defaultConfig := diagramconfig.DefaultConfig()
	defaultConfig.StyleType = "cli"
	defaultRendered, err := mermaidascii.RenderDiagram(normalizeMermaid(source), defaultConfig)
	if err != nil {
		t.Fatalf("default render failed: %v", err)
	}

	compactRendered, err := renderDiagram(source, Options{Width: 80})
	if err != nil {
		t.Fatalf("compact render failed: %v", err)
	}

	defaultWidth, defaultHeight := renderDimensions(strings.TrimRight(defaultRendered, "\n"))
	compactWidth, compactHeight := renderDimensions(compactRendered)

	if compactHeight > defaultHeight {
		t.Fatalf("expected compact render to be no taller than default, got compact=%d default=%d", compactHeight, defaultHeight)
	}
	if compactWidth > defaultWidth && compactHeight == defaultHeight {
		t.Fatalf("expected compact render to reduce size, got compact=%dx%d default=%dx%d", compactWidth, compactHeight, defaultWidth, defaultHeight)
	}
}
