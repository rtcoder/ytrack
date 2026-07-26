package youtrack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

func (c *Client) doJSON(ctx context.Context, method, path string, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
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
