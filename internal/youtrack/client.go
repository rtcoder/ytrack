package youtrack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

type Issue struct {
	ID         string `json:"id"`
	IDReadable string `json:"idReadable"`
	Summary    string `json:"summary"`
}

type IssueCustomField struct {
	Name  string `json:"name"`
	Value any    `json:"value"`
}

type ProjectIssue struct {
	ID           string             `json:"id"`
	IDReadable   string             `json:"idReadable"`
	Summary      string             `json:"summary"`
	Description  string             `json:"description"`
	CustomFields []IssueCustomField `json:"customFields"`
}

type IssueFilters struct {
	Status   string
	User     string
	Type     string
	Priority string
}

type IssueUpdate struct {
	Summary     string
	Description string
	Type        string
	AssigneeID  string
	Priority    string
}

type IssueComment struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Author User   `json:"author"`
}

type CreateIssueOptions struct {
	Type       string
	AssigneeID string
	Priority   string
	Version    string
}

type CommandResult struct {
	Issues   []Issue         `json:"issues"`
	Commands json.RawMessage `json:"commands"`
	Errors   json.RawMessage `json:"errors"`
}

type User struct {
	ID       string `json:"id"`
	Login    string `json:"login"`
	Name     string `json:"name"`
	FullName string `json:"fullName"`
	Email    string `json:"email"`
	Banned   bool   `json:"banned"`
}

type Project struct {
	ID        string `json:"id"`
	ShortName string `json:"shortName"`
	Name      string `json:"name"`
	Leader    User   `json:"leader"`
}

type NamedValue struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ProjectCustomField struct {
	ID    string `json:"id"`
	Field struct {
		Name string `json:"name"`
	} `json:"field"`
	Bundle struct {
		Values []NamedValue `json:"values"`
	} `json:"bundle"`
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		Token:      token,
		HTTPClient: http.DefaultClient,
	}
}

func (c *Client) CreateIssue(ctx context.Context, projectID, summary, description string) (Issue, []byte, error) {
	return c.CreateIssueWithOptions(ctx, projectID, summary, description, CreateIssueOptions{})
}

func (c *Client) CreateIssueWithOptions(ctx context.Context, projectID, summary, description string, opts CreateIssueOptions) (Issue, []byte, error) {
	payload := map[string]any{
		"project": map[string]string{"id": projectID},
		"summary": summary,
	}
	if description != "" {
		payload["description"] = description
	}
	customFields := createIssueCustomFields(opts)
	if len(customFields) > 0 {
		payload["customFields"] = customFields
	}

	var issue Issue
	raw, err := c.doJSON(ctx, http.MethodPost, "/api/issues?fields=id,idReadable,summary", payload)
	if err != nil {
		return Issue{}, nil, err
	}
	if err := json.Unmarshal(raw, &issue); err != nil {
		return Issue{}, raw, fmt.Errorf("parse create issue response: %w", err)
	}
	return issue, raw, nil
}

func (c *Client) SetStatus(ctx context.Context, issueID, status string) (CommandResult, []byte, error) {
	return c.ApplyCommand(ctx, issueID, "State "+status)
}

func (c *Client) GetIssue(ctx context.Context, issueID string) (ProjectIssue, []byte, error) {
	var issue ProjectIssue
	path := "/api/issues/" + url.PathEscape(issueID) + "?fields=id,idReadable,summary,description,customFields(name,value(name,login))"
	raw, err := c.doNoBody(ctx, http.MethodGet, path)
	if err != nil {
		return ProjectIssue{}, nil, err
	}
	if err := json.Unmarshal(raw, &issue); err != nil {
		return ProjectIssue{}, raw, fmt.Errorf("parse issue response: %w", err)
	}
	return issue, raw, nil
}

func (c *Client) UpdateIssue(ctx context.Context, issueID string, update IssueUpdate) (ProjectIssue, []byte, error) {
	payload := map[string]any{}
	if update.Summary != "" {
		payload["summary"] = update.Summary
	}
	if update.Description != "" {
		payload["description"] = update.Description
	}

	var customFields []map[string]any
	if update.Type != "" {
		customFields = append(customFields, enumIssueCustomField("Type", update.Type))
	}
	if update.AssigneeID != "" {
		customFields = append(customFields, userIssueCustomField("Assignee", update.AssigneeID))
	}
	if update.Priority != "" {
		customFields = append(customFields, enumIssueCustomField("Priority", update.Priority))
	}
	if len(customFields) > 0 {
		payload["customFields"] = customFields
	}

	var issue ProjectIssue
	path := "/api/issues/" + url.PathEscape(issueID) + "?fields=id,idReadable,summary,description,customFields(name,value(name,login))"
	raw, err := c.doJSON(ctx, http.MethodPost, path, payload)
	if err != nil {
		return ProjectIssue{}, nil, err
	}
	if err := json.Unmarshal(raw, &issue); err != nil {
		return ProjectIssue{}, raw, fmt.Errorf("parse update issue response: %w", err)
	}
	return issue, raw, nil
}

func (c *Client) AddIssueComment(ctx context.Context, issueID, text string) (IssueComment, []byte, error) {
	payload := map[string]any{"text": text}
	var comment IssueComment
	path := "/api/issues/" + url.PathEscape(issueID) + "/comments?fields=id,text,author(login)"
	raw, err := c.doJSON(ctx, http.MethodPost, path, payload)
	if err != nil {
		return IssueComment{}, nil, err
	}
	if err := json.Unmarshal(raw, &comment); err != nil {
		return IssueComment{}, raw, fmt.Errorf("parse add comment response: %w", err)
	}
	return comment, raw, nil
}

func (c *Client) ApplyCommand(ctx context.Context, issueID, command string) (CommandResult, []byte, error) {
	payload := map[string]any{
		"query": command,
		"issues": []map[string]string{
			{"idReadable": issueID},
		},
	}

	var result CommandResult
	raw, err := c.doJSON(ctx, http.MethodPost, "/api/commands?fields=issues(id,idReadable,summary),commands,errors", payload)
	if err != nil {
		return CommandResult{}, nil, err
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return CommandResult{}, raw, fmt.Errorf("parse command response: %w", err)
	}
	return result, raw, nil
}

func (c *Client) GetMe(ctx context.Context) (User, []byte, error) {
	var user User
	raw, err := c.doNoBody(ctx, http.MethodGet, "/api/users/me?fields=id,login,name,fullName,email")
	if err != nil {
		return User{}, nil, err
	}
	if err := json.Unmarshal(raw, &user); err != nil {
		return User{}, raw, fmt.Errorf("parse current user response: %w", err)
	}
	return user, raw, nil
}

func (c *Client) GetUser(ctx context.Context, userID string) (User, []byte, error) {
	var user User
	path := "/api/users/" + url.PathEscape(userID) + "?fields=id,login,name,fullName,email,banned"
	raw, err := c.doNoBody(ctx, http.MethodGet, path)
	if err != nil {
		return User{}, nil, err
	}
	if err := json.Unmarshal(raw, &user); err != nil {
		return User{}, raw, fmt.Errorf("parse user response: %w", err)
	}
	return user, raw, nil
}

func (c *Client) ListUsers(ctx context.Context, top, skip int) ([]User, []byte, error) {
	values := url.Values{}
	if skip > 0 {
		values.Set("$skip", fmt.Sprintf("%d", skip))
	}
	if top > 0 {
		values.Set("$top", fmt.Sprintf("%d", top))
	}
	values.Set("fields", "id,login,name,fullName,email,banned")

	var users []User
	raw, err := c.doNoBody(ctx, http.MethodGet, "/api/users?"+values.Encode())
	if err != nil {
		return nil, nil, err
	}
	if err := json.Unmarshal(raw, &users); err != nil {
		return nil, raw, fmt.Errorf("parse users response: %w", err)
	}
	return users, raw, nil
}

func (c *Client) ResolveUser(ctx context.Context, ref string) (User, error) {
	ref = strings.TrimSpace(ref)
	if strings.EqualFold(ref, "me") {
		user, _, err := c.GetMe(ctx)
		return user, err
	}
	if isUserID(ref) {
		user, _, err := c.GetUser(ctx, ref)
		return user, err
	}

	users, _, err := c.ListUsers(ctx, 100, 0)
	if err != nil {
		return User{}, err
	}

	var matches []User
	refLower := strings.ToLower(ref)
	for _, user := range users {
		if userMatches(user, refLower) {
			matches = append(matches, user)
		}
	}
	switch len(matches) {
	case 0:
		return User{}, fmt.Errorf("user %q not found", ref)
	case 1:
		return matches[0], nil
	default:
		return User{}, fmt.Errorf("ambiguous user %q, matches: %s", ref, formatUserMatches(matches))
	}
}

func (c *Client) CreateProject(ctx context.Context, name, shortName, leaderID, template string) (Project, []byte, error) {
	payload := map[string]any{
		"name":      name,
		"shortName": shortName,
		"leader":    map[string]string{"id": leaderID},
	}

	path := "/api/admin/projects?fields=id,shortName,name,leader(id,login,name)"
	if template != "" {
		values := url.Values{}
		values.Set("fields", "id,shortName,name,leader(id,login,name)")
		values.Set("template", template)
		path = "/api/admin/projects?" + values.Encode()
	}

	var project Project
	raw, err := c.doJSON(ctx, http.MethodPost, path, payload)
	if err != nil {
		return Project{}, nil, err
	}
	if err := json.Unmarshal(raw, &project); err != nil {
		return Project{}, raw, fmt.Errorf("parse create project response: %w", err)
	}
	return project, raw, nil
}

func (c *Client) ListProjectIssues(ctx context.Context, projectID string) ([]ProjectIssue, []byte, error) {
	var issues []ProjectIssue
	path := "/api/admin/projects/" + url.PathEscape(projectID) + "/issues?fields=id,idReadable,summary,customFields(name,value(name,login))"
	raw, err := c.doNoBody(ctx, http.MethodGet, path)
	if err != nil {
		return nil, nil, err
	}
	if err := json.Unmarshal(raw, &issues); err != nil {
		return nil, raw, fmt.Errorf("parse project issues response: %w", err)
	}
	return issues, raw, nil
}

func (c *Client) ListProjectIssuesFiltered(ctx context.Context, projectID string, filters IssueFilters) ([]ProjectIssue, []byte, error) {
	if filters == (IssueFilters{}) {
		return c.ListProjectIssues(ctx, projectID)
	}

	project, _, err := c.GetProject(ctx, projectID)
	if err != nil {
		return nil, nil, err
	}

	query := buildIssueQuery(project.ShortName, filters)
	values := url.Values{}
	values.Set("fields", "id,idReadable,summary,customFields(name,value(name,login))")
	values.Set("query", query)

	var issues []ProjectIssue
	raw, err := c.doNoBody(ctx, http.MethodGet, "/api/issues?"+values.Encode())
	if err != nil {
		return nil, nil, err
	}
	if err := json.Unmarshal(raw, &issues); err != nil {
		return nil, raw, fmt.Errorf("parse filtered issues response: %w", err)
	}
	return issues, raw, nil
}

func (c *Client) GetProject(ctx context.Context, projectID string) (Project, []byte, error) {
	var project Project
	path := "/api/admin/projects/" + url.PathEscape(projectID) + "?fields=id,shortName,name"
	raw, err := c.doNoBody(ctx, http.MethodGet, path)
	if err != nil {
		return Project{}, nil, err
	}
	if err := json.Unmarshal(raw, &project); err != nil {
		return Project{}, raw, fmt.Errorf("parse project response: %w", err)
	}
	return project, raw, nil
}

func (c *Client) ListProjectUsers(ctx context.Context, projectID string) ([]User, []byte, error) {
	var users []User
	path := "/api/admin/projects/" + url.PathEscape(projectID) + "/team/users?fields=id,login,name,fullName,email,banned"
	raw, err := c.doNoBody(ctx, http.MethodGet, path)
	if err != nil {
		return nil, nil, err
	}
	if err := json.Unmarshal(raw, &users); err != nil {
		return nil, raw, fmt.Errorf("parse project users response: %w", err)
	}
	return users, raw, nil
}

func (c *Client) ListProjectStatuses(ctx context.Context, projectID string) ([]NamedValue, []byte, error) {
	return c.listProjectFieldValues(ctx, projectID, "State")
}

func (c *Client) ListProjectTypes(ctx context.Context, projectID string) ([]NamedValue, []byte, error) {
	return c.listProjectFieldValues(ctx, projectID, "Type")
}

func (c *Client) ListProjectPriorities(ctx context.Context, projectID string) ([]NamedValue, []byte, error) {
	return c.listProjectFieldValues(ctx, projectID, "Priority")
}

func (c *Client) ListProjectVersions(ctx context.Context, projectID string) ([]NamedValue, []byte, error) {
	fields, raw, err := c.listProjectCustomFields(ctx, projectID)
	if err != nil {
		return nil, nil, err
	}
	var values []NamedValue
	for _, field := range fields {
		if field.Field.Name == "Fix versions" || field.Field.Name == "Affected versions" {
			values = append(values, field.Bundle.Values...)
		}
	}
	return values, raw, nil
}

func (c *Client) listProjectFieldValues(ctx context.Context, projectID, fieldName string) ([]NamedValue, []byte, error) {
	fields, raw, err := c.listProjectCustomFields(ctx, projectID)
	if err != nil {
		return nil, nil, err
	}
	for _, field := range fields {
		if field.Field.Name == fieldName {
			return field.Bundle.Values, raw, nil
		}
	}
	return nil, raw, nil
}

func (c *Client) listProjectCustomFields(ctx context.Context, projectID string) ([]ProjectCustomField, []byte, error) {
	var fields []ProjectCustomField
	path := "/api/admin/projects/" + url.PathEscape(projectID) + "/customFields?fields=id,field(name),bundle(values(id,name))"
	raw, err := c.doNoBody(ctx, http.MethodGet, path)
	if err != nil {
		return nil, nil, err
	}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, raw, fmt.Errorf("parse project custom fields response: %w", err)
	}
	return fields, raw, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	return c.doRequest(ctx, method, path, bytes.NewReader(body))
}

func (c *Client) doNoBody(ctx context.Context, method, path string) ([]byte, error) {
	return c.doRequest(ctx, method, path, nil)
}

func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("youtrack API error: status %d: %s", resp.StatusCode, formatAPIError(raw))
	}
	return raw, nil
}

var userIDPattern = regexp.MustCompile(`^\d+-\d+$`)

func isUserID(ref string) bool {
	return userIDPattern.MatchString(ref)
}

func userMatches(user User, refLower string) bool {
	return strings.Contains(strings.ToLower(user.Login), refLower) ||
		strings.Contains(strings.ToLower(user.Name), refLower) ||
		strings.Contains(strings.ToLower(user.FullName), refLower) ||
		strings.Contains(strings.ToLower(user.Email), refLower)
}

func formatUserMatches(users []User) string {
	parts := make([]string, 0, len(users))
	for _, user := range users {
		parts = append(parts, fmt.Sprintf("%s %s (%s)", user.ID, user.Login, displayUserName(user)))
	}
	return strings.Join(parts, ", ")
}

func displayUserName(user User) string {
	if user.FullName != "" {
		return user.FullName
	}
	return user.Name
}

func buildIssueQuery(projectShortName string, filters IssueFilters) string {
	parts := []string{"project: " + projectShortName}
	if filters.Status != "" {
		parts = append(parts, "State: {"+filters.Status+"}")
	}
	if filters.User != "" {
		parts = append(parts, "Assignee: {"+filters.User+"}")
	}
	if filters.Type != "" {
		parts = append(parts, "Type: {"+filters.Type+"}")
	}
	if filters.Priority != "" {
		parts = append(parts, "Priority: {"+filters.Priority+"}")
	}
	return strings.Join(parts, " ")
}

func enumIssueCustomField(name, value string) map[string]any {
	return map[string]any{
		"name":  name,
		"$type": "SingleEnumIssueCustomField",
		"value": map[string]any{
			"name":  value,
			"$type": "EnumBundleElement",
		},
	}
}

func userIssueCustomField(name, userID string) map[string]any {
	return map[string]any{
		"name":  name,
		"$type": "SingleUserIssueCustomField",
		"value": map[string]any{
			"id":    userID,
			"$type": "User",
		},
	}
}

func createIssueCustomFields(opts CreateIssueOptions) []map[string]any {
	var fields []map[string]any
	if opts.Type != "" {
		fields = append(fields, enumIssueCustomField("Type", opts.Type))
	}
	if opts.AssigneeID != "" {
		fields = append(fields, userIssueCustomField("Assignee", opts.AssigneeID))
	}
	if opts.Priority != "" {
		fields = append(fields, enumIssueCustomField("Priority", opts.Priority))
	}
	if opts.Version != "" {
		fields = append(fields, map[string]any{
			"name":  "Fix versions",
			"$type": "MultiVersionIssueCustomField",
			"value": []map[string]any{
				{
					"name":  opts.Version,
					"$type": "VersionBundleElement",
				},
			},
		})
	}
	return fields
}

func formatAPIError(raw []byte) string {
	trimmed := strings.TrimSpace(string(raw))
	var body struct {
		Description string `json:"error_description"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(raw, &body); err == nil {
		if body.Description != "" {
			return body.Description
		}
		if body.Error != "" {
			return body.Error
		}
	}
	return trimmed
}
