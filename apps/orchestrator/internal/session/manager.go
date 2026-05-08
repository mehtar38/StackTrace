// Package session manages the full lifecycle of a debugging session:
// prewarm → promote → active → (exit | expire) → cleanup.
package session

import (
	"context"
	"fmt"
	"log/slog"
	"stacktrace/orchestrator/internal/redis"
	"stacktrace/orchestrator/internal/sandbox"
	"stacktrace/orchestrator/internal/supabase"
	"time"
)

// redisSessionKey returns the Redis key for a session's state.
func redisSessionKey(sessionID string) string {
	return "session:" + sessionID + ":state"
}

// redisAnonKey returns the Redis key mapping an anon token to a session ID.
func redisAnonKey(anonToken string) string {
	return "anon:" + anonToken + ":session_id"
}

// redisDirtyFilesKey returns the Redis key for the set of modified file paths.
func redisDirtyFilesKey(sessionID string) string {
	return "session:" + sessionID + ":dirty_files"
}

// RedisSessionState is the in-memory session state stored in Redis.
// It is the fast-path for all hot operations (file writes, terminal pings).
// Supabase is the source of truth; Redis is the cache.
type RedisSessionState struct {
	SessionID     string `json:"session_id"`
	UserID        string `json:"user_id"`
	ChallengeID   string `json:"challenge_id"`
	ContainerID   string `json:"container_id"`
	ContainerHost string `json:"container_host"`
	Status        string `json:"status"`
	StartedAt     int64  `json:"started_at"`     // Unix timestamp
	LastActiveAt  int64  `json:"last_active_at"` // Unix timestamp
}

// ManagerDeps holds all dependencies injected into the session Manager.
type ManagerDeps struct {
	Provider       sandbox.Provider
	Redis          *redis.Client
	Supabase       *supabase.Client
	InactivitySecs int
	HardLimitSecs  int
}

// Manager orchestrates session lifecycle.
type Manager struct {
	deps ManagerDeps
}

// NewManager constructs a Manager.
func NewManager(deps ManagerDeps) *Manager {
	return &Manager{deps: deps}
}

// PrewarmResult is returned to the caller after a container is pre-warmed.
type PrewarmResult struct {
	SessionID string
}

// Prewarm spins up a container before the user clicks "Start Challenge".
// Called with an anonymous token from localStorage. No auth required.
func (m *Manager) Prewarm(ctx context.Context, challengeID, anonToken string) (*PrewarmResult, error) {
	slog.Info("prewarming container", "challenge", challengeID, "anon_token", anonToken[:8]+"...")

	// Spin up the container
	info, err := m.deps.Provider.StartContainer(ctx, challengeID)
	if err != nil {
		return nil, fmt.Errorf("start container: %w", err)
	}

	// Persist to Supabase
	session, err := m.deps.Supabase.CreatePrewarmSession(ctx, challengeID, anonToken, info.ID, info.Host)
	if err != nil {
		// Best-effort: kill the container we just started
		_ = m.deps.Provider.StopContainer(context.Background(), info.ID)
		return nil, fmt.Errorf("create prewarm session: %w", err)
	}

	// Cache in Redis (5 min TTL — prewarm cleanup reaper uses this)
	state := RedisSessionState{
		SessionID:     session.ID,
		ChallengeID:   challengeID,
		ContainerID:   info.ID,
		ContainerHost: info.Host,
		Status:        string(supabase.SessionStatusPrewarming),
	}
	if err := m.deps.Redis.SetJSON(ctx, redisSessionKey(session.ID), state, 300); err != nil {
		slog.Warn("redis set failed for prewarm state", "session_id", session.ID, "error", err)
		// Non-fatal — Supabase is source of truth
	}

	// Map anon token → session ID for the promote call
	if err := m.deps.Redis.Set(ctx, redisAnonKey(anonToken), session.ID, 300); err != nil {
		slog.Warn("redis set failed for anon key", "anon_token", anonToken[:8], "error", err)
	}

	return &PrewarmResult{SessionID: session.ID}, nil
}

// StartResult is returned to the caller when a session is promoted.
type StartResult struct {
	SessionID     string
	ContainerHost string
	ChallengeID   string
}

// Start promotes a pre-warmed anonymous session to a real authenticated session.
// Validates that the anon token maps to a prewarming session, then attaches the user.
func (m *Manager) Start(ctx context.Context, userID, anonToken string) (*StartResult, error) {
	slog.Info("starting session", "user_id", userID, "anon_token", anonToken[:8]+"...")

	// Promote in Supabase (anon_token is the lookup key)
	session, err := m.deps.Supabase.PromoteSession(ctx, anonToken, userID)
	if err != nil {
		return nil, fmt.Errorf("promote session: %w", err)
	}

	now := time.Now().Unix()
	state := RedisSessionState{
		SessionID:     session.ID,
		UserID:        userID,
		ChallengeID:   session.ChallengeID,
		ContainerID:   *session.ContainerID,
		ContainerHost: *session.ContainerHost,
		Status:        string(supabase.SessionStatusActive),
		StartedAt:     now,
		LastActiveAt:  now,
	}

	// TTL = hard limit so Redis auto-expires if we forget to clean up
	if err := m.deps.Redis.SetJSON(ctx, redisSessionKey(session.ID), state, m.deps.HardLimitSecs); err != nil {
		slog.Warn("redis set failed on session start", "session_id", session.ID, "error", err)
	}

	// Clean up the anon key
	_ = m.deps.Redis.Del(ctx, redisAnonKey(anonToken))

	return &StartResult{
		SessionID:     session.ID,
		ContainerHost: *session.ContainerHost,
		ChallengeID:   session.ChallengeID,
	}, nil
}

// GetState retrieves current session state from Redis (fast path).
// Falls back to Supabase if the Redis key has expired.
func (m *Manager) GetState(ctx context.Context, sessionID string) (*RedisSessionState, error) {
	var state RedisSessionState
	found, err := m.deps.Redis.GetJSON(ctx, redisSessionKey(sessionID), &state)
	if err != nil {
		return nil, fmt.Errorf("redis get: %w", err)
	}
	if found {
		return &state, nil
	}

	// Fallback: load from Supabase
	session, err := m.deps.Supabase.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("supabase get session: %w", err)
	}
	if session == nil {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}
	if session.ContainerID == nil || session.ContainerHost == nil {
		return nil, fmt.Errorf("session %s has no container", sessionID)
	}

	state = RedisSessionState{
		SessionID:     session.ID,
		ChallengeID:   session.ChallengeID,
		ContainerID:   *session.ContainerID,
		ContainerHost: *session.ContainerHost,
		Status:        string(session.Status),
	}
	if session.UserID != nil {
		state.UserID = *session.UserID
	}
	return &state, nil
}

// TouchActivity updates the last_active_at timestamp on both Redis and Supabase.
// Called on every file write and terminal input.
func (m *Manager) TouchActivity(ctx context.Context, sessionID string) {
	now := time.Now().Unix()

	// Update Redis state in-place
	var state RedisSessionState
	if found, _ := m.deps.Redis.GetJSON(ctx, redisSessionKey(sessionID), &state); found {
		state.LastActiveAt = now
		_ = m.deps.Redis.SetJSON(ctx, redisSessionKey(sessionID), state, m.deps.HardLimitSecs)
	}

	// Async Supabase touch (non-blocking)
	go func() {
		if err := m.deps.Supabase.TouchSession(context.Background(), sessionID); err != nil {
			slog.Warn("touch session failed", "session_id", sessionID, "error", err)
		}
	}()
}

// WriteFile writes file content to the container and tracks the path as dirty.
func (m *Manager) WriteFile(ctx context.Context, sessionID, filePath, content string) error {
	state, err := m.GetState(ctx, sessionID)
	if err != nil {
		return err
	}

	if state.Status != string(supabase.SessionStatusActive) {
		return fmt.Errorf("session %s is not active (status: %s)", sessionID, state.Status)
	}

	if err := m.deps.Provider.WriteFile(ctx, state.ContainerID, filePath, content); err != nil {
		return fmt.Errorf("write file to container: %w", err)
	}

	// Track dirty paths in Redis for save-on-exit
	_ = m.deps.Redis.SAdd(ctx, redisDirtyFilesKey(sessionID), filePath)

	m.TouchActivity(ctx, sessionID)
	return nil
}

// ReadFile reads a file's content from the container.
func (m *Manager) ReadFile(ctx context.Context, sessionID, filePath string) (string, error) {
	state, err := m.GetState(ctx, sessionID)
	if err != nil {
		return "", err
	}
	return m.deps.Provider.ReadFile(ctx, state.ContainerID, filePath)
}

// ExitResult contains the summary produced by a clean exit.
type ExitResult struct {
	SavedFiles []string
}

// Exit performs a clean Save & Exit: saves all dirty files to Supabase,
// then kills the container.
func (m *Manager) Exit(ctx context.Context, sessionID string) (*ExitResult, error) {
	slog.Info("session exit", "session_id", sessionID)

	state, err := m.GetState(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Collect dirty file paths
	dirtyFiles, err := m.deps.Redis.SMembers(ctx, redisDirtyFilesKey(sessionID))
	if err != nil {
		slog.Warn("failed to get dirty files from redis; saving nothing", "session_id", sessionID, "error", err)
		dirtyFiles = []string{}
	}

	// Read each dirty file from the container and save to Supabase
	savedFiles := make([]string, 0, len(dirtyFiles))
	for _, filePath := range dirtyFiles {
		content, err := m.deps.Provider.ReadFile(ctx, state.ContainerID, filePath)
		if err != nil {
			slog.Error("failed to read file for exit save", "session_id", sessionID, "file", filePath, "error", err)
			continue // Save as many as we can; don't abort the whole exit
		}
		if err := m.deps.Supabase.UpsertFileDiff(ctx, sessionID, filePath, content); err != nil {
			slog.Error("failed to save file diff", "session_id", sessionID, "file", filePath, "error", err)
			continue
		}
		savedFiles = append(savedFiles, filePath)
	}

	// Mark session as exited
	now := time.Now()
	var durationSecs *int
	if state.StartedAt > 0 {
		d := int(now.Unix() - state.StartedAt)
		durationSecs = &d
	}

	if err := m.deps.Supabase.UpdateSessionStatus(ctx, sessionID, supabase.SessionStatusExited, &now, durationSecs); err != nil {
		slog.Error("failed to update session status on exit", "session_id", sessionID, "error", err)
	}

	// Kill the container
	if err := m.deps.Provider.StopContainer(ctx, state.ContainerID); err != nil {
		slog.Error("failed to stop container on exit", "container_id", state.ContainerID, "error", err)
	}

	// Clean up Redis
	_ = m.deps.Redis.Del(ctx, redisSessionKey(sessionID), redisDirtyFilesKey(sessionID))

	return &ExitResult{SavedFiles: savedFiles}, nil
}

// Resume replays saved file diffs into a fresh container for a returning user.
func (m *Manager) Resume(ctx context.Context, userID, challengeID, previousSessionID string) (*StartResult, error) {
	slog.Info("resuming session", "user_id", userID, "previous_session", previousSessionID)

	// Start a fresh container
	info, err := m.deps.Provider.StartContainer(ctx, challengeID)
	if err != nil {
		return nil, fmt.Errorf("start container for resume: %w", err)
	}

	// Load saved file diffs from Supabase
	diffs, err := m.deps.Supabase.GetFileDiffs(ctx, previousSessionID)
	if err != nil {
		_ = m.deps.Provider.StopContainer(context.Background(), info.ID)
		return nil, fmt.Errorf("load file diffs: %w", err)
	}

	// Replay diffs into the fresh container
	for _, diff := range diffs {
		if err := m.deps.Provider.WriteFile(ctx, info.ID, diff.FilePath, diff.Content); err != nil {
			slog.Error("failed to replay file diff on resume", "file", diff.FilePath, "error", err)
			// Non-fatal — continue replaying other files
		}
	}

	// Create a new session record linked to the resumed state
	session, err := m.deps.Supabase.CreatePrewarmSession(ctx, challengeID, "", info.ID, info.Host)
	if err != nil {
		_ = m.deps.Provider.StopContainer(context.Background(), info.ID)
		return nil, fmt.Errorf("create resume session: %w", err)
	}

	// Immediately promote to active (no prewarm phase for resume)
	activeSession, err := m.deps.Supabase.PromoteSession(ctx, *session.AnonToken, userID)
	if err != nil {
		_ = m.deps.Provider.StopContainer(context.Background(), info.ID)
		return nil, fmt.Errorf("promote resume session: %w", err)
	}

	now := time.Now().Unix()
	state := RedisSessionState{
		SessionID:     activeSession.ID,
		UserID:        userID,
		ChallengeID:   challengeID,
		ContainerID:   info.ID,
		ContainerHost: info.Host,
		Status:        string(supabase.SessionStatusActive),
		StartedAt:     now,
		LastActiveAt:  now,
	}
	_ = m.deps.Redis.SetJSON(ctx, redisSessionKey(activeSession.ID), state, m.deps.HardLimitSecs)

	return &StartResult{
		SessionID:     activeSession.ID,
		ContainerHost: info.Host,
		ChallengeID:   challengeID,
	}, nil
}

// expireSession kills a container and marks the session expired.
// Used by both the inactivity reaper and the hard-limit reaper.
func (m *Manager) expireSession(ctx context.Context, sessionID, containerID string, startedAt int64) {
	slog.Info("expiring session", "session_id", sessionID)

	now := time.Now()
	var durationSecs *int
	if startedAt > 0 {
		d := int(now.Unix() - startedAt)
		durationSecs = &d
	}

	if err := m.deps.Provider.StopContainer(ctx, containerID); err != nil {
		slog.Error("failed to stop container on expiry", "container_id", containerID, "error", err)
	}

	if err := m.deps.Supabase.UpdateSessionStatus(ctx, sessionID, supabase.SessionStatusExpired, &now, durationSecs); err != nil {
		slog.Error("failed to update session status on expiry", "session_id", sessionID, "error", err)
	}

	_ = m.deps.Redis.Del(ctx, redisSessionKey(sessionID), redisDirtyFilesKey(sessionID))
}

// RunInactivityReaper runs a background loop that expires sessions that have
// been inactive for longer than InactivitySecs, and sessions that have exceeded
// the hard HardLimitSecs wall-clock limit.
func (m *Manager) RunInactivityReaper(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.reapExpiredSessions(ctx)
		}
	}
}

func (m *Manager) reapExpiredSessions(ctx context.Context) {
	sessions, err := m.deps.Supabase.GetActiveSessions(ctx)
	if err != nil {
		slog.Error("inactivity reaper: failed to get active sessions", "error", err)
		return
	}

	now := time.Now()
	for _, s := range sessions {
		if s.ContainerID == nil {
			continue
		}

		// Hard limit check
		if s.StartedAt != nil && now.Sub(*s.StartedAt).Seconds() > float64(m.deps.HardLimitSecs) {
			slog.Info("hard limit reached", "session_id", s.ID)
			var startedUnix int64
			if s.StartedAt != nil {
				startedUnix = s.StartedAt.Unix()
			}
			m.expireSession(ctx, s.ID, *s.ContainerID, startedUnix)
			continue
		}

		// Inactivity check
		if s.LastActiveAt != nil && now.Sub(*s.LastActiveAt).Seconds() > float64(m.deps.InactivitySecs) {
			slog.Info("inactivity timeout", "session_id", s.ID)
			var startedUnix int64
			if s.StartedAt != nil {
				startedUnix = s.StartedAt.Unix()
			}
			m.expireSession(ctx, s.ID, *s.ContainerID, startedUnix)
		}
	}
}

// RunPrewarmCleanup removes pre-warmed containers that were never promoted to
// a real session after 5 minutes. Runs as a background goroutine.
func (m *Manager) RunPrewarmCleanup(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.reapStalePrwarmSessions(ctx)
		}
	}
}

func (m *Manager) reapStalePrwarmSessions(ctx context.Context) {
	sessions, err := m.deps.Supabase.GetPrewarmingSessions(ctx)
	if err != nil {
		slog.Error("prewarm cleanup: failed to get prewarming sessions", "error", err)
		return
	}

	cutoff := time.Now().Add(-5 * time.Minute)
	for _, s := range sessions {
		if s.CreatedAt.After(cutoff) {
			continue // Still within the 5-minute grace period
		}
		if s.ContainerID == nil {
			continue
		}

		slog.Info("reaping stale prewarm container", "session_id", s.ID, "created_at", s.CreatedAt)

		if err := m.deps.Provider.StopContainer(ctx, *s.ContainerID); err != nil {
			slog.Error("failed to stop stale prewarm container", "container_id", *s.ContainerID, "error", err)
		}

		now := time.Now()
		_ = m.deps.Supabase.UpdateSessionStatus(ctx, s.ID, supabase.SessionStatusExpired, &now, nil)
		_ = m.deps.Redis.Del(ctx, redisSessionKey(s.ID))
	}
}

// OpenShell opens an interactive PTY session to the container.
func (m *Manager) OpenShell(ctx context.Context, sessionID string) (sandbox.ShellSession, error) {
	state, err := m.GetState(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if state.Status != string(supabase.SessionStatusActive) {
		return nil, fmt.Errorf("session %s is not active", sessionID)
	}
	return m.deps.Provider.ExecShell(ctx, state.ContainerID)
}
