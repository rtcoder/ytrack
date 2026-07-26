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

func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		Token:      token,
		HTTPClient: http.DefaultClient,
	}
}

func (c *Client) CreateIssue(ctx context.Context, projectID, summary, description string) (Issue, []byte, error) {
	payload := map[string]any{
		"project": map[string]string{"id": projectID},
		"summary": summary,
	}
	if description != "" {
		payload["description"] = description
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
	payload := map[string]any{
		"query": "State " + status,
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
		return CommandResult{}, raw, fmt.Errorf("parse set status response: %w", err)
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
		return nil, fmt.Errorf("youtrack API error: status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
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
