// Package redis provides an Upstash Redis client using the REST API.
// We deliberately avoid TCP Redis clients because the orchestrator runs on
// Azure Container Apps (serverless), where persistent TCP connections are
// unreliable. Every operation is a simple HTTPS request.
package redis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a minimal Upstash Redis REST client covering only the operations
// the orchestrator needs: SET (with EX), GET, DEL, and SADD/SREM/SMEMBERS
// for the dirty-file tracking set.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewUpstashClient constructs a Client.
// baseURL: the Upstash REST endpoint, e.g. "https://your-db.upstash.io"
// token:   the Upstash REST token (UPSTASH_REDIS_TOKEN env var)
func NewUpstashClient(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// upstashResponse is the envelope Upstash wraps all responses in.
type upstashResponse struct {
	Result interface{} `json:"result"`
	Error  string      `json:"error"`
}

// Set stores key=value with an optional TTL in seconds (0 = no expiry).
func (c *Client) Set(ctx context.Context, key, value string, ttlSecs int) error {
	var args []interface{}
	if ttlSecs > 0 {
		args = []interface{}{"SET", key, value, "EX", ttlSecs}
	} else {
		args = []interface{}{"SET", key, value}
	}
	_, err := c.do(ctx, args...)
	return err
}

// Get retrieves the value for key. Returns ("", nil) if the key does not exist.
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	result, err := c.do(ctx, "GET", key)
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", nil
	}
	s, ok := result.(string)
	if !ok {
		return "", fmt.Errorf("unexpected GET result type: %T", result)
	}
	return s, nil
}

// Del deletes one or more keys.
func (c *Client) Del(ctx context.Context, keys ...string) error {
	args := make([]interface{}, len(keys)+1)
	args[0] = "DEL"
	for i, k := range keys {
		args[i+1] = k
	}
	_, err := c.do(ctx, args...)
	return err
}

// SAdd adds members to a set. Creates the set if it does not exist.
func (c *Client) SAdd(ctx context.Context, key string, members ...string) error {
	args := make([]interface{}, len(members)+2)
	args[0] = "SADD"
	args[1] = key
	for i, m := range members {
		args[i+2] = m
	}
	_, err := c.do(ctx, args...)
	return err
}

// SMembers returns all members of a set. Returns an empty slice if key does not exist.
func (c *Client) SMembers(ctx context.Context, key string) ([]string, error) {
	result, err := c.do(ctx, "SMEMBERS", key)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return []string{}, nil
	}
	raw, ok := result.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected SMEMBERS result type: %T", result)
	}
	members := make([]string, len(raw))
	for i, v := range raw {
		members[i] = fmt.Sprintf("%v", v)
	}
	return members, nil
}

// Expire sets a TTL (seconds) on an existing key.
func (c *Client) Expire(ctx context.Context, key string, ttlSecs int) error {
	_, err := c.do(ctx, "EXPIRE", key, ttlSecs)
	return err
}

// SetJSON marshals v to JSON and stores it under key with optional TTL.
func (c *Client) SetJSON(ctx context.Context, key string, v interface{}, ttlSecs int) error {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return c.Set(ctx, key, string(b), ttlSecs)
}

// GetJSON retrieves key and unmarshals the JSON value into v.
// Returns (false, nil) if the key does not exist.
func (c *Client) GetJSON(ctx context.Context, key string, v interface{}) (bool, error) {
	s, err := c.Get(ctx, key)
	if err != nil {
		return false, err
	}
	if s == "" {
		return false, nil
	}
	if err := json.Unmarshal([]byte(s), v); err != nil {
		return false, fmt.Errorf("unmarshal: %w", err)
	}
	return true, nil
}

// do executes a Redis command via the Upstash REST API and returns the raw result.
// The Upstash REST API accepts commands as JSON arrays POSTed to the base URL.
func (c *Client) do(ctx context.Context, args ...interface{}) (interface{}, error) {
	body, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("marshal command: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstash HTTP %d: %s", resp.StatusCode, string(raw))
	}

	var envelope upstashResponse
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if envelope.Error != "" {
		return nil, fmt.Errorf("redis error: %s", envelope.Error)
	}

	return envelope.Result, nil
}

// PipelineURL returns the URL for multi-command pipeline requests.
// Not used by the basic client but available for future batch operations.
func (c *Client) pipelineURL() string {
	u, _ := url.Parse(c.baseURL)
	u.Path = "/pipeline"
	return u.String()
}
