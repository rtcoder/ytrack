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

func TestGetMeRequestsCurrentUserProfile(t *testing.T) {
	var gotPath, gotAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"id":"1-2","login":"rtcoder","name":"Robert","fullName":"Robert","email":"robert@example.com","$type":"Me"}`))
	}))
	defer server.Close()

	user, raw, err := NewClient(server.URL, "perm:token").GetMe(context.Background())
	if err != nil {
		t.Fatalf("GetMe() error = %v", err)
	}

	wantPath := "/api/users/me?fields=id,login,name,fullName,email"
	if gotPath != wantPath {
		t.Fatalf("path = %q, want %q", gotPath, wantPath)
	}
	if gotAuth != "Bearer perm:token" {
		t.Fatalf("Authorization = %q, want bearer token", gotAuth)
	}
	if user.ID != "1-2" || user.Login != "rtcoder" || user.FullName != "Robert" || user.Email != "robert@example.com" || string(raw) == "" {
		t.Fatalf("GetMe() user=%+v raw=%q, want parsed user and raw JSON", user, string(raw))
	}
}

func TestListUsersRequestsLimitedUserFields(t *testing.T) {
	var gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		_, _ = w.Write([]byte(`[{"id":"24-55","login":"l.downton","name":"Luisa Downton","fullName":"Luisa Downton","email":"luisa@example.com","banned":false,"$type":"User"}]`))
	}))
	defer server.Close()

	users, raw, err := NewClient(server.URL, "perm:token").ListUsers(context.Background(), 20, 5)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}

	wantPath := "/api/users?%24skip=5&%24top=20&fields=id%2Clogin%2Cname%2CfullName%2Cemail%2Cbanned"
	if gotPath != wantPath {
		t.Fatalf("path = %q, want %q", gotPath, wantPath)
	}
	if len(users) != 1 || users[0].ID != "24-55" || users[0].Login != "l.downton" || users[0].Banned {
		t.Fatalf("ListUsers() users=%+v, want parsed active user", users)
	}
	if string(raw) == "" {
		t.Fatalf("ListUsers() raw = empty, want raw JSON")
	}
}

func TestGetUserRequestsSpecificUserProfile(t *testing.T) {
	var gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		_, _ = w.Write([]byte(`{"id":"24-55","login":"l.downton","name":"Luisa Downton","fullName":"Luisa Downton","email":"luisa@example.com","banned":false,"$type":"User"}`))
	}))
	defer server.Close()

	user, raw, err := NewClient(server.URL, "perm:token").GetUser(context.Background(), "24-55")
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}

	wantPath := "/api/users/24-55?fields=id,login,name,fullName,email,banned"
	if gotPath != wantPath {
		t.Fatalf("path = %q, want %q", gotPath, wantPath)
	}
	if user.ID != "24-55" || user.Login != "l.downton" || user.FullName != "Luisa Downton" || string(raw) == "" {
		t.Fatalf("GetUser() user=%+v raw=%q, want parsed user and raw JSON", user, string(raw))
	}
}

func TestResolveUserFindsMeIDAndUniqueLogin(t *testing.T) {
	var requested []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = append(requested, r.URL.RequestURI())
		switch r.URL.Path {
		case "/api/users/me":
			_, _ = w.Write([]byte(`{"id":"1-2","login":"rtcoder","name":"Robert","fullName":"Robert","email":"robert@example.com","$type":"Me"}`))
		case "/api/users/24-55":
			_, _ = w.Write([]byte(`{"id":"24-55","login":"l.downton","name":"Luisa Downton","fullName":"Luisa Downton","email":"luisa@example.com","banned":false,"$type":"User"}`))
		case "/api/users":
			_, _ = w.Write([]byte(`[
				{"id":"24-55","login":"l.downton","name":"Luisa Downton","fullName":"Luisa Downton","email":"luisa@example.com","banned":false,"$type":"User"},
				{"id":"24-56","login":"m.scott","name":"Michael Scott","fullName":"Michael Scott","email":"michael@example.com","banned":false,"$type":"User"}
			]`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "perm:token")
	me, err := client.ResolveUser(context.Background(), "me")
	if err != nil {
		t.Fatalf("ResolveUser(me) error = %v", err)
	}
	byID, err := client.ResolveUser(context.Background(), "24-55")
	if err != nil {
		t.Fatalf("ResolveUser(id) error = %v", err)
	}
	byLogin, err := client.ResolveUser(context.Background(), "m.scott")
	if err != nil {
		t.Fatalf("ResolveUser(login) error = %v", err)
	}

	if me.ID != "1-2" || byID.Login != "l.downton" || byLogin.ID != "24-56" {
		t.Fatalf("resolved users: me=%+v byID=%+v byLogin=%+v", me, byID, byLogin)
	}
	if len(requested) != 3 {
		t.Fatalf("requested paths = %#v, want one request per resolution", requested)
	}
}

func TestResolveUserReportsAmbiguousMatches(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
			{"id":"24-55","login":"l.downton","name":"Luisa Downton","fullName":"Luisa Downton","email":"luisa@example.com","banned":false,"$type":"User"},
			{"id":"24-56","login":"l.doe","name":"Luisa Doe","fullName":"Luisa Doe","email":"luisa.doe@example.com","banned":false,"$type":"User"}
		]`))
	}))
	defer server.Close()

	_, err := NewClient(server.URL, "perm:token").ResolveUser(context.Background(), "luisa")
	if err == nil {
		t.Fatal("ResolveUser() error = nil, want ambiguous user error")
	}
	want := `ambiguous user "luisa", matches: 24-55 l.downton (Luisa Downton), 24-56 l.doe (Luisa Doe)`
	if err.Error() != want {
		t.Fatalf("ResolveUser() error = %q, want %q", err.Error(), want)
	}
}

func TestCreateProjectPostsExpectedPayloadAndReturnsProject(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"id":"0-16","shortName":"MOB","name":"Mobile App","leader":{"id":"1-2","login":"rtcoder","name":"Robert","$type":"User"},"$type":"Project"}`))
	}))
	defer server.Close()

	project, raw, err := NewClient(server.URL, "perm:token").CreateProject(context.Background(), "Mobile App", "MOB", "1-2", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	wantPath := "/api/admin/projects?fields=id,shortName,name,leader(id,login,name)"
	if gotPath != wantPath {
		t.Fatalf("path = %q, want %q", gotPath, wantPath)
	}
	if gotAuth != "Bearer perm:token" {
		t.Fatalf("Authorization = %q, want bearer token", gotAuth)
	}
	if gotContentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", gotContentType)
	}
	if gotPayload["name"] != "Mobile App" || gotPayload["shortName"] != "MOB" {
		t.Fatalf("payload = %#v, want name and shortName", gotPayload)
	}
	leader := gotPayload["leader"].(map[string]any)
	if leader["id"] != "1-2" {
		t.Fatalf("leader id = %q, want 1-2", leader["id"])
	}
	if project.ID != "0-16" || project.ShortName != "MOB" || project.Name != "Mobile App" || project.Leader.ID != "1-2" || string(raw) == "" {
		t.Fatalf("CreateProject() project=%+v raw=%q, want parsed project and raw JSON", project, string(raw))
	}
}

func TestCreateProjectUsesTemplateQueryParameter(t *testing.T) {
	var gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		_, _ = w.Write([]byte(`{"id":"0-16","shortName":"MOB","name":"Mobile App","leader":{"id":"1-2","login":"rtcoder","name":"Robert","$type":"User"},"$type":"Project"}`))
	}))
	defer server.Close()

	_, _, err := NewClient(server.URL, "perm:token").CreateProject(context.Background(), "Mobile App", "MOB", "1-2", "kanban")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	wantPath := "/api/admin/projects?fields=id%2CshortName%2Cname%2Cleader%28id%2Clogin%2Cname%29&template=kanban"
	if gotPath != wantPath {
		t.Fatalf("path = %q, want %q", gotPath, wantPath)
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
