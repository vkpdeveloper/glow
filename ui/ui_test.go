package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/glamour/styles"
)

func TestNewModelKeepsDirectDocumentBody(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	body := "# Title\n\nThis is a test document.\n"

	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write temp markdown: %v", err)
	}

	m, ok := newModel(Config{
		Path:         path,
		GlamourStyle: styles.DarkStyle,
	}, body).(model)
	if !ok {
		t.Fatal("newModel did not return ui.model")
	}

	if m.state != stateShowDocument {
		t.Fatalf("expected state %v, got %v", stateShowDocument, m.state)
	}

	if m.pager.currentDocument.Body != body {
		t.Fatalf("expected cached body %q, got %q", body, m.pager.currentDocument.Body)
	}
}
