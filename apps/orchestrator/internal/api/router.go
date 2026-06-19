// Package api wires the Gin router and injects all handler dependencies.
package api

import (
	"net/http"
	"os"

	"stacktrace/orchestrator/internal/clerk"
	"stacktrace/orchestrator/internal/db"
	"stacktrace/orchestrator/internal/middleware"
	"stacktrace/orchestrator/internal/session"

	"github.com/gin-gonic/gin"
)

// RouterDeps holds all dependencies needed to build the router.
type RouterDeps struct {
	SessionManager *session.Manager
	ClerkVerifier  *clerk.JWTVerifier
	DB             *db.Client
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
		db:       deps.DB,
	}

	// Health check — no auth, used by Docker/ACA readiness probes
	r.GET("/healthz", h.healthz)

	// Prewarm — no auth (anon token only)
	r.POST("/prewarm", h.prewarm)

	// WebSocket terminal — auth handled inside the handler via subprotocol token,
	// NOT via RequireAuth middleware (browser WS can't send custom headers)
	r.GET("/sessions/:id/terminal", h.terminal)

	// All other session routes require a valid Clerk JWT in Authorization header
	auth := r.Group("/sessions")
	auth.Use(middleware.RequireAuth(deps.ClerkVerifier))
	{
		auth.POST("", h.startSession)
		auth.GET("/:id", h.getSession)
		auth.DELETE("/:id", h.exitSession)
		auth.POST("/:id/files", h.writeFile)
		auth.GET("/:id/files", h.readFile)
		auth.POST("/:id/resume", h.resumeSession)
		auth.GET("/:id/tree", h.listFiles)
	}

	return r
}
