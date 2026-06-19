package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ── Domain types ────────────────────────────────────────────────────────────

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
	ID              string
	UserID          *string
	ChallengeID     string
	Status          SessionStatus
	AnonToken       *string
	ContainerID     *string
	ContainerHost   *string
	StartedAt       *time.Time
	LastActiveAt    *time.Time
	EndedAt         *time.Time
	DurationSeconds *int
	CreatedAt       time.Time
}

type SessionFileDiff struct {
	ID        string
	SessionID string
	FilePath  string
	Content   string
	SavedAt   time.Time
}

// ── Users ─────────────────────────────────────────────────────────────────────

// UpsertUser creates a user record if it doesn't exist, otherwise no-ops.
func (c *Client) UpsertUser(ctx context.Context, id, email string) error {
	_, err := c.pool.Exec(ctx, `
		INSERT INTO users (id, email) VALUES ($1, $2)
		ON CONFLICT (id) DO NOTHING
	`, id, email)
	if err != nil {
		return fmt.Errorf("upsert user: %w", err)
	}
	return nil
}

// ── Sessions ──────────────────────────────────────────────────────────────────

// CreatePrewarmSession inserts a new prewarming session with an anon token.
func (c *Client) CreatePrewarmSession(ctx context.Context, challengeID, anonToken, containerID, containerHost string) (*Session, error) {
	row := c.pool.QueryRow(ctx, `
		INSERT INTO sessions (challenge_id, anon_token, status, container_id, container_host)
		VALUES ($1, $2, 'prewarming', $3, $4)
		RETURNING id, user_id, challenge_id, status, anon_token, container_id,
		          container_host, started_at, last_active_at, ended_at,
		          duration_seconds, created_at
	`, challengeID, nullIfEmpty(anonToken), containerID, containerHost)

	return scanSession(row)
}

// PromoteSession promotes a prewarmed anon session to an authenticated session.
func (c *Client) PromoteSession(ctx context.Context, anonToken, userID string) (*Session, error) {
	now := time.Now().UTC()
	row := c.pool.QueryRow(ctx, `
		UPDATE sessions
		SET user_id = $1, status = 'active', started_at = $2, last_active_at = $2
		WHERE anon_token = $3 AND status = 'prewarming'
		RETURNING id, user_id, challenge_id, status, anon_token, container_id,
		          container_host, started_at, last_active_at, ended_at,
		          duration_seconds, created_at
	`, userID, now, anonToken)

	session, err := scanSession(row)
	if err != nil {
		return nil, fmt.Errorf("no prewarming session found for anon_token: %w", err)
	}
	return session, nil
}

// GetSessionByID retrieves a session by its UUID.
func (c *Client) GetSessionByID(ctx context.Context, sessionID string) (*Session, error) {
	row := c.pool.QueryRow(ctx, `
		SELECT id, user_id, challenge_id, status, anon_token, container_id,
		       container_host, started_at, last_active_at, ended_at,
		       duration_seconds, created_at
		FROM sessions WHERE id = $1
	`, sessionID)

	session, err := scanSession(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return session, nil
}

// UpdateSessionStatus updates a session's status and optionally ended_at/duration.
func (c *Client) UpdateSessionStatus(ctx context.Context, sessionID string, status SessionStatus, endedAt *time.Time, durationSecs *int) error {
	_, err := c.pool.Exec(ctx, `
		UPDATE sessions SET status = $1, ended_at = $2, duration_seconds = $3
		WHERE id = $4
	`, status, endedAt, durationSecs, sessionID)
	if err != nil {
		return fmt.Errorf("update session status: %w", err)
	}
	return nil
}

// TouchSession updates last_active_at (called on file write / terminal activity).
func (c *Client) TouchSession(ctx context.Context, sessionID string) error {
	_, err := c.pool.Exec(ctx, `
		UPDATE sessions SET last_active_at = now() WHERE id = $1
	`, sessionID)
	if err != nil {
		return fmt.Errorf("touch session: %w", err)
	}
	return nil
}

// GetActiveSessions returns all sessions with status=active.
func (c *Client) GetActiveSessions(ctx context.Context) ([]Session, error) {
	rows, err := c.pool.Query(ctx, `
		SELECT id, user_id, challenge_id, status, anon_token, container_id,
		       container_host, started_at, last_active_at, ended_at,
		       duration_seconds, created_at
		FROM sessions WHERE status = 'active'
	`)
	if err != nil {
		return nil, fmt.Errorf("query active sessions: %w", err)
	}
	defer rows.Close()
	return scanSessions(rows)
}

// GetPrewarmingSessions returns all sessions with status=prewarming.
func (c *Client) GetPrewarmingSessions(ctx context.Context) ([]Session, error) {
	rows, err := c.pool.Query(ctx, `
		SELECT id, user_id, challenge_id, status, anon_token, container_id,
		       container_host, started_at, last_active_at, ended_at,
		       duration_seconds, created_at
		FROM sessions WHERE status = 'prewarming'
	`)
	if err != nil {
		return nil, fmt.Errorf("query prewarming sessions: %w", err)
	}
	defer rows.Close()
	return scanSessions(rows)
}

// ── File diffs ────────────────────────────────────────────────────────────────

// UpsertFileDiff saves (or replaces) the full content of a modified file.
func (c *Client) UpsertFileDiff(ctx context.Context, sessionID, filePath, content string) error {
	_, err := c.pool.Exec(ctx, `
		INSERT INTO session_file_diffs (session_id, file_path, content, saved_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (session_id, file_path)
		DO UPDATE SET content = EXCLUDED.content, saved_at = now()
	`, sessionID, filePath, content)
	if err != nil {
		return fmt.Errorf("upsert file diff: %w", err)
	}
	return nil
}

// GetFileDiffs retrieves all saved file contents for a session.
func (c *Client) GetFileDiffs(ctx context.Context, sessionID string) ([]SessionFileDiff, error) {
	rows, err := c.pool.Query(ctx, `
		SELECT id, session_id, file_path, content, saved_at
		FROM session_file_diffs WHERE session_id = $1
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("query file diffs: %w", err)
	}
	defer rows.Close()

	var diffs []SessionFileDiff
	for rows.Next() {
		var d SessionFileDiff
		if err := rows.Scan(&d.ID, &d.SessionID, &d.FilePath, &d.Content, &d.SavedAt); err != nil {
			return nil, fmt.Errorf("scan file diff: %w", err)
		}
		diffs = append(diffs, d)
	}
	return diffs, rows.Err()
}

// ── Scan helpers ──────────────────────────────────────────────────────────────

func scanSession(row pgx.Row) (*Session, error) {
	var s Session
	err := row.Scan(
		&s.ID, &s.UserID, &s.ChallengeID, &s.Status, &s.AnonToken,
		&s.ContainerID, &s.ContainerHost, &s.StartedAt, &s.LastActiveAt,
		&s.EndedAt, &s.DurationSeconds, &s.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func scanSessions(rows pgx.Rows) ([]Session, error) {
	var sessions []Session
	for rows.Next() {
		s, err := scanSessionFromRows(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, *s)
	}
	return sessions, rows.Err()
}

func scanSessionFromRows(rows pgx.Rows) (*Session, error) {
	var s Session
	err := rows.Scan(
		&s.ID, &s.UserID, &s.ChallengeID, &s.Status, &s.AnonToken,
		&s.ContainerID, &s.ContainerHost, &s.StartedAt, &s.LastActiveAt,
		&s.EndedAt, &s.DurationSeconds, &s.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
