package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestInitPromptsAndWritesLocalProjectConfig(t *testing.T) {
	temp := t.TempDir()
	paths := config.Paths{
		Global: filepath.Join(temp, "global", "config.json"),
		Local:  filepath.Join(temp, "project", ".ytrack", "config.json"),
	}

	out := runCLIWithInput(t, paths, "https://youtrack.example.com\nperm:secret\n0-3\n", "init")

	for _, want := range []string{
		"Set YouTrack URL:",
		"Set YouTrack token:",
		"Set project ID:",
		"Saved local project config",
		"Add .ytrack/ to .gitignore",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("init output = %q, want %q", out, want)
		}
	}
	if strings.Contains(out, "perm:secret") {
		t.Fatalf("init output leaked token: %q", out)
	}
	cfg, err := config.Load(paths.Local)
	if err != nil {
		t.Fatalf("load local config: %v", err)
	}
	if cfg.URL != "https://youtrack.example.com" || cfg.Token != "perm:secret" || cfg.ProjectID != "0-3" {
		t.Fatalf("local config = %+v, want prompted values", cfg)
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

func TestIssueCreateAcceptsMetadataOptions(t *testing.T) {
	var gotPayload map[string]any
	var sawAssigneeLookup bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/users/me":
			sawAssigneeLookup = true
			_, _ = w.Write([]byte(`{"id":"1-2","login":"rtcoder","name":"Robert","fullName":"Robert","email":"robert@example.com","$type":"Me"}`))
		case "/api/issues":
			if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			_, _ = w.Write([]byte(`{"id":"2-1","idReadable":"ART-123","summary":"Crash on save"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
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

	out := runCLI(t, paths, "issue", "create", "Crash on save", "Steps", "--type", "Bug", "--assignee", "me", "--priority", "High", "--version", "v0.1.10")

	if !sawAssigneeLookup {
		t.Fatal("assignee lookup was not requested")
	}
	fields := gotPayload["customFields"].([]any)
	if len(fields) != 4 {
		t.Fatalf("customFields = %#v, want type, assignee, priority, version", fields)
	}
	if !strings.Contains(out, `Created ART-123: "Crash on save"`) {
		t.Fatalf("issue create output = %q, want created issue", out)
	}
}

func TestIssueCreatePrintsIssueURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"2-1","idReadable":"ART-123","summary":"Crash on save"}`))
	}))
	defer server.Close()

	paths := configuredPaths(t, server.URL)
	runCLI(t, paths, "set-project-id", "0-1")

	out := runCLI(t, paths, "issue", "create", "Crash on save")

	if !strings.Contains(out, "url: "+server.URL+"/issue/ART-123") {
		t.Fatalf("issue create output = %q, want issue URL", out)
	}
}

func TestIssueCreateReadsDescriptionFile(t *testing.T) {
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		_, _ = w.Write([]byte(`{"id":"2-1","idReadable":"ART-123","summary":"Crash on save"}`))
	}))
	defer server.Close()

	temp := t.TempDir()
	descPath := filepath.Join(temp, "description.md")
	if err := os.WriteFile(descPath, []byte("Steps from file\n"), 0o600); err != nil {
		t.Fatalf("write description file: %v", err)
	}
	paths := configuredPaths(t, server.URL)
	runCLI(t, paths, "set-project-id", "0-1")

	runCLI(t, paths, "issue", "create", "Crash on save", "--description-file", descPath)

	if gotPayload["description"] != "Steps from file\n" {
		t.Fatalf("payload = %#v, want file description", gotPayload)
	}
}

func TestIssueCreateReadsDescriptionFromStdin(t *testing.T) {
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		_, _ = w.Write([]byte(`{"id":"2-1","idReadable":"ART-123","summary":"Crash on save"}`))
	}))
	defer server.Close()

	paths := configuredPaths(t, server.URL)
	runCLI(t, paths, "set-project-id", "0-1")

	runCLIWithInput(t, paths, "Steps from stdin\n", "issue", "create", "Crash on save", "--description-file", "-")

	if gotPayload["description"] != "Steps from stdin\n" {
		t.Fatalf("payload = %#v, want stdin description", gotPayload)
	}
}

func TestIssueCloseSetsStatusToFixed(t *testing.T) {
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		_, _ = w.Write([]byte(`{"issues":[{"id":"3-1","idReadable":"YR-14","summary":"Add init"}],"commands":"State Fixed","errors":[]}`))
	}))
	defer server.Close()

	paths := configuredPaths(t, server.URL)

	out := runCLI(t, paths, "issue", "close", "YR-14")

	if gotPayload["query"] != "State Fixed" {
		t.Fatalf("payload = %#v, want State Fixed command", gotPayload)
	}
	if !strings.Contains(out, "Updated YR-14 to Fixed") {
		t.Fatalf("issue close output = %q, want close confirmation", out)
	}
}

func TestIssueListListsConfiguredProjectIssues(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		_, _ = w.Write([]byte(`[{"id":"3-1","idReadable":"YR-14","summary":"Add init","customFields":[{"name":"State","value":{"name":"Submitted","$type":"StateBundleElement"},"$type":"StateIssueCustomField"}],"$type":"Issue"}]`))
	}))
	defer server.Close()

	paths := configuredPaths(t, server.URL)
	runCLI(t, paths, "set-project-id", "0-3")

	out := runCLI(t, paths, "issue", "list")

	if gotPath != "/api/admin/projects/0-3/issues?fields=id,idReadable,summary,customFields(name,value(name,login))" {
		t.Fatalf("path = %q, want project issue list", gotPath)
	}
	if !strings.Contains(out, "YR-14") || !strings.Contains(out, "Submitted") || !strings.Contains(out, "Add init") {
		t.Fatalf("issue list output = %q, want issue row", out)
	}
}

func TestIssueListSupportsStateAndAssigneeFilters(t *testing.T) {
	var requested []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = append(requested, r.URL.RequestURI())
		switch r.URL.Path {
		case "/api/admin/projects/0-3":
			_, _ = w.Write([]byte(`{"id":"0-3","shortName":"YR","name":"ytrack","$type":"Project"}`))
		case "/api/issues":
			_, _ = w.Write([]byte(`[{"id":"3-1","idReadable":"YR-14","summary":"Add init","customFields":[],"$type":"Issue"}]`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	paths := configuredPaths(t, server.URL)
	runCLI(t, paths, "set-project-id", "0-3")

	runCLI(t, paths, "issue", "list", "--state", "Submitted", "--assigned-to", "me")

	wantIssuesPath := "/api/issues?fields=id%2CidReadable%2Csummary%2CcustomFields%28name%2Cvalue%28name%2Clogin%29%29&query=project%3A+YR+State%3A+%7BSubmitted%7D+Assignee%3A+%7Bme%7D"
	if len(requested) != 2 || requested[1] != wantIssuesPath {
		t.Fatalf("requested = %#v, want filtered issue search", requested)
	}
}

func TestIssueShowPrintsIssueDetailsAndURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RequestURI() != "/api/issues/YR-14?fields=id,idReadable,summary,description,customFields(name,value(name,login))" {
			t.Fatalf("path = %q, want issue details", r.URL.RequestURI())
		}
		_, _ = w.Write([]byte(`{"id":"3-1","idReadable":"YR-14","summary":"Add init","description":"Interactive setup","customFields":[{"name":"State","value":{"name":"Submitted","$type":"StateBundleElement"},"$type":"StateIssueCustomField"},{"name":"Assignee","value":{"login":"rtcoder","name":"Robert","$type":"User"},"$type":"SingleUserIssueCustomField"},{"name":"Priority","value":{"name":"Normal","$type":"EnumBundleElement"},"$type":"SingleEnumIssueCustomField"}],"$type":"Issue"}`))
	}))
	defer server.Close()

	paths := configuredPaths(t, server.URL)

	out := runCLI(t, paths, "issue", "show", "YR-14")

	for _, want := range []string{
		"id: YR-14",
		"title: Add init",
		"state: Submitted",
		"assignee: rtcoder",
		"priority: Normal",
		"url: " + server.URL + "/issue/YR-14",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("issue show output = %q, want %q", out, want)
		}
	}
}

func TestIssueShowJSONPrintsRawResponse(t *testing.T) {
	raw := `{"id":"3-1","idReadable":"YR-14","summary":"Add init","description":"Interactive setup","customFields":[],"$type":"Issue"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(raw))
	}))
	defer server.Close()

	paths := configuredPaths(t, server.URL)

	out := runCLI(t, paths, "--json", "issue", "show", "YR-14")

	if strings.TrimSpace(out) != raw {
		t.Fatalf("issue show json output = %q, want raw JSON", out)
	}
}

func TestIssueTypeUpdatesTypeField(t *testing.T) {
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		_, _ = w.Write([]byte(`{"id":"3-1","idReadable":"YR-14","summary":"Add init","customFields":[{"name":"Type","value":{"name":"Task","$type":"EnumBundleElement"},"$type":"SingleEnumIssueCustomField"}],"$type":"Issue"}`))
	}))
	defer server.Close()

	paths := configuredPaths(t, server.URL)

	out := runCLI(t, paths, "issue", "type", "YR-14", "Task")

	fields := gotPayload["customFields"].([]any)
	typeField := fields[0].(map[string]any)
	typeValue := typeField["value"].(map[string]any)
	if typeField["name"] != "Type" || typeValue["name"] != "Task" {
		t.Fatalf("payload = %#v, want Type Task", gotPayload)
	}
	if !strings.Contains(out, "Updated YR-14 type to Task") {
		t.Fatalf("issue type output = %q, want update confirmation", out)
	}
}

func TestIssuePriorityUpdatesPriorityField(t *testing.T) {
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		_, _ = w.Write([]byte(`{"id":"3-1","idReadable":"YR-14","summary":"Add init","customFields":[{"name":"Priority","value":{"name":"High","$type":"EnumBundleElement"},"$type":"SingleEnumIssueCustomField"}],"$type":"Issue"}`))
	}))
	defer server.Close()

	paths := configuredPaths(t, server.URL)

	out := runCLI(t, paths, "issue", "priority", "YR-14", "High")

	fields := gotPayload["customFields"].([]any)
	priorityField := fields[0].(map[string]any)
	priorityValue := priorityField["value"].(map[string]any)
	if priorityField["name"] != "Priority" || priorityValue["name"] != "High" {
		t.Fatalf("payload = %#v, want Priority High", gotPayload)
	}
	if !strings.Contains(out, "Updated YR-14 priority to High") {
		t.Fatalf("issue priority output = %q, want update confirmation", out)
	}
}

func TestIssueEditUpdatesProvidedFields(t *testing.T) {
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		_, _ = w.Write([]byte(`{"id":"3-1","idReadable":"YR-14","summary":"New title","description":"New description","customFields":[{"name":"Type","value":{"name":"Task","$type":"EnumBundleElement"},"$type":"SingleEnumIssueCustomField"},{"name":"Priority","value":{"name":"High","$type":"EnumBundleElement"},"$type":"SingleEnumIssueCustomField"}],"$type":"Issue"}`))
	}))
	defer server.Close()

	paths := configuredPaths(t, server.URL)

	out := runCLI(t, paths, "issue", "edit", "YR-14", "-t", "New title", "-d", "New description", "--type", "Task", "--priority", "High")

	if gotPayload["summary"] != "New title" || gotPayload["description"] != "New description" {
		t.Fatalf("payload = %#v, want title and description", gotPayload)
	}
	fields := gotPayload["customFields"].([]any)
	if len(fields) != 2 {
		t.Fatalf("customFields = %#v, want Type and Priority", fields)
	}
	if !strings.Contains(out, "Updated YR-14") {
		t.Fatalf("issue edit output = %q, want update confirmation", out)
	}
}

func TestIssueEditJSONPrintsRawResponse(t *testing.T) {
	raw := `{"id":"3-1","idReadable":"YR-14","summary":"New title","description":"New description","customFields":[],"$type":"Issue"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(raw))
	}))
	defer server.Close()

	paths := configuredPaths(t, server.URL)

	out := runCLI(t, paths, "--json", "issue", "edit", "YR-14", "--title", "New title")

	if strings.TrimSpace(out) != raw {
		t.Fatalf("issue edit json output = %q, want raw JSON", out)
	}
}

func TestIssueCommentAddsComment(t *testing.T) {
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/issues/YR-14/comments" {
			t.Fatalf("path = %q, want comments endpoint", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		_, _ = w.Write([]byte(`{"id":"4-1","text":"Looks good","author":{"login":"rtcoder","$type":"User"},"$type":"IssueComment"}`))
	}))
	defer server.Close()

	paths := configuredPaths(t, server.URL)

	out := runCLI(t, paths, "issue", "comment", "YR-14", "Looks good")

	if gotPayload["text"] != "Looks good" {
		t.Fatalf("payload = %#v, want comment text", gotPayload)
	}
	if !strings.Contains(out, "Added comment to YR-14") {
		t.Fatalf("issue comment output = %q, want confirmation", out)
	}
}

func TestIssueAssignResolvesUserAndSetsAssignee(t *testing.T) {
	var gotPayload map[string]any
	var sawUserLookup bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/users/me":
			sawUserLookup = true
			_, _ = w.Write([]byte(`{"id":"1-2","login":"rtcoder","name":"Robert","fullName":"Robert","email":"robert@example.com","$type":"Me"}`))
		case "/api/issues/YR-14":
			if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			_, _ = w.Write([]byte(`{"id":"3-1","idReadable":"YR-14","summary":"Add init","customFields":[{"name":"Assignee","value":{"id":"1-2","login":"rtcoder","$type":"User"},"$type":"SingleUserIssueCustomField"}],"$type":"Issue"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	paths := configuredPaths(t, server.URL)

	out := runCLI(t, paths, "issue", "assign", "YR-14", "me")

	if !sawUserLookup {
		t.Fatal("user lookup was not requested")
	}
	fields := gotPayload["customFields"].([]any)
	assignee := fields[0].(map[string]any)
	value := assignee["value"].(map[string]any)
	if assignee["name"] != "Assignee" || value["id"] != "1-2" {
		t.Fatalf("payload = %#v, want assignee 1-2", gotPayload)
	}
	if !strings.Contains(out, "Updated YR-14 assignee to me") {
		t.Fatalf("issue assign output = %q, want confirmation", out)
	}
}

func TestIssueCommandAppliesArbitraryCommand(t *testing.T) {
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		_, _ = w.Write([]byte(`{"issues":[{"id":"3-1","idReadable":"YR-14","summary":"Add init"}],"commands":"Priority High","errors":[]}`))
	}))
	defer server.Close()

	paths := configuredPaths(t, server.URL)

	out := runCLI(t, paths, "issue", "command", "YR-14", "Priority High")

	if gotPayload["query"] != "Priority High" {
		t.Fatalf("payload = %#v, want command query", gotPayload)
	}
	if !strings.Contains(out, "Updated YR-14") {
		t.Fatalf("issue command output = %q, want update confirmation", out)
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

func TestProjectCreateWithFlagsResolvesSupportedLeaderRefs(t *testing.T) {
	tests := []struct {
		name       string
		leaderRef  string
		leaderID   string
		leaderPath string
		leaderBody string
	}{
		{
			name:       "me",
			leaderRef:  "me",
			leaderID:   "1-2",
			leaderPath: "/api/users/me",
			leaderBody: `{"id":"1-2","login":"rtcoder","name":"Robert","fullName":"Robert","email":"robert@example.com","$type":"Me"}`,
		},
		{
			name:       "login",
			leaderRef:  "rtcoder",
			leaderID:   "1-2",
			leaderPath: "/api/users",
			leaderBody: `[
				{"id":"1-2","login":"rtcoder","name":"Robert","fullName":"Robert","email":"robert@example.com","banned":false,"$type":"User"},
				{"id":"24-56","login":"m.scott","name":"Michael Scott","fullName":"Michael Scott","email":"michael@example.com","banned":false,"$type":"User"}
			]`,
		},
		{
			name:       "id",
			leaderRef:  "1-2",
			leaderID:   "1-2",
			leaderPath: "/api/users/1-2",
			leaderBody: `{"id":"1-2","login":"rtcoder","name":"Robert","fullName":"Robert","email":"robert@example.com","banned":false,"$type":"User"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPayload map[string]any
			var sawLeaderLookup bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case tt.leaderPath:
					sawLeaderLookup = true
					_, _ = w.Write([]byte(tt.leaderBody))
				case "/api/admin/projects":
					if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
						t.Fatalf("decode request body: %v", err)
					}
					_, _ = w.Write([]byte(`{"id":"0-16","shortName":"MOB","name":"Mobile App","leader":{"id":"1-2","login":"rtcoder","name":"Robert","$type":"User"},"$type":"Project"}`))
				default:
					t.Fatalf("unexpected path %s", r.URL.Path)
				}
			}))
			defer server.Close()

			paths := configuredPaths(t, server.URL)

			out := runCLI(t, paths, "project", "create", "Mobile App", "--key", "MOB", "--leader", tt.leaderRef)

			if !sawLeaderLookup {
				t.Fatalf("leader lookup for %q was not requested", tt.leaderRef)
			}
			if gotPayload["name"] != "Mobile App" || gotPayload["shortName"] != "MOB" {
				t.Fatalf("payload = %#v, want project name and key", gotPayload)
			}
			leader := gotPayload["leader"].(map[string]any)
			if leader["id"] != tt.leaderID {
				t.Fatalf("leader id = %q, want %s", leader["id"], tt.leaderID)
			}
			if !strings.Contains(out, `Created project MOB: "Mobile App"`) || !strings.Contains(out, "id: 0-16") {
				t.Fatalf("project create output = %q, want created project details", out)
			}
		})
	}
}

func TestProjectCreatePromptsForMissingValues(t *testing.T) {
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/users/me":
			_, _ = w.Write([]byte(`{"id":"1-2","login":"rtcoder","name":"Robert","fullName":"Robert","email":"robert@example.com","$type":"Me"}`))
		case "/api/admin/projects":
			if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			_, _ = w.Write([]byte(`{"id":"0-16","shortName":"MOB","name":"Mobile App","leader":{"id":"1-2","login":"rtcoder","name":"Robert","$type":"User"},"$type":"Project"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	paths := configuredPaths(t, server.URL)

	out := runCLIWithInput(t, paths, "Mobile App\nMOB\nme\n", "project", "create")

	if !strings.Contains(out, "Set project name:") || !strings.Contains(out, "Set project key:") || !strings.Contains(out, "Set leader:") {
		t.Fatalf("project create interactive output = %q, want prompts", out)
	}
	if gotPayload["name"] != "Mobile App" || gotPayload["shortName"] != "MOB" {
		t.Fatalf("payload = %#v, want prompted project name and key", gotPayload)
	}
	leader := gotPayload["leader"].(map[string]any)
	if leader["id"] != "1-2" {
		t.Fatalf("leader id = %q, want 1-2", leader["id"])
	}
}

func TestProjectCreateSetProjectIDFlagSavesNewProjectAsLocalProject(t *testing.T) {
	server := newProjectCreateServer(t, "0-16")
	defer server.Close()

	paths := configuredPaths(t, server.URL)

	out := runCLI(t, paths, "project", "create", "Mobile App", "--key", "MOB", "--leader", "me", "--set-project-id")

	if !strings.Contains(out, "Saved local project_id: 0-16") {
		t.Fatalf("project create output = %q, want local project_id confirmation", out)
	}
	cfg, err := config.Load(paths.Local)
	if err != nil {
		t.Fatalf("load local config: %v", err)
	}
	if cfg.ProjectID != "0-16" {
		t.Fatalf("local project_id = %q, want 0-16", cfg.ProjectID)
	}
}

func TestProjectCreateInteractivePromptsToSaveNewProjectAsLocalProject(t *testing.T) {
	server := newProjectCreateServer(t, "0-16")
	defer server.Close()

	paths := configuredPaths(t, server.URL)

	out := runCLIWithInput(t, paths, "Mobile App\nMOB\nme\ny\n", "project", "create")

	if !strings.Contains(out, "Set new project as local project_id? [y/N]") {
		t.Fatalf("project create output = %q, want save project prompt", out)
	}
	cfg, err := config.Load(paths.Local)
	if err != nil {
		t.Fatalf("load local config: %v", err)
	}
	if cfg.ProjectID != "0-16" {
		t.Fatalf("local project_id = %q, want 0-16", cfg.ProjectID)
	}
}

func TestProjectCreateInteractivePromptsBeforeOverwritingExistingProjectID(t *testing.T) {
	server := newProjectCreateServer(t, "0-16")
	defer server.Close()

	paths := configuredPaths(t, server.URL)
	runCLI(t, paths, "set-project-id", "0-1")

	out := runCLIWithInput(t, paths, "Mobile App\nMOB\nme\nn\n", "project", "create")

	if !strings.Contains(out, "Local project_id is 0-1. Overwrite with 0-16? [y/N]") {
		t.Fatalf("project create output = %q, want overwrite prompt", out)
	}
	cfg, err := config.Load(paths.Local)
	if err != nil {
		t.Fatalf("load local config: %v", err)
	}
	if cfg.ProjectID != "0-1" {
		t.Fatalf("local project_id = %q, want existing value preserved", cfg.ProjectID)
	}
}

func TestProjectCreateJSONPrintsRawResponse(t *testing.T) {
	raw := `{"id":"0-16","shortName":"MOB","name":"Mobile App","leader":{"id":"1-2","login":"rtcoder","name":"Robert","$type":"User"},"$type":"Project"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/users/me":
			_, _ = w.Write([]byte(`{"id":"1-2","login":"rtcoder","name":"Robert","fullName":"Robert","email":"robert@example.com","$type":"Me"}`))
		case "/api/admin/projects":
			_, _ = w.Write([]byte(raw))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	paths := configuredPaths(t, server.URL)

	out := runCLI(t, paths, "--json", "project", "create", "Mobile App", "--key", "MOB", "--leader", "me")

	if strings.TrimSpace(out) != raw {
		t.Fatalf("project create json output = %q, want raw JSON", out)
	}
}

func TestProjectCreateJSONWithSetProjectIDSavesLocalProjectAndPrintsOnlyRawResponse(t *testing.T) {
	raw := `{"id":"0-16","shortName":"MOB","name":"Mobile App","leader":{"id":"1-2","login":"rtcoder","name":"Robert","$type":"User"},"$type":"Project"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/users/me":
			_, _ = w.Write([]byte(`{"id":"1-2","login":"rtcoder","name":"Robert","fullName":"Robert","email":"robert@example.com","$type":"Me"}`))
		case "/api/admin/projects":
			_, _ = w.Write([]byte(raw))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	paths := configuredPaths(t, server.URL)

	out := runCLI(t, paths, "--json", "project", "create", "Mobile App", "--key", "MOB", "--leader", "me", "--set-project-id")

	if strings.TrimSpace(out) != raw {
		t.Fatalf("project create json output = %q, want raw JSON only", out)
	}
	cfg, err := config.Load(paths.Local)
	if err != nil {
		t.Fatalf("load local config: %v", err)
	}
	if cfg.ProjectID != "0-16" {
		t.Fatalf("local project_id = %q, want 0-16", cfg.ProjectID)
	}
}

func TestProjectListCommandsPrintProjectResources(t *testing.T) {
	tests := []struct {
		name       string
		resource   string
		response   string
		wantPath   string
		wantOutput []string
	}{
		{
			name:     "issues",
			resource: "issues",
			response: `[{"id":"3-1","idReadable":"YR-14","summary":"Add init","customFields":[{"name":"State","value":{"name":"Submitted","$type":"StateBundleElement"},"$type":"StateIssueCustomField"}],"$type":"Issue"}]`,
			wantPath: "/api/admin/projects/0-3/issues?fields=id,idReadable,summary,customFields(name,value(name,login))",
			wantOutput: []string{
				"YR-14",
				"Submitted",
				"Add init",
			},
		},
		{
			name:     "users",
			resource: "users",
			response: `[{"id":"1-2","login":"rtcoder","name":"Robert","fullName":"Robert","email":"robert@example.com","banned":false,"$type":"User"}]`,
			wantPath: "/api/admin/projects/0-3/team/users?fields=id,login,name,fullName,email,banned",
			wantOutput: []string{
				"1-2",
				"rtcoder",
				"Robert",
			},
		},
		{
			name:     "statuses",
			resource: "statuses",
			response: projectCustomFieldsResponse(),
			wantPath: "/api/admin/projects/0-3/customFields?fields=id,field(name),bundle(values(id,name))",
			wantOutput: []string{
				"162-0",
				"Submitted",
				"162-7",
				"Fixed",
			},
		},
		{
			name:     "types",
			resource: "types",
			response: projectCustomFieldsResponse(),
			wantPath: "/api/admin/projects/0-3/customFields?fields=id,field(name),bundle(values(id,name))",
			wantOutput: []string{
				"160-5",
				"Bug",
			},
		},
		{
			name:     "priorities",
			resource: "priorities",
			response: projectCustomFieldsResponse(),
			wantPath: "/api/admin/projects/0-3/customFields?fields=id,field(name),bundle(values(id,name))",
			wantOutput: []string{
				"160-3",
				"Normal",
			},
		},
		{
			name:     "versions",
			resource: "versions",
			response: projectCustomFieldsResponse(),
			wantPath: "/api/admin/projects/0-3/customFields?fields=id,field(name),bundle(values(id,name))",
			wantOutput: []string{
				"170-1",
				"v0.1.7",
				"170-2",
				"v0.1.6",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.RequestURI()
				_, _ = w.Write([]byte(tt.response))
			}))
			defer server.Close()

			paths := configuredPaths(t, server.URL)
			runCLI(t, paths, "set-project-id", "0-3")

			out := runCLI(t, paths, "project", "list", tt.resource)

			if gotPath != tt.wantPath {
				t.Fatalf("path = %q, want %q", gotPath, tt.wantPath)
			}
			for _, want := range tt.wantOutput {
				if !strings.Contains(out, want) {
					t.Fatalf("project list %s output = %q, want %q", tt.resource, out, want)
				}
			}
		})
	}
}

func TestProjectListJSONPrintsRawResponse(t *testing.T) {
	raw := `[{"id":"1-2","login":"rtcoder","name":"Robert","fullName":"Robert","email":"robert@example.com","banned":false,"$type":"User"}]`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(raw))
	}))
	defer server.Close()

	paths := configuredPaths(t, server.URL)
	runCLI(t, paths, "set-project-id", "0-3")

	out := runCLI(t, paths, "--json", "project", "list", "users")

	if strings.TrimSpace(out) != raw {
		t.Fatalf("project list json output = %q, want raw JSON", out)
	}
}

func TestProjectListIssuesWithFiltersSearchesIssues(t *testing.T) {
	var requested []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = append(requested, r.URL.RequestURI())
		switch r.URL.Path {
		case "/api/admin/projects/0-3":
			_, _ = w.Write([]byte(`{"id":"0-3","shortName":"YR","name":"ytrack","$type":"Project"}`))
		case "/api/issues":
			_, _ = w.Write([]byte(`[{"id":"3-1","idReadable":"YR-14","summary":"Add init","customFields":[{"name":"State","value":{"name":"Submitted","$type":"StateBundleElement"},"$type":"StateIssueCustomField"}],"$type":"Issue"}]`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	paths := configuredPaths(t, server.URL)
	runCLI(t, paths, "set-project-id", "0-3")

	out := runCLI(t, paths, "project", "list", "issues", "--status", "Submitted", "--user", "me", "--type", "Bug", "--priority", "Normal")

	wantIssuesPath := "/api/issues?fields=id%2CidReadable%2Csummary%2CcustomFields%28name%2Cvalue%28name%2Clogin%29%29&query=project%3A+YR+State%3A+%7BSubmitted%7D+Assignee%3A+%7Bme%7D+Type%3A+%7BBug%7D+Priority%3A+%7BNormal%7D"
	if len(requested) != 2 || requested[1] != wantIssuesPath {
		t.Fatalf("requested = %#v, want filtered issue search", requested)
	}
	if !strings.Contains(out, "YR-14") || !strings.Contains(out, "Submitted") || !strings.Contains(out, "Add init") {
		t.Fatalf("project list issues output = %q, want filtered issue row", out)
	}
}

func newProjectCreateServer(t *testing.T, projectID string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/users/me":
			_, _ = w.Write([]byte(`{"id":"1-2","login":"rtcoder","name":"Robert","fullName":"Robert","email":"robert@example.com","$type":"Me"}`))
		case "/api/admin/projects":
			fmt.Fprintf(w, `{"id":%q,"shortName":"MOB","name":"Mobile App","leader":{"id":"1-2","login":"rtcoder","name":"Robert","$type":"User"},"$type":"Project"}`, projectID)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
}

func projectCustomFieldsResponse() string {
	return `[
		{"id":"186-13","field":{"name":"State","$type":"CustomField"},"bundle":{"values":[{"id":"162-0","name":"Submitted","$type":"StateBundleElement"},{"id":"162-7","name":"Fixed","$type":"StateBundleElement"}],"$type":"StateBundle"},"$type":"StateProjectCustomField"},
		{"id":"186-12","field":{"name":"Type","$type":"CustomField"},"bundle":{"values":[{"id":"160-5","name":"Bug","$type":"EnumBundleElement"}],"$type":"EnumBundle"},"$type":"EnumProjectCustomField"},
		{"id":"186-11","field":{"name":"Priority","$type":"CustomField"},"bundle":{"values":[{"id":"160-3","name":"Normal","$type":"EnumBundleElement"}],"$type":"EnumBundle"},"$type":"EnumProjectCustomField"},
		{"id":"186-15","field":{"name":"Fix versions","$type":"CustomField"},"bundle":{"values":[{"id":"170-1","name":"v0.1.7","$type":"VersionBundleElement"}],"$type":"VersionBundle"},"$type":"VersionProjectCustomField"},
		{"id":"186-16","field":{"name":"Affected versions","$type":"CustomField"},"bundle":{"values":[{"id":"170-2","name":"v0.1.6","$type":"VersionBundleElement"}],"$type":"VersionBundle"},"$type":"VersionProjectCustomField"}
	]`
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
	return runCLIWithInput(t, paths, "", args...)
}

func runCLIWithInput(t *testing.T, paths config.Paths, input string, args ...string) string {
	t.Helper()

	var out, errOut bytes.Buffer
	cmd := NewRootCommand(Options{
		Paths: paths,
		In:    strings.NewReader(input),
		Out:   &out,
		Err:   &errOut,
	})
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ytrack %s failed: %v\nstderr: %s", strings.Join(args, " "), err, errOut.String())
	}
	return out.String()
}
