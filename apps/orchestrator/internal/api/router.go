// Package api wires the Gin router and injects all handler dependencies.
package api

import (
	"net/http"
	"os"

	"stacktrace/orchestrator/internal/clerk"
	"stacktrace/orchestrator/internal/middleware"
	"stacktrace/orchestrator/internal/session"
	"stacktrace/orchestrator/internal/supabase"

	"github.com/gin-gonic/gin"
)

// RouterDeps holds all dependencies needed to build the router.
type RouterDeps struct {
	SessionManager *session.Manager
	ClerkVerifier  *clerk.JWTVerifier
	Supabase       *supabase.Client
}

// NewRouter constructs and returns the fully configured Gin engine.
func NewRouter(deps RouterDeps) http.Handler {
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(requestLogger())
	r.Use(corsMiddleware())

	h := &handlers{
		sessions: deps.SessionManager,
		verifier: deps.ClerkVerifier,
		supabase: deps.Supabase,
	}

	// Health check — no auth required, used by container readiness probes
	r.GET("/healthz", h.healthz)

	// Prewarm — no auth required (anon token only).
	// Called as soon as user lands on the challenge page.
	r.POST("/prewarm", h.prewarm)

	// Authenticated session routes
	auth := r.Group("/sessions")
	auth.Use(middleware.RequireAuth(deps.ClerkVerifier))
	{
		// POST /sessions — promote a pre-warmed container to a real session
		auth.POST("", h.startSession)

		// GET /sessions/:id — session status + metadata
		auth.GET("/:id", h.getSession)

		// DELETE /sessions/:id — Save & Exit
		auth.DELETE("/:id", h.exitSession)

		// POST /sessions/:id/files — write a file into the container (Monaco sync)
		auth.POST("/:id/files", h.writeFile)

		// GET /sessions/:id/files — read a file from the container
		auth.GET("/:id/files", h.readFile)

		// GET /sessions/:id/resume — start a new session replaying saved diffs
		auth.POST("/:id/resume", h.resumeSession)

		// WebSocket terminal — upgrade happens here; auth header validated before upgrade
		auth.GET("/:id/terminal", h.terminal)
	}

	return r
}
