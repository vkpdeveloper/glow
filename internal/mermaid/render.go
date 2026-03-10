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

type segment struct {
	markdown string
	diagram  string
	indent   string
}

// RenderMarkdown replaces Mermaid fenced code blocks with terminal-friendly
// diagram output wrapped in a plain code fence so Glamour can style it.
func RenderMarkdown(markdown string, opts Options) string {
	segments := splitMarkdown(markdown, opts)
	lines := make([]string, 0, len(strings.Split(markdown, "\n")))

	for _, seg := range segments {
		if seg.markdown != "" {
			lines = append(lines, strings.Split(seg.markdown, "\n")...)
			continue
		}
		lines = append(lines, wrapRenderedDiagram(fence{prefix: seg.indent, marker: '`', count: 3}, seg.diagram)...)
	}

	return strings.Join(lines, "\n")
}

// Render renders markdown with Mermaid blocks emitted as plain terminal output
// so they bypass Glamour's code-block styling.
func Render(markdown string, opts Options, renderMarkdown func(string) (string, error)) (string, error) {
	segments := splitMarkdown(markdown, opts)

	var out strings.Builder
	for i, seg := range segments {
		if seg.markdown != "" {
			rendered, err := renderMarkdown(seg.markdown)
			if err != nil {
				return "", err
			}
			out.WriteString(rendered)
			continue
		}

		if out.Len() > 0 && !strings.HasSuffix(out.String(), "\n") {
			out.WriteByte('\n')
		}
		if out.Len() > 0 && !strings.HasSuffix(out.String(), "\n\n") {
			out.WriteByte('\n')
		}

		out.WriteString(indentDiagram(seg.diagram, seg.indent))

		if i+1 < len(segments) && !strings.HasSuffix(out.String(), "\n\n") {
			out.WriteString("\n\n")
		}
	}

	return out.String(), nil
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

		rendered = beautifyDiagram(strings.TrimRight(rendered, "\n"))
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

func splitMarkdown(markdown string, opts Options) []segment {
	lines := strings.Split(markdown, "\n")
	segments := make([]segment, 0, 8)
	markdownLines := make([]string, 0, len(lines))

	flushMarkdown := func() {
		if len(markdownLines) == 0 {
			return
		}
		segments = append(segments, segment{markdown: strings.Join(markdownLines, "\n")})
		markdownLines = markdownLines[:0]
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		start, ok := parseFenceStart(line)
		if !ok || !isMermaidFence(start.info) {
			markdownLines = append(markdownLines, line)
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
			markdownLines = append(markdownLines, line)
			continue
		}

		diagram, err := renderDiagram(strings.Join(body, "\n"), opts)
		if err != nil {
			log.Debug("unable to render mermaid block", "error", err)
			markdownLines = append(markdownLines, lines[i:end+1]...)
			i = end
			continue
		}

		flushMarkdown()
		segments = append(segments, segment{
			diagram: diagram,
			indent:  start.prefix,
		})
		i = end
	}

	flushMarkdown()
	return segments
}

func indentDiagram(diagram, indent string) string {
	if indent == "" || diagram == "" {
		return diagram
	}

	lines := strings.Split(diagram, "\n")
	for i, line := range lines {
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n")
}

func beautifyDiagram(diagram string) string {
	if diagram == "" {
		return diagram
	}

	lines := strings.Split(diagram, "\n")
	lines = slimBoxBands(lines)
	lines = compressConnectorRows(lines, 1)

	for i, line := range lines {
		lines[i] = strings.TrimRightFunc(styleBoxCorners(line), unicode.IsSpace)
	}

	return strings.Join(lines, "\n")
}

func slimBoxBands(lines []string) []string {
	out := make([]string, 0, len(lines))

	for i := 0; i < len(lines); {
		if i+4 < len(lines) &&
			isTopBorderBand(lines[i]) &&
			isVerticalPaddingBand(lines[i+1]) &&
			hasBoxTextBand(lines[i+2]) &&
			isVerticalPaddingBand(lines[i+3]) &&
			isBottomBorderBand(lines[i+4]) {
			out = append(out,
				styleTopBorder(lines[i]),
				lines[i+2],
				styleBottomBorder(lines[i+4]),
			)
			i += 5
			continue
		}

		out = append(out, styleBoxCorners(lines[i]))
		i++
	}

	return out
}

func compressConnectorRows(lines []string, maxRun int) []string {
	out := make([]string, 0, len(lines))
	run := 0

	for _, line := range lines {
		if isConnectorOnlyRow(line) {
			if run < maxRun {
				out = append(out, line)
			}
			run++
			continue
		}

		run = 0
		out = append(out, line)
	}

	return out
}

func styleBoxCorners(line string) string {
	replacer := strings.NewReplacer(
		"┌", "╭",
		"┐", "╮",
		"└", "╰",
		"┘", "╯",
	)
	return replacer.Replace(line)
}

func styleTopBorder(line string) string {
	return styleBoxCorners(line)
}

func styleBottomBorder(line string) string {
	return styleBoxCorners(line)
}

func isTopBorderBand(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed != "" &&
		strings.ContainsAny(trimmed, "┌+") &&
		strings.ContainsAny(trimmed, "┐+") &&
		!containsTextRune(trimmed)
}

func isBottomBorderBand(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed != "" &&
		strings.ContainsAny(trimmed, "└+") &&
		strings.ContainsAny(trimmed, "┘+") &&
		!containsTextRune(trimmed)
}

func isVerticalPaddingBand(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || containsTextRune(trimmed) {
		return false
	}

	for _, r := range trimmed {
		if !strings.ContainsRune("│| ", r) {
			return false
		}
	}

	return strings.ContainsRune(trimmed, '│') || strings.ContainsRune(trimmed, '|')
}

func hasBoxTextBand(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}

	return (strings.ContainsRune(trimmed, '│') || strings.ContainsRune(trimmed, '|')) && containsTextRune(trimmed)
}

func isConnectorOnlyRow(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || containsTextRune(trimmed) {
		return false
	}

	for _, r := range trimmed {
		if !strings.ContainsRune("│|", r) {
			return false
		}
	}

	return true
}

func containsTextRune(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
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
