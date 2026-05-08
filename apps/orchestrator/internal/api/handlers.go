package api

import (
	"net/http"

	"stacktrace/orchestrator/internal/clerk"
	"stacktrace/orchestrator/internal/middleware"
	"stacktrace/orchestrator/internal/session"
	"stacktrace/orchestrator/internal/supabase"

	"github.com/gin-gonic/gin"
)

// handlers holds all injected dependencies for HTTP handler methods.
type handlers struct {
	sessions *session.Manager
	verifier *clerk.JWTVerifier
	supabase *supabase.Client
}

// ── Health ────────────────────────────────────────────────────────────────────

// healthz godoc
// GET /healthz
// Used by Docker and Azure Container Apps readiness probes.
func (h *handlers) healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ── Prewarm ───────────────────────────────────────────────────────────────────

type prewarmRequest struct {
	ChallengeID string `json:"challenge_id" binding:"required"`
	AnonToken   string `json:"anon_token"   binding:"required,uuid"`
}

type prewarmResponse struct {
	SessionID string `json:"session_id"`
}

// prewarm godoc
// POST /prewarm
// No auth required. Starts a container before the user clicks "Start Challenge".
func (h *handlers) prewarm(c *gin.Context) {
	var req prewarmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.sessions.Prewarm(c.Request.Context(), req.ChallengeID, req.AnonToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to prewarm container"})
		return
	}

	c.JSON(http.StatusAccepted, prewarmResponse{SessionID: result.SessionID})
}

// ── Start session ─────────────────────────────────────────────────────────────

type startSessionRequest struct {
	AnonToken string `json:"anon_token" binding:"required,uuid"`
}

type startSessionResponse struct {
	SessionID     string `json:"session_id"`
	ContainerHost string `json:"container_host"`
	ChallengeID   string `json:"challenge_id"`
	TerminalWSURL string `json:"terminal_ws_url"`
}

// startSession godoc
// POST /sessions
// Requires auth. Promotes a pre-warmed anon session to a real authenticated session.
func (h *handlers) startSession(c *gin.Context) {
	claims := middleware.GetClaims(c)

	var req startSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Upsert the user (no-op if already exists)
	if err := h.supabase.UpsertUser(c.Request.Context(), claims.Sub, claims.Email); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upsert user"})
		return
	}

	result, err := h.sessions.Start(c.Request.Context(), claims.Sub, req.AnonToken)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Build the WebSocket URL the frontend will connect to for the terminal
	wsURL := buildWSURL(c.Request, "/sessions/"+result.SessionID+"/terminal")

	c.JSON(http.StatusCreated, startSessionResponse{
		SessionID:     result.SessionID,
		ContainerHost: result.ContainerHost,
		ChallengeID:   result.ChallengeID,
		TerminalWSURL: wsURL,
	})
}

// ── Get session ───────────────────────────────────────────────────────────────

type getSessionResponse struct {
	SessionID     string `json:"session_id"`
	ChallengeID   string `json:"challenge_id"`
	Status        string `json:"status"`
	ContainerHost string `json:"container_host"`
	ElapsedSecs   int64  `json:"elapsed_secs"`
}

// getSession godoc
// GET /sessions/:id
// Returns current session status and elapsed time.
func (h *handlers) getSession(c *gin.Context) {
	claims := middleware.GetClaims(c)
	sessionID := c.Param("id")

	state, err := h.sessions.GetState(c.Request.Context(), sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	// Enforce ownership — a user can only see their own sessions
	if state.UserID != "" && state.UserID != claims.Sub {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	var elapsedSecs int64
	if state.StartedAt > 0 {
		elapsedSecs = state.LastActiveAt - state.StartedAt
	}

	c.JSON(http.StatusOK, getSessionResponse{
		SessionID:     state.SessionID,
		ChallengeID:   state.ChallengeID,
		Status:        state.Status,
		ContainerHost: state.ContainerHost,
		ElapsedSecs:   elapsedSecs,
	})
}

// ── Exit session ──────────────────────────────────────────────────────────────

type exitSessionResponse struct {
	SavedFiles []string `json:"saved_files"`
}

// exitSession godoc
// DELETE /sessions/:id
// Clean Save & Exit. Saves all dirty files to Supabase, kills the container.
func (h *handlers) exitSession(c *gin.Context) {
	claims := middleware.GetClaims(c)
	sessionID := c.Param("id")

	// Ownership check before any mutation
	state, err := h.sessions.GetState(c.Request.Context(), sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	if state.UserID != claims.Sub {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	result, err := h.sessions.Exit(c.Request.Context(), sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "exit failed"})
		return
	}

	c.JSON(http.StatusOK, exitSessionResponse{SavedFiles: result.SavedFiles})
}

// ── Write file ────────────────────────────────────────────────────────────────

type writeFileRequest struct {
	// FilePath is relative to /app inside the container, e.g. "src/db/write.js"
	FilePath string `json:"file_path" binding:"required"`
	Content  string `json:"content"   binding:"required"`
}

// writeFile godoc
// POST /sessions/:id/files
// Writes full file content into the container. Called by Monaco on 5–8s debounce.
func (h *handlers) writeFile(c *gin.Context) {
	claims := middleware.GetClaims(c)
	sessionID := c.Param("id")

	var req writeFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Ownership check
	state, err := h.sessions.GetState(c.Request.Context(), sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	if state.UserID != claims.Sub {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	if err := h.sessions.WriteFile(c.Request.Context(), sessionID, req.FilePath, req.Content); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "write failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ── Read file ─────────────────────────────────────────────────────────────────

// readFile godoc
// GET /sessions/:id/files?path=src/db/write.js
// Reads file content from the container. Used when Monaco needs to hydrate
// after a session resume.
func (h *handlers) readFile(c *gin.Context) {
	claims := middleware.GetClaims(c)
	sessionID := c.Param("id")
	filePath := c.Query("path")

	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing ?path= query parameter"})
		return
	}

	state, err := h.sessions.GetState(c.Request.Context(), sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	if state.UserID != claims.Sub {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	content, err := h.sessions.ReadFile(c.Request.Context(), sessionID, filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "read failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"file_path": filePath, "content": content})
}

// ── Resume session ────────────────────────────────────────────────────────────

type resumeSessionRequest struct {
	ChallengeID string `json:"challenge_id" binding:"required"`
}

// resumeSession godoc
// POST /sessions/:id/resume
// :id is the previous (exited) session ID whose diffs will be replayed.
func (h *handlers) resumeSession(c *gin.Context) {
	claims := middleware.GetClaims(c)
	previousSessionID := c.Param("id")

	var req resumeSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.supabase.UpsertUser(c.Request.Context(), claims.Sub, claims.Email); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upsert user"})
		return
	}

	result, err := h.sessions.Resume(c.Request.Context(), claims.Sub, req.ChallengeID, previousSessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "resume failed"})
		return
	}

	wsURL := buildWSURL(c.Request, "/sessions/"+result.SessionID+"/terminal")

	c.JSON(http.StatusCreated, startSessionResponse{
		SessionID:     result.SessionID,
		ContainerHost: result.ContainerHost,
		ChallengeID:   result.ChallengeID,
		TerminalWSURL: wsURL,
	})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// buildWSURL derives the WebSocket URL for a given path from the incoming HTTP request.
// Uses wss:// in production (X-Forwarded-Proto: https) and ws:// in development.
func buildWSURL(r *http.Request, path string) string {
	scheme := "ws"
	if r.Header.Get("X-Forwarded-Proto") == "https" || r.TLS != nil {
		scheme = "wss"
	}
	return scheme + "://" + r.Host + path
}
