package mermaid

import (
	"strings"
	"unicode"

	mermaidascii "github.com/AlexanderGrooff/mermaid-ascii/cmd"
	diagramconfig "github.com/AlexanderGrooff/mermaid-ascii/pkg/diagram"
	"github.com/charmbracelet/log"
	"github.com/mattn/go-runewidth"
)

// Options controls how Mermaid blocks are rendered for terminal output.
type Options struct {
	Width    int
	UseASCII bool
}

type fence struct {
	prefix string
	marker byte
	count  int
	info   string
}

// RenderMarkdown replaces Mermaid fenced code blocks with terminal-friendly
// diagram output wrapped in a plain code fence so Glamour can style it.
func RenderMarkdown(markdown string, opts Options) string {
	lines := strings.Split(markdown, "\n")
	out := make([]string, 0, len(lines))

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		start, ok := parseFenceStart(line)
		if !ok || !isMermaidFence(start.info) {
			out = append(out, line)
			continue
		}

		end := -1
		body := make([]string, 0, 16)
		for j := i + 1; j < len(lines); j++ {
			if isFenceEnd(lines[j], start) {
				end = j
				break
			}
			body = append(body, trimFenceIndent(lines[j], len(start.prefix)))
		}

		if end == -1 {
			out = append(out, line)
			continue
		}

		diagram, err := renderDiagram(strings.Join(body, "\n"), opts)
		if err != nil {
			log.Debug("unable to render mermaid block", "error", err)
			out = append(out, lines[i:end+1]...)
			i = end
			continue
		}

		out = append(out, wrapRenderedDiagram(start, diagram)...)
		i = end
	}

	return strings.Join(out, "\n")
}

func renderDiagram(source string, opts Options) (string, error) {
	source = normalizeMermaid(source)
	source = strings.TrimSpace(source)

	var (
		bestOutput string
		bestScore  int
		haveBest   bool
		lastErr    error
	)

	for _, config := range candidateConfigs(source, opts) {
		rendered, err := mermaidascii.RenderDiagram(source, config)
		if err != nil {
			lastErr = err
			continue
		}

		rendered = strings.TrimRight(rendered, "\n")
		score := renderScore(rendered, opts.Width)
		if !haveBest || score < bestScore {
			bestOutput = rendered
			bestScore = score
			haveBest = true
		}
	}

	if haveBest {
		return bestOutput, nil
	}

	return "", lastErr
}

func wrapRenderedDiagram(start fence, diagram string) []string {
	lines := []string{
		start.prefix + strings.Repeat(string(start.marker), start.count),
	}

	if diagram != "" {
		for _, line := range strings.Split(diagram, "\n") {
			lines = append(lines, start.prefix+line)
		}
	}

	lines = append(lines, start.prefix+strings.Repeat(string(start.marker), start.count))
	return lines
}

func parseFenceStart(line string) (fence, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	if len(trimmed) < 3 {
		return fence{}, false
	}

	marker := trimmed[0]
	if marker != '`' && marker != '~' {
		return fence{}, false
	}

	count := countRepeatedPrefix(trimmed, marker)
	if count < 3 {
		return fence{}, false
	}

	prefixLen := len(line) - len(trimmed)
	return fence{
		prefix: line[:prefixLen],
		marker: marker,
		count:  count,
		info:   strings.TrimSpace(trimmed[count:]),
	}, true
}

func isFenceEnd(line string, start fence) bool {
	trimmed := strings.TrimLeft(line, " \t")
	if len(trimmed) < start.count {
		return false
	}

	if trimmed[0] != start.marker {
		return false
	}

	count := countRepeatedPrefix(trimmed, start.marker)
	return count >= start.count && strings.TrimSpace(trimmed[count:]) == ""
}

func isMermaidFence(info string) bool {
	fields := strings.Fields(info)
	return len(fields) > 0 && strings.EqualFold(fields[0], "mermaid")
}

func countRepeatedPrefix(s string, marker byte) int {
	count := 0
	for count < len(s) && s[count] == marker {
		count++
	}
	return count
}

func trimFenceIndent(line string, indent int) string {
	for i := 0; i < indent && len(line) > 0; i++ {
		if line[0] != ' ' && line[0] != '\t' {
			break
		}
		line = line[1:]
	}
	return line
}

func candidateConfigs(source string, opts Options) []*diagramconfig.Config {
	makeConfig := func(boxPadding, paddingX, paddingY, participantSpacing, messageSpacing, selfWidth int) *diagramconfig.Config {
		config := diagramconfig.DefaultConfig()
		config.StyleType = "cli"
		config.UseAscii = opts.UseASCII
		config.BoxBorderPadding = boxPadding
		config.PaddingBetweenX = paddingX
		config.PaddingBetweenY = paddingY
		config.SequenceParticipantSpacing = participantSpacing
		config.SequenceMessageSpacing = messageSpacing
		config.SequenceSelfMessageWidth = selfWidth
		return config
	}

	if isSequenceSource(source) {
		return []*diagramconfig.Config{
			makeConfig(0, 1, 1, 1, 0, 2),
			makeConfig(0, 1, 1, 2, 0, 2),
			makeConfig(0, 2, 1, 2, 1, 2),
			makeConfig(0, 2, 2, 3, 1, 3),
		}
	}

	configs := []*diagramconfig.Config{
		makeConfig(0, 1, 1, 2, 0, 2),
		makeConfig(0, 2, 1, 2, 0, 2),
		makeConfig(0, 2, 2, 3, 1, 2),
		makeConfig(1, 2, 2, 3, 1, 3),
	}

	if opts.Width > 0 && opts.Width <= 72 {
		return configs[:3]
	}

	return configs
}

func renderScore(rendered string, viewportWidth int) int {
	width, height := renderDimensions(rendered)
	overflow := 0
	if viewportWidth > 0 && width > viewportWidth {
		overflow = width - viewportWidth
	}

	return overflow*100000 + width*height + height*100 + width
}

func renderDimensions(rendered string) (int, int) {
	if rendered == "" {
		return 0, 0
	}

	width := 0
	lines := strings.Split(rendered, "\n")
	for _, line := range lines {
		width = max(width, runewidth.StringWidth(line))
	}
	return width, len(lines)
}

func isSequenceSource(source string) bool {
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "%%") {
			continue
		}
		return strings.HasPrefix(trimmed, "sequenceDiagram")
	}
	return false
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func normalizeMermaid(source string) string {
	lines := strings.Split(source, "\n")
	labels := map[string]string{}
	out := make([]string, 0, len(lines))

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			out = append(out, line)
		case strings.HasPrefix(trimmed, "graph "), strings.HasPrefix(trimmed, "flowchart "):
			out = append(out, trimmed)
		case strings.HasPrefix(trimmed, "%%"):
			out = append(out, trimmed)
		case strings.HasPrefix(trimmed, "subgraph "), trimmed == "end":
			out = append(out, trimmed)
		case strings.HasPrefix(trimmed, "classDef "):
			out = append(out, trimmed)
		case strings.HasPrefix(trimmed, "class "), strings.HasPrefix(trimmed, "style "),
			strings.HasPrefix(trimmed, "click "), strings.HasPrefix(trimmed, "linkStyle "):
			// Drop directives the terminal renderer does not understand.
			continue
		default:
			out = append(out, normalizeLine(line, labels))
		}
	}

	return strings.Join(out, "\n")
}

func normalizeLine(line string, labels map[string]string) string {
	var b strings.Builder

	for i := 0; i < len(line); {
		consumed, replacement, ok := readNodeToken(line, i, labels)
		if ok {
			b.WriteString(replacement)
			i += consumed
			continue
		}
		b.WriteByte(line[i])
		i++
	}

	return b.String()
}

func readNodeToken(line string, start int, labels map[string]string) (int, string, bool) {
	if !isNodeStartBoundary(line, start) {
		return 0, "", false
	}

	i := start
	if i >= len(line) || !isIdentifierStart(rune(line[i])) {
		return 0, "", false
	}

	j := i + 1
	for j < len(line) && isIdentifierPart(rune(line[j])) {
		j++
	}

	id := line[i:j]

	if j < len(line) {
		if parsed, consumed, ok := parseLabeledNode(id, line[j:]); ok {
			label := sanitizeNodeLabel(parsed)
			if label != "" {
				labels[id] = label
				return j - i + consumed, label, true
			}
		}
	}

	if label, ok := labels[id]; ok && isNodeEndBoundary(line, j) {
		return j - i, label, true
	}

	return 0, "", false
}

func parseLabeledNode(id, rest string) (string, int, bool) {
	_ = id
	pairs := []struct {
		open  string
		close string
	}{
		{"([", "])"},
		{"[[", "]]"},
		{"[(", ")]"},
		{"((", "))"},
		{"{{", "}}"},
		{"[/", "/]"},
		{"[\\", "\\]"},
		{"[", "]"},
		{"(", ")"},
		{"{", "}"},
	}

	for _, pair := range pairs {
		if !strings.HasPrefix(rest, pair.open) {
			continue
		}

		end := strings.Index(rest[len(pair.open):], pair.close)
		if end < 0 {
			return "", 0, false
		}

		content := rest[len(pair.open) : len(pair.open)+end]
		return content, len(pair.open) + end + len(pair.close), true
	}

	return "", 0, false
}

func sanitizeNodeLabel(label string) string {
	label = strings.TrimSpace(label)
	label = strings.Trim(label, `"'`)
	label = strings.ReplaceAll(label, "<br>", " ")
	label = strings.ReplaceAll(label, "<br/>", " ")
	label = strings.ReplaceAll(label, "<br />", " ")
	label = strings.Join(strings.Fields(label), " ")
	return label
}

func isNodeStartBoundary(line string, idx int) bool {
	if idx == 0 {
		return true
	}

	prev := rune(line[idx-1])
	return unicode.IsSpace(prev) || strings.ContainsRune("([{-|>;,", prev)
}

func isNodeEndBoundary(line string, idx int) bool {
	if idx >= len(line) {
		return true
	}

	next := rune(line[idx])
	return unicode.IsSpace(next) || strings.ContainsRune(")]}-|<>,;:", next)
}

func isIdentifierStart(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func isIdentifierPart(r rune) bool {
	return isIdentifierStart(r) || r == '-'
}
