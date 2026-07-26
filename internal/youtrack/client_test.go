package youtrack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateIssuePostsExpectedPayloadAndReturnsIssue(t *testing.T) {
	var gotPath, gotAuth, gotContentType string
	var gotPayload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"2-1","idReadable":"ART-123","summary":"Crash on save"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "perm:token")
	issue, raw, err := client.CreateIssue(context.Background(), "0-1", "Crash on save", "Steps to reproduce")
	if err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}

	if gotPath != "/api/issues?fields=id,idReadable,summary" {
		t.Fatalf("path = %q, want issue create endpoint", gotPath)
	}
	if gotAuth != "Bearer perm:token" {
		t.Fatalf("Authorization = %q, want bearer token", gotAuth)
	}
	if gotContentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", gotContentType)
	}
	if gotPayload["summary"] != "Crash on save" || gotPayload["description"] != "Steps to reproduce" {
		t.Fatalf("payload = %#v, want summary and description", gotPayload)
	}
	project := gotPayload["project"].(map[string]any)
	if project["id"] != "0-1" {
		t.Fatalf("project id = %q, want 0-1", project["id"])
	}
	if issue.IDReadable != "ART-123" || string(raw) == "" {
		t.Fatalf("CreateIssue() issue=%+v raw=%q, want parsed issue and raw JSON", issue, string(raw))
	}
}

func TestCreateIssueOmitsEmptyDescription(t *testing.T) {
	var gotPayload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		_, _ = w.Write([]byte(`{"id":"2-1","idReadable":"ART-123","summary":"Crash on save"}`))
	}))
	defer server.Close()

	_, _, err := NewClient(server.URL, "perm:token").CreateIssue(context.Background(), "0-1", "Crash on save", "")
	if err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}
	if _, ok := gotPayload["description"]; ok {
		t.Fatalf("payload has description for empty input: %#v", gotPayload)
	}
}

func TestSetStatusPostsCommandPayload(t *testing.T) {
	var gotPath, gotAuth string
	var gotPayload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		_, _ = w.Write([]byte(`{"issues":[{"id":"2-1","idReadable":"ART-123","summary":"Crash on save"}],"commands":"State Done","errors":[]}`))
	}))
	defer server.Close()

	result, raw, err := NewClient(server.URL, "perm:token").SetStatus(context.Background(), "ART-123", "Done")
	if err != nil {
		t.Fatalf("SetStatus() error = %v", err)
	}

	if gotPath != "/api/commands?fields=issues(id,idReadable,summary),commands,errors" {
		t.Fatalf("path = %q, want command endpoint", gotPath)
	}
	if gotAuth != "Bearer perm:token" {
		t.Fatalf("Authorization = %q, want bearer token", gotAuth)
	}
	if gotPayload["query"] != "State Done" {
		t.Fatalf("query = %q, want State Done", gotPayload["query"])
	}
	issues := gotPayload["issues"].([]any)
	first := issues[0].(map[string]any)
	if first["idReadable"] != "ART-123" {
		t.Fatalf("idReadable = %q, want ART-123", first["idReadable"])
	}
	if len(result.Issues) != 1 || string(raw) == "" {
		t.Fatalf("SetStatus() result=%+v raw=%q, want parsed response and raw JSON", result, string(raw))
	}
}

func TestClientReturnsHTTPErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad token", http.StatusUnauthorized)
	}))
	defer server.Close()

	_, _, err := NewClient(server.URL, "perm:token").CreateIssue(context.Background(), "0-1", "Crash", "")
	if err == nil {
		t.Fatal("CreateIssue() error = nil, want HTTP error")
	}
	want := "youtrack API error: status 401: bad token"
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}
