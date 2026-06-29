package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"stacktrace/orchestrator/internal/clerk"
	"stacktrace/orchestrator/internal/db"
	"stacktrace/orchestrator/internal/middleware"
	"stacktrace/orchestrator/internal/session"

	"github.com/gin-gonic/gin"
)

type handlers struct {
	sessions      *session.Manager
	verifier      *clerk.JWTVerifier
	db            *db.Client
	challengesDir string
}

// ── Health ─────────────────────────────────────────────────────────────────────

func (h *handlers) healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ── Prewarm ────────────────────────────────────────────────────────────────────

type prewarmRequest struct {
	ChallengeID string `json:"challenge_id" binding:"required"`
	AnonToken   string `json:"anon_token"   binding:"required,uuid"`
}

type prewarmResponse struct {
	SessionID string `json:"session_id"`
}

func (h *handlers) prewarm(c *gin.Context) {
	var req prewarmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.sessions.Prewarm(c.Request.Context(), req.ChallengeID, req.AnonToken)
	if err != nil {
		slog.Error("prewarm failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, prewarmResponse{SessionID: result.SessionID})
}

// ── Start session ──────────────────────────────────────────────────────────────

type startSessionRequest struct {
	AnonToken   string `json:"anon_token"   binding:"required,uuid"`
	ChallengeID string `json:"challenge_id" binding:"required"`
}

type startSessionResponse struct {
	SessionID     string `json:"session_id"`
	ContainerHost string `json:"container_host"`
	ChallengeID   string `json:"challenge_id"`
	TerminalWSURL string `json:"terminal_ws_url"`
}

func (h *handlers) startSession(c *gin.Context) {
	claims := middleware.GetClaims(c)

	var req startSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.db.UpsertUser(c.Request.Context(), claims.Sub, claims.Email); err != nil {
		slog.Error("upsert user failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result, err := h.sessions.Start(c.Request.Context(), claims.Sub, req.AnonToken)
	if err != nil {
		slog.Info("start via prewarm failed, trying on-demand", "error", err)
		result, err = h.sessions.StartOnDemand(c.Request.Context(), claims.Sub, req.ChallengeID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	wsURL := buildWSURL(c.Request, "/sessions/"+result.SessionID+"/terminal")
	c.JSON(http.StatusCreated, startSessionResponse{
		SessionID:     result.SessionID,
		ContainerHost: result.ContainerHost,
		ChallengeID:   result.ChallengeID,
		TerminalWSURL: wsURL,
	})
}

// ── Get session ────────────────────────────────────────────────────────────────

type getSessionResponse struct {
	SessionID     string `json:"session_id"`
	ChallengeID   string `json:"challenge_id"`
	Status        string `json:"status"`
	ContainerHost string `json:"container_host"`
	ElapsedSecs   int64  `json:"elapsed_secs"`
}

func (h *handlers) getSession(c *gin.Context) {
	claims := middleware.GetClaims(c)
	sessionID := c.Param("id")

	state, err := h.sessions.GetState(c.Request.Context(), sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
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

// ── Exit session ───────────────────────────────────────────────────────────────

type exitSessionResponse struct {
	SavedFiles []string `json:"saved_files"`
}

func (h *handlers) exitSession(c *gin.Context) {
	claims := middleware.GetClaims(c)
	sessionID := c.Param("id")

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
		slog.Error("exit session failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, exitSessionResponse{SavedFiles: result.SavedFiles})
}

// ── Write file ─────────────────────────────────────────────────────────────────

type writeFileRequest struct {
	FilePath string `json:"file_path" binding:"required"`
	Content  string `json:"content"   binding:"required"`
}

func (h *handlers) writeFile(c *gin.Context) {
	claims := middleware.GetClaims(c)
	sessionID := c.Param("id")

	var req writeFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

	if err := h.sessions.WriteFile(c.Request.Context(), sessionID, req.FilePath, req.Content); err != nil {
		slog.Error("write file failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ── Read file ──────────────────────────────────────────────────────────────────

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
		slog.Error("read file failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"file_path": filePath, "content": content})
}

// ── List files (file tree) ─────────────────────────────────────────────────────
// GET /sessions/:id/tree
// Runs find inside the container and returns the file tree dynamically.
// No manual file listing needed in challenge.json — works for any codebase.

type fileTreeNode struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Type     string `json:"type"`     // "file" | "directory"
	Language string `json:"language"` // derived from extension
}

func (h *handlers) listFiles(c *gin.Context) {
	claims := middleware.GetClaims(c)
	sessionID := c.Param("id")

	state, err := h.sessions.GetState(c.Request.Context(), sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	if state.UserID != claims.Sub {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	// working_dir comes from challenge.json — each challenge controls its own
	// scope so the orchestrator never hardcodes a path for any specific challenge.
	workingDir, err := h.getWorkingDir(state.ChallengeID)
	if err != nil {
		slog.Error("failed to read working_dir", "error", err, "challenge", state.ChallengeID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	containerRoot := "/app/" + workingDir
	if workingDir != "" {
		containerRoot = "/app/" + workingDir
	}

	// -mindepth 1 excludes the root directory itself from the output, so we
	// don't need to special-case it when parsing.
	output, err := h.sessions.ExecCommand(c.Request.Context(), sessionID,
		[]string{"find", containerRoot, "-mindepth", "1", "-not", "-path", "*/node_modules/*", "-not", "-path", "*/node_modules", "-not", "-name", ".*"})
	if err != nil {
		slog.Error("list files failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Paths returned to the frontend are relative to /app (NOT to containerRoot)
	// so they match exactly what ReadFile/WriteFile expect — those functions
	// always prepend "/app/" and nothing else, regardless of working_dir.
	nodes := parseFileTree(output, "/app/")
	c.JSON(http.StatusOK, gin.H{"files": nodes})
}

// getWorkingDir reads challenge.json from disk and returns its working_dir
// field. This is the single source of truth for where a challenge's editable
// code lives inside its container — keeps the orchestrator generic across
// any challenge's folder structure.
func (h *handlers) getWorkingDir(challengeID string) (string, error) {
	path := filepath.Join(h.challengesDir, challengeID, "challenge.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read challenge.json: %w", err)
	}

	var parsed struct {
		WorkingDir string `json:"working_dir"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("parse challenge.json: %w", err)
	}
	return parsed.WorkingDir, nil
}

// parseFileTree converts find output lines into FileTreeNode structs.
// Paths returned are relative to /app — ReadFile and WriteFile are the only
// functions that ever add the "/app/" prefix back (via containerFilePath),
// so there is exactly one place in the whole codebase that knows about /app,
// and the tree's paths always match what those functions expect to receive.
func parseFileTree(findOutput, appPrefix string) []fileTreeNode {
	lines := strings.Split(strings.TrimSpace(findOutput), "\n")
	nodes := make([]fileTreeNode, 0, len(lines))

	prefix := appPrefix

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Strip the container root prefix (e.g. "/app/examples/web-service/")
		// so the path returned to the frontend matches exactly what ReadFile
		// and WriteFile expect to receive.
		relativePath := strings.TrimPrefix(line, prefix)
		if relativePath == line {
			// Prefix didn't match — skip rather than risk a malformed path
			continue
		}

		parts := strings.Split(relativePath, "/")
		name := parts[len(parts)-1]

		nodeType := "file"
		language := languageFromName(name)
		if !strings.Contains(name, ".") {
			nodeType = "directory"
			language = ""
		}

		nodes = append(nodes, fileTreeNode{
			Name:     name,
			Path:     relativePath,
			Type:     nodeType,
			Language: language,
		})
	}
	return nodes
}

func languageFromName(name string) string {
	parts := strings.Split(name, ".")
	if len(parts) < 2 {
		return "plaintext"
	}
	ext := parts[len(parts)-1]
	switch ext {
	case "js", "mjs", "cjs":
		return "javascript"
	case "ts", "tsx":
		return "typescript"
	case "json":
		return "json"
	case "md":
		return "markdown"
	case "py":
		return "python"
	case "go":
		return "go"
	case "sh":
		return "shell"
	case "html":
		return "html"
	case "css":
		return "css"
	case "yml", "yaml":
		return "yaml"
	default:
		return "plaintext"
	}
}

// ── Resume session ─────────────────────────────────────────────────────────────

type resumeSessionRequest struct {
	ChallengeID string `json:"challenge_id" binding:"required"`
}

func (h *handlers) resumeSession(c *gin.Context) {
	claims := middleware.GetClaims(c)
	previousSessionID := c.Param("id")

	var req resumeSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.db.UpsertUser(c.Request.Context(), claims.Sub, claims.Email); err != nil {
		slog.Error("upsert user failed (resume)", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result, err := h.sessions.Resume(c.Request.Context(), claims.Sub, req.ChallengeID, previousSessionID)
	if err != nil {
		slog.Error("resume session failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

// ── Helpers ────────────────────────────────────────────────────────────────────

func buildWSURL(r *http.Request, path string) string {
	scheme := "ws"
	if r.Header.Get("X-Forwarded-Proto") == "https" || r.TLS != nil {
		scheme = "wss"
	}
	return scheme + "://" + r.Host + path
}
