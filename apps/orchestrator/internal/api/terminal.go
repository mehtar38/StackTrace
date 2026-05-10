package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		allowed := getAllowedOrigins()
		return isAllowedOrigin(origin, allowed)
	},
}

type terminalMessage struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	Cols uint16 `json:"cols,omitempty"`
	Rows uint16 `json:"rows,omitempty"`
}

// terminal handles WebSocket upgrades for the container PTY.
//
// Auth note: the browser WebSocket API cannot send custom HTTP headers during
// the upgrade. We use Sec-WebSocket-Protocol as a token carrier instead.
// The frontend sends: Sec-WebSocket-Protocol: bearer.<clerk_jwt>
// This handler extracts it, validates it, then upgrades. The route does NOT
// use RequireAuth middleware — auth happens here before the upgrade.
func (h *handlers) terminal(c *gin.Context) {
	sessionID := c.Param("id")

	// Extract token from subprotocol header
	token, err := extractWSToken(c.Request)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// Validate Clerk JWT
	claims, err := h.verifier.Verify(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
		return
	}

	// Verify session ownership before upgrading
	state, err := h.sessions.GetState(c.Request.Context(), sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	if state.UserID != claims.Sub {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	// Echo subprotocol back so the browser completes the handshake
	responseHeader := http.Header{}
	responseHeader.Set("Sec-WebSocket-Protocol", "bearer.ok")

	conn, err := upgrader.Upgrade(c.Writer, c.Request, responseHeader)
	if err != nil {
		slog.Error("websocket upgrade failed", "session_id", sessionID, "error", err)
		return
	}
	defer conn.Close()

	slog.Info("terminal connected", "session_id", sessionID, "user", claims.Sub)

	shell, err := h.sessions.OpenShell(c.Request.Context(), sessionID)
	if err != nil {
		sendWSError(conn, "failed to open shell: "+err.Error())
		return
	}
	defer shell.Close()

	// Container → browser pump
	wsWriteErr := make(chan error, 1)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := shell.Read(buf)
			if n > 0 {
				msg := terminalMessage{Type: "output", Data: string(buf[:n])}
				if writeErr := writeWSJSON(conn, msg); writeErr != nil {
					wsWriteErr <- writeErr
					return
				}
			}
			if err != nil {
				if err != io.EOF {
					wsWriteErr <- err
				} else {
					wsWriteErr <- nil
				}
				return
			}
		}
	}()

	// Browser → container pump
	for {
		select {
		case err := <-wsWriteErr:
			if err != nil {
				slog.Warn("terminal output pump error", "session_id", sessionID, "error", err)
			}
			return
		default:
		}

		_, rawMsg, err := conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				slog.Warn("websocket read error", "session_id", sessionID, "error", err)
			}
			return
		}

		var msg terminalMessage
		if err := json.Unmarshal(rawMsg, &msg); err != nil {
			_, _ = shell.Write(rawMsg)
			h.sessions.TouchActivity(context.Background(), sessionID)
			continue
		}

		switch strings.ToLower(msg.Type) {
		case "input":
			if _, err := shell.Write([]byte(msg.Data)); err != nil {
				slog.Warn("shell write error", "session_id", sessionID, "error", err)
				return
			}
			h.sessions.TouchActivity(context.Background(), sessionID)
		case "resize":
			if msg.Rows > 0 && msg.Cols > 0 {
				if err := shell.Resize(msg.Rows, msg.Cols); err != nil {
					slog.Warn("shell resize error", "session_id", sessionID, "error", err)
				}
			}
		default:
			slog.Debug("unknown terminal message type", "type", msg.Type)
		}
	}
}

// extractWSToken pulls the Clerk JWT from Sec-WebSocket-Protocol.
// The browser sends: "bearer.<token>" — we strip the prefix and return the JWT.
func extractWSToken(r *http.Request) (string, error) {
	protocols := websocket.Subprotocols(r)
	for _, p := range protocols {
		if strings.HasPrefix(p, "bearer.") {
			token := strings.TrimPrefix(p, "bearer.")
			if token == "" {
				return "", fmt.Errorf("empty bearer token in subprotocol")
			}
			return token, nil
		}
	}
	return "", fmt.Errorf("no bearer token in Sec-WebSocket-Protocol header")
}

func writeWSJSON(conn *websocket.Conn, v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, b)
}

func sendWSError(conn *websocket.Conn, msg string) {
	_ = writeWSJSON(conn, terminalMessage{Type: "error", Data: msg})
}
