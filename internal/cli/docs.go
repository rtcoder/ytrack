package cli

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func CommandReferenceMarkdown(root *cobra.Command) string {
	var out bytes.Buffer
	fmt.Fprintln(&out, "# ytrack command reference")
	fmt.Fprintln(&out)
	writeCommandMarkdown(&out, root)
	return out.String()
}

func writeCommandMarkdown(out *bytes.Buffer, cmd *cobra.Command) {
	if !cmd.IsAvailableCommand() && cmd.Name() != "ytrack" {
		return
	}
	fmt.Fprintf(out, "## %s\n\n", commandPath(cmd))
	if cmd.Short != "" {
		fmt.Fprintf(out, "%s\n\n", cmd.Short)
	}
	if cmd.UseLine() != "" {
		fmt.Fprintf(out, "```text\n%s\n```\n\n", cmd.UseLine())
	}

	children := cmd.Commands()
	sort.Slice(children, func(i, j int) bool {
		return children[i].CommandPath() < children[j].CommandPath()
	})
	for _, child := range children {
		writeCommandMarkdown(out, child)
	}
}

func commandPath(cmd *cobra.Command) string {
	path := cmd.CommandPath()
	return strings.TrimSpace(path)
}
