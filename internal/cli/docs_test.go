package cli

import (
	"strings"
	"testing"
)

func TestCommandReferenceMarkdownIncludesNestedCommands(t *testing.T) {
	root := NewRootCommand(Options{})

	doc := CommandReferenceMarkdown(root)

	for _, want := range []string{
		"# ytrack command reference",
		"## ytrack issue create",
		"## ytrack issue list",
		"## ytrack project list",
		"## ytrack user me",
		"## ytrack completion",
		"ytrack completion bash|zsh|fish|powershell",
	} {
		if !strings.Contains(doc, want) {
			t.Fatalf("command reference missing %q\n%s", want, doc)
		}
	}
}
