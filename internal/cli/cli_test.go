package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rtcoder/ytrack/internal/config"
)

func TestConfigCommandsWriteAndShowMaskedEffectiveConfig(t *testing.T) {
	temp := t.TempDir()
	paths := config.Paths{
		Global: filepath.Join(temp, "global", "config.json"),
		Local:  filepath.Join(temp, "project", ".ytrack", "config.json"),
	}

	runCLI(t, paths, "global", "set-url", "https://global.example")
	runCLI(t, paths, "global", "set-token", "perm:global-secret")
	runCLI(t, paths, "set-url", "https://local.example")
	runCLI(t, paths, "set-project-id", "0-1")

	out := runCLI(t, paths, "show")

	if !strings.Contains(out, "url: https://local.example") {
		t.Fatalf("show output = %q, want local url", out)
	}
	if !strings.Contains(out, "token: perm:xxxx...cret") {
		t.Fatalf("show output = %q, want masked token", out)
	}
	if !strings.Contains(out, "project_id: 0-1") {
		t.Fatalf("show output = %q, want local project_id", out)
	}
	if strings.Contains(out, "global-secret") {
		t.Fatalf("show output leaked full token: %q", out)
	}
}

func TestIssueCreateJSONUsesEffectiveConfig(t *testing.T) {
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		_, _ = w.Write([]byte(`{"id":"2-1","idReadable":"ART-123","summary":"Crash on save"}`))
	}))
	defer server.Close()

	temp := t.TempDir()
	paths := config.Paths{
		Global: filepath.Join(temp, "global", "config.json"),
		Local:  filepath.Join(temp, "project", ".ytrack", "config.json"),
	}
	runCLI(t, paths, "global", "set-url", server.URL)
	runCLI(t, paths, "global", "set-token", "perm:global-secret")
	runCLI(t, paths, "set-project-id", "0-1")

	out := runCLI(t, paths, "--json", "issue", "create", "Crash on save", "Steps")

	if strings.TrimSpace(out) != `{"id":"2-1","idReadable":"ART-123","summary":"Crash on save"}` {
		t.Fatalf("issue create output = %q, want raw JSON", out)
	}
	if gotPayload["summary"] != "Crash on save" || gotPayload["description"] != "Steps" {
		t.Fatalf("payload = %#v, want CLI arguments in request", gotPayload)
	}
}

func runCLI(t *testing.T, paths config.Paths, args ...string) string {
	t.Helper()

	var out, errOut bytes.Buffer
	cmd := NewRootCommand(Options{
		Paths: paths,
		Out:   &out,
		Err:   &errOut,
	})
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ytrack %s failed: %v\nstderr: %s", strings.Join(args, " "), err, errOut.String())
	}
	return out.String()
}
