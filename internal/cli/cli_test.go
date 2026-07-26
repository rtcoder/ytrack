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

func TestUserMePrintsCurrentUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"1-2","login":"rtcoder","name":"Robert","fullName":"Robert","email":"robert@example.com","$type":"Me"}`))
	}))
	defer server.Close()

	paths := configuredPaths(t, server.URL)

	out := runCLI(t, paths, "user", "me")

	if !strings.Contains(out, "id: 1-2") {
		t.Fatalf("user me output = %q, want id", out)
	}
	if !strings.Contains(out, "login: rtcoder") {
		t.Fatalf("user me output = %q, want login", out)
	}
	if !strings.Contains(out, "name: Robert") {
		t.Fatalf("user me output = %q, want name", out)
	}
	if !strings.Contains(out, "email: robert@example.com") {
		t.Fatalf("user me output = %q, want email", out)
	}
}

func TestUserListPrintsUsers(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		_, _ = w.Write([]byte(`[
			{"id":"24-55","login":"l.downton","name":"Luisa Downton","fullName":"Luisa Downton","email":"luisa@example.com","banned":false,"$type":"User"},
			{"id":"24-56","login":"m.scott","name":"Michael Scott","fullName":"Michael Scott","email":"michael@example.com","banned":true,"$type":"User"}
		]`))
	}))
	defer server.Close()

	paths := configuredPaths(t, server.URL)

	out := runCLI(t, paths, "user", "list", "--top", "2")

	if gotPath != "/api/users?%24top=2&fields=id%2Clogin%2Cname%2CfullName%2Cemail%2Cbanned" {
		t.Fatalf("path = %q, want limited users request", gotPath)
	}
	if !strings.Contains(out, "24-55") || !strings.Contains(out, "l.downton") || !strings.Contains(out, "Luisa Downton") {
		t.Fatalf("user list output = %q, want first user", out)
	}
	if !strings.Contains(out, "24-56") || !strings.Contains(out, "m.scott") || !strings.Contains(out, "banned") {
		t.Fatalf("user list output = %q, want banned marker", out)
	}
}

func TestUserFindPrintsUniqueMatchedUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
			{"id":"24-55","login":"l.downton","name":"Luisa Downton","fullName":"Luisa Downton","email":"luisa@example.com","banned":false,"$type":"User"},
			{"id":"24-56","login":"m.scott","name":"Michael Scott","fullName":"Michael Scott","email":"michael@example.com","banned":false,"$type":"User"}
		]`))
	}))
	defer server.Close()

	paths := configuredPaths(t, server.URL)

	out := runCLI(t, paths, "user", "find", "m.scott")

	if !strings.Contains(out, "id: 24-56") || !strings.Contains(out, "login: m.scott") || !strings.Contains(out, "name: Michael Scott") {
		t.Fatalf("user find output = %q, want matched user details", out)
	}
}

func TestUserMeJSONPrintsRawResponse(t *testing.T) {
	raw := `{"id":"1-2","login":"rtcoder","name":"Robert","fullName":"Robert","email":"robert@example.com","$type":"Me"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(raw))
	}))
	defer server.Close()

	paths := configuredPaths(t, server.URL)

	out := runCLI(t, paths, "--json", "user", "me")

	if strings.TrimSpace(out) != raw {
		t.Fatalf("user me json output = %q, want raw JSON", out)
	}
}

func configuredPaths(t *testing.T, serverURL string) config.Paths {
	t.Helper()

	temp := t.TempDir()
	paths := config.Paths{
		Global: filepath.Join(temp, "global", "config.json"),
		Local:  filepath.Join(temp, "project", ".ytrack", "config.json"),
	}
	runCLI(t, paths, "global", "set-url", serverURL)
	runCLI(t, paths, "global", "set-token", "perm:global-secret")
	return paths
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
