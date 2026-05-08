// Package middleware provides Gin middleware for the orchestrator.
package middleware

import (
	"net/http"
	"strings"

	"stacktrace/orchestrator/internal/clerk"

	"github.com/gin-gonic/gin"
)

// claimsKey is the Gin context key for the validated Clerk claims.
const claimsKey = "clerk_claims"

// RequireAuth is a Gin middleware that validates the Clerk JWT in the
// Authorization header. On success it injects *clerk.Claims into the context.
// On failure it aborts with 401.
//
// The token must be supplied as: Authorization: Bearer <token>
// For WebSocket upgrade requests the same header applies — the WS client must
// supply it during the HTTP upgrade handshake.
func RequireAuth(verifier *clerk.JWTVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing Authorization header"})
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header must be: Bearer <token>"})
			return
		}

		claims, err := verifier.Verify(c.Request.Context(), parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		c.Set(claimsKey, claims)
		c.Next()
	}
}

// GetClaims extracts the validated Clerk claims from a Gin context.
// Panics if RequireAuth middleware was not applied — this is intentional,
// as it indicates a routing misconfiguration.
func GetClaims(c *gin.Context) *clerk.Claims {
	v, exists := c.Get(claimsKey)
	if !exists {
		panic("GetClaims called without RequireAuth middleware")
	}
	return v.(*clerk.Claims)
}
