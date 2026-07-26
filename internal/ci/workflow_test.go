package ci_test

import (
	"os"
	"strings"
	"testing"
)

func TestPullRequestWorkflowRunsGoTests(t *testing.T) {
	content, err := os.ReadFile("../../.github/workflows/test.yml")
	if err != nil {
		t.Fatalf("read pull request workflow: %v", err)
	}
	workflow := string(content)

	for _, want := range []string{
		"pull_request:",
		"go-version-file: go.mod",
		"go test -count=1 ./...",
	} {
		if !strings.Contains(workflow, want) {
			t.Fatalf("workflow missing %q\n%s", want, workflow)
		}
	}
}
