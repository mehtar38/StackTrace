// Package supabase provides a thin REST client for the Supabase PostgREST API.
// The orchestrator uses the service role key, which bypasses RLS — this is
// intentional because the orchestrator is a trusted backend service.
package supabase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client wraps the Supabase PostgREST REST API.
type Client struct {
	baseURL        string
	serviceRoleKey string
	httpClient     *http.Client
}

// NewClient constructs a Supabase client.
// baseURL: e.g. "https://your-project.supabase.co"
// serviceRoleKey: the service_role JWT (bypasses RLS)
func NewClient(baseURL, serviceRoleKey string) *Client {
	return &Client{
		baseURL:        strings.TrimRight(baseURL, "/"),
		serviceRoleKey: serviceRoleKey,
		httpClient:     &http.Client{Timeout: 10 * time.Second},
	}
}

// --- Domain types (mirror the SQL schema) ---

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SessionStatus string

const (
	SessionStatusPrewarming SessionStatus = "prewarming"
	SessionStatusActive     SessionStatus = "active"
	SessionStatusExited     SessionStatus = "exited"
	SessionStatusCompleted  SessionStatus = "completed"
	SessionStatusExpired    SessionStatus = "expired"
	SessionStatusError      SessionStatus = "error"
)

type Session struct {
	ID              string        `json:"id"`
	UserID          *string       `json:"user_id"`
	ChallengeID     string        `json:"challenge_id"`
	Status          SessionStatus `json:"status"`
	AnonToken       *string       `json:"anon_token"`
	ContainerID     *string       `json:"container_id"`
	ContainerHost   *string       `json:"container_host"`
	StartedAt       *time.Time    `json:"started_at"`
	LastActiveAt    *time.Time    `json:"last_active_at"`
	EndedAt         *time.Time    `json:"ended_at"`
	DurationSeconds *int          `json:"duration_seconds"`
	CreatedAt       time.Time     `json:"created_at"`
}

type SessionFileDiff struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	FilePath  string    `json:"file_path"`
	Content   string    `json:"content"`
	SavedAt   time.Time `json:"saved_at"`
}

// --- User operations ---

// UpsertUser creates a user record if it doesn't exist, otherwise does nothing.
// Called on every authenticated request so we always have a user row.
func (c *Client) UpsertUser(ctx context.Context, id, email string) error {
	body := map[string]string{"id": id, "email": email}
	return c.post(ctx, "/rest/v1/users", body, map[string]string{
		"Prefer": "resolution=ignore-duplicates,return=minimal",
	})
}

// GetUser retrieves a user by ID. Returns nil if not found.
func (c *Client) GetUser(ctx context.Context, id string) (*User, error) {
	var users []User
	err := c.get(ctx, "/rest/v1/users", map[string]string{"id": "eq." + id, "limit": "1"}, &users)
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, nil
	}
	return &users[0], nil
}

// --- Session operations ---

// CreatePrewarmSession inserts a new prewarming session with an anon token.
func (c *Client) CreatePrewarmSession(ctx context.Context, challengeID, anonToken, containerID, containerHost string) (*Session, error) {
	body := map[string]interface{}{
		"challenge_id":   challengeID,
		"anon_token":     anonToken,
		"status":         SessionStatusPrewarming,
		"container_id":   containerID,
		"container_host": containerHost,
	}
	var sessions []Session
	err := c.postReturning(ctx, "/rest/v1/sessions", body, &sessions)
	if err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, fmt.Errorf("no session returned from insert")
	}
	return &sessions[0], nil
}

// PromoteSession promotes a prewarmed anon session to an authenticated session.
// Sets user_id, status=active, and started_at.
func (c *Client) PromoteSession(ctx context.Context, anonToken, userID string) (*Session, error) {
	now := time.Now().UTC()
	body := map[string]interface{}{
		"user_id":        userID,
		"status":         SessionStatusActive,
		"started_at":     now,
		"last_active_at": now,
	}
	var sessions []Session
	err := c.patch(ctx, "/rest/v1/sessions", map[string]string{"anon_token": "eq." + anonToken}, body, &sessions)
	if err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, fmt.Errorf("no session found for anon_token %s", anonToken)
	}
	return &sessions[0], nil
}

// GetSessionByID retrieves a session by its UUID.
func (c *Client) GetSessionByID(ctx context.Context, sessionID string) (*Session, error) {
	var sessions []Session
	err := c.get(ctx, "/rest/v1/sessions", map[string]string{"id": "eq." + sessionID, "limit": "1"}, &sessions)
	if err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, nil
	}
	return &sessions[0], nil
}

// UpdateSessionStatus updates a session's status and optionally sets ended_at + duration.
func (c *Client) UpdateSessionStatus(ctx context.Context, sessionID string, status SessionStatus, endedAt *time.Time, durationSecs *int) error {
	body := map[string]interface{}{"status": status}
	if endedAt != nil {
		body["ended_at"] = endedAt
	}
	if durationSecs != nil {
		body["duration_seconds"] = durationSecs
	}
	return c.patchNoReturn(ctx, "/rest/v1/sessions", map[string]string{"id": "eq." + sessionID}, body)
}

// TouchSession updates last_active_at on a session (called on file write / terminal activity).
func (c *Client) TouchSession(ctx context.Context, sessionID string) error {
	now := time.Now().UTC()
	body := map[string]interface{}{"last_active_at": now}
	return c.patchNoReturn(ctx, "/rest/v1/sessions", map[string]string{"id": "eq." + sessionID}, body)
}

// GetActiveSessions returns all sessions with status=active, used by the inactivity reaper.
func (c *Client) GetActiveSessions(ctx context.Context) ([]Session, error) {
	var sessions []Session
	err := c.get(ctx, "/rest/v1/sessions", map[string]string{"status": "eq.active"}, &sessions)
	return sessions, err
}

// GetPrewarmingSessions returns all sessions with status=prewarming.
func (c *Client) GetPrewarmingSessions(ctx context.Context) ([]Session, error) {
	var sessions []Session
	err := c.get(ctx, "/rest/v1/sessions", map[string]string{"status": "eq.prewarming"}, &sessions)
	return sessions, err
}

// --- File diff operations ---

// UpsertFileDiff saves (or replaces) the full content of a modified file for a session.
func (c *Client) UpsertFileDiff(ctx context.Context, sessionID, filePath, content string) error {
	body := map[string]interface{}{
		"session_id": sessionID,
		"file_path":  filePath,
		"content":    content,
		"saved_at":   time.Now().UTC(),
	}
	return c.post(ctx, "/rest/v1/session_file_diffs", body, map[string]string{
		"Prefer": "resolution=merge-duplicates,return=minimal",
	})
}

// GetFileDiffs retrieves all saved file contents for a session (used on resume).
func (c *Client) GetFileDiffs(ctx context.Context, sessionID string) ([]SessionFileDiff, error) {
	var diffs []SessionFileDiff
	err := c.get(ctx, "/rest/v1/session_file_diffs", map[string]string{"session_id": "eq." + sessionID}, &diffs)
	return diffs, err
}

// --- HTTP helpers ---

func (c *Client) get(ctx context.Context, path string, queryParams map[string]string, out interface{}) error {
	url := c.baseURL + path
	if len(queryParams) > 0 {
		parts := make([]string, 0, len(queryParams))
		for k, v := range queryParams {
			parts = append(parts, k+"="+v)
		}
		url += "?" + strings.Join(parts, "&")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	c.setHeaders(req, nil)

	return c.execute(req, out)
}

func (c *Client) post(ctx context.Context, path string, body interface{}, extraHeaders map[string]string) error {
	return c.postReturningRaw(ctx, path, body, extraHeaders, nil)
}

func (c *Client) postReturning(ctx context.Context, path string, body interface{}, out interface{}) error {
	return c.postReturningRaw(ctx, path, body, map[string]string{"Prefer": "return=representation"}, out)
}

func (c *Client) postReturningRaw(ctx context.Context, path string, body interface{}, extraHeaders map[string]string, out interface{}) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	c.setHeaders(req, extraHeaders)
	return c.execute(req, out)
}

func (c *Client) patch(ctx context.Context, path string, queryParams map[string]string, body interface{}, out interface{}) error {
	return c.patchRaw(ctx, path, queryParams, body, map[string]string{"Prefer": "return=representation"}, out)
}

func (c *Client) patchNoReturn(ctx context.Context, path string, queryParams map[string]string, body interface{}) error {
	return c.patchRaw(ctx, path, queryParams, body, map[string]string{"Prefer": "return=minimal"}, nil)
}

func (c *Client) patchRaw(ctx context.Context, path string, queryParams map[string]string, body interface{}, extraHeaders map[string]string, out interface{}) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}

	url := c.baseURL + path
	if len(queryParams) > 0 {
		parts := make([]string, 0, len(queryParams))
		for k, v := range queryParams {
			parts = append(parts, k+"="+v)
		}
		url += "?" + strings.Join(parts, "&")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	c.setHeaders(req, extraHeaders)
	return c.execute(req, out)
}

func (c *Client) setHeaders(req *http.Request, extra map[string]string) {
	req.Header.Set("apikey", c.serviceRoleKey)
	req.Header.Set("Authorization", "Bearer "+c.serviceRoleKey)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range extra {
		req.Header.Set(k, v)
	}
}

func (c *Client) execute(req *http.Request, out interface{}) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("supabase request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("supabase HTTP %d: %s", resp.StatusCode, string(raw))
	}

	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}
	return nil
}
