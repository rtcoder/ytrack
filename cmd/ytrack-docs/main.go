package main

import (
	"fmt"
	"os"

	"github.com/rtcoder/ytrack/internal/cli"
)

func main() {
	root := cli.NewRootCommand(cli.Options{})
	if _, err := fmt.Fprint(os.Stdout, cli.CommandReferenceMarkdown(root)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
