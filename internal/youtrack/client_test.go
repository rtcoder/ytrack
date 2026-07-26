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

func TestListProjectIssuesRequestsProjectIssues(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		_, _ = w.Write([]byte(`[{"id":"3-1","idReadable":"YR-14","summary":"Add init","customFields":[{"name":"State","value":{"name":"Submitted","$type":"StateBundleElement"},"$type":"StateIssueCustomField"}],"$type":"Issue"}]`))
	}))
	defer server.Close()

	issues, raw, err := NewClient(server.URL, "perm:token").ListProjectIssues(context.Background(), "0-3")
	if err != nil {
		t.Fatalf("ListProjectIssues() error = %v", err)
	}

	wantPath := "/api/admin/projects/0-3/issues?fields=id,idReadable,summary,customFields(name,value(name,login))"
	if gotPath != wantPath {
		t.Fatalf("path = %q, want %q", gotPath, wantPath)
	}
	if len(issues) != 1 || issues[0].IDReadable != "YR-14" || issues[0].Summary != "Add init" || string(raw) == "" {
		t.Fatalf("ListProjectIssues() issues=%+v raw=%q, want parsed issues and raw JSON", issues, string(raw))
	}
}

func TestListProjectIssuesWithFiltersSearchesByProjectShortName(t *testing.T) {
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

	filters := IssueFilters{
		Status:   "Submitted",
		User:     "me",
		Type:     "Bug",
		Priority: "Normal",
	}
	issues, raw, err := NewClient(server.URL, "perm:token").ListProjectIssuesFiltered(context.Background(), "0-3", filters)
	if err != nil {
		t.Fatalf("ListProjectIssuesFiltered() error = %v", err)
	}

	wantProjectPath := "/api/admin/projects/0-3?fields=id,shortName,name"
	wantIssuesPath := "/api/issues?fields=id%2CidReadable%2Csummary%2CcustomFields%28name%2Cvalue%28name%2Clogin%29%29&query=project%3A+YR+State%3A+%7BSubmitted%7D+Assignee%3A+%7Bme%7D+Type%3A+%7BBug%7D+Priority%3A+%7BNormal%7D"
	if len(requested) != 2 || requested[0] != wantProjectPath || requested[1] != wantIssuesPath {
		t.Fatalf("requested = %#v, want project lookup then filtered issues", requested)
	}
	if len(issues) != 1 || issues[0].IDReadable != "YR-14" || string(raw) == "" {
		t.Fatalf("ListProjectIssuesFiltered() issues=%+v raw=%q, want parsed issues and raw JSON", issues, string(raw))
	}
}

func TestListProjectUsersRequestsProjectTeamUsers(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		_, _ = w.Write([]byte(`[{"id":"1-2","login":"rtcoder","name":"Robert","fullName":"Robert","email":"robert@example.com","banned":false,"$type":"User"}]`))
	}))
	defer server.Close()

	users, raw, err := NewClient(server.URL, "perm:token").ListProjectUsers(context.Background(), "0-3")
	if err != nil {
		t.Fatalf("ListProjectUsers() error = %v", err)
	}

	wantPath := "/api/admin/projects/0-3/team/users?fields=id,login,name,fullName,email,banned"
	if gotPath != wantPath {
		t.Fatalf("path = %q, want %q", gotPath, wantPath)
	}
	if len(users) != 1 || users[0].Login != "rtcoder" || string(raw) == "" {
		t.Fatalf("ListProjectUsers() users=%+v raw=%q, want parsed users and raw JSON", users, string(raw))
	}
}

func TestGetIssueRequestsIssueDetails(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		_, _ = w.Write([]byte(`{"id":"3-1","idReadable":"YR-14","summary":"Add init","description":"Interactive setup","customFields":[{"name":"State","value":{"name":"Submitted","$type":"StateBundleElement"},"$type":"StateIssueCustomField"},{"name":"Assignee","value":{"login":"rtcoder","name":"Robert","$type":"User"},"$type":"SingleUserIssueCustomField"},{"name":"Priority","value":{"name":"Normal","$type":"EnumBundleElement"},"$type":"SingleEnumIssueCustomField"}],"$type":"Issue"}`))
	}))
	defer server.Close()

	issue, raw, err := NewClient(server.URL, "perm:token").GetIssue(context.Background(), "YR-14")
	if err != nil {
		t.Fatalf("GetIssue() error = %v", err)
	}

	wantPath := "/api/issues/YR-14?fields=id,idReadable,summary,description,customFields(name,value(name,login))"
	if gotPath != wantPath {
		t.Fatalf("path = %q, want %q", gotPath, wantPath)
	}
	if issue.IDReadable != "YR-14" || issue.Summary != "Add init" || issue.Description != "Interactive setup" || string(raw) == "" {
		t.Fatalf("GetIssue() issue=%+v raw=%q, want parsed issue and raw JSON", issue, string(raw))
	}
}

func TestListProjectBundleValuesFiltersCustomFieldBundles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RequestURI() != "/api/admin/projects/0-3/customFields?fields=id,field(name),bundle(values(id,name))" {
			t.Fatalf("path = %q, want project custom fields", r.URL.RequestURI())
		}
		_, _ = w.Write([]byte(`[
			{"id":"186-13","field":{"name":"State","$type":"CustomField"},"bundle":{"values":[{"id":"162-0","name":"Submitted","$type":"StateBundleElement"},{"id":"162-7","name":"Fixed","$type":"StateBundleElement"}],"$type":"StateBundle"},"$type":"StateProjectCustomField"},
			{"id":"186-12","field":{"name":"Type","$type":"CustomField"},"bundle":{"values":[{"id":"160-5","name":"Bug","$type":"EnumBundleElement"}],"$type":"EnumBundle"},"$type":"EnumProjectCustomField"},
			{"id":"186-11","field":{"name":"Priority","$type":"CustomField"},"bundle":{"values":[{"id":"160-3","name":"Normal","$type":"EnumBundleElement"}],"$type":"EnumBundle"},"$type":"EnumProjectCustomField"},
			{"id":"186-15","field":{"name":"Fix versions","$type":"CustomField"},"bundle":{"values":[{"id":"170-1","name":"v0.1.7","$type":"VersionBundleElement"}],"$type":"VersionBundle"},"$type":"VersionProjectCustomField"},
			{"id":"186-16","field":{"name":"Affected versions","$type":"CustomField"},"bundle":{"values":[{"id":"170-2","name":"v0.1.6","$type":"VersionBundleElement"}],"$type":"VersionBundle"},"$type":"VersionProjectCustomField"}
		]`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "perm:token")
	statuses, rawStatuses, err := client.ListProjectStatuses(context.Background(), "0-3")
	if err != nil {
		t.Fatalf("ListProjectStatuses() error = %v", err)
	}
	types, _, err := client.ListProjectTypes(context.Background(), "0-3")
	if err != nil {
		t.Fatalf("ListProjectTypes() error = %v", err)
	}
	priorities, _, err := client.ListProjectPriorities(context.Background(), "0-3")
	if err != nil {
		t.Fatalf("ListProjectPriorities() error = %v", err)
	}
	versions, _, err := client.ListProjectVersions(context.Background(), "0-3")
	if err != nil {
		t.Fatalf("ListProjectVersions() error = %v", err)
	}

	if len(statuses) != 2 || statuses[1].Name != "Fixed" || string(rawStatuses) == "" {
		t.Fatalf("statuses=%+v raw=%q, want State bundle values", statuses, string(rawStatuses))
	}
	if len(types) != 1 || types[0].Name != "Bug" {
		t.Fatalf("types=%+v, want Type bundle values", types)
	}
	if len(priorities) != 1 || priorities[0].Name != "Normal" {
		t.Fatalf("priorities=%+v, want Priority bundle values", priorities)
	}
	if len(versions) != 2 || versions[0].Name != "v0.1.7" || versions[1].Name != "v0.1.6" {
		t.Fatalf("versions=%+v, want Fix and Affected version values", versions)
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
