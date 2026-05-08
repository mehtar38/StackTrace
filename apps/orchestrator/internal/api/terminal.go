package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"stacktrace/orchestrator/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	// CheckOrigin validates against the same allowed origins used by the CORS middleware.
	// The allowedOrigins list is read from ALLOWED_ORIGINS env var.
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		allowed := getAllowedOrigins()
		return isAllowedOrigin(origin, allowed)
	},
}

// terminalMessage is the JSON envelope for messages between the browser and orchestrator.
// The browser sends: { "type": "input", "data": "<base64 or raw string>" }
//
//	{ "type": "resize", "cols": 220, "rows": 50 }
//
// The orchestrator sends: { "type": "output", "data": "<raw string>" }
//
//	{ "type": "error",  "data": "<message>" }
type terminalMessage struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	Cols uint16 `json:"cols,omitempty"`
	Rows uint16 `json:"rows,omitempty"`
}

// terminal godoc
// GET /sessions/:id/terminal  (WebSocket upgrade)
//
// Auth: Clerk JWT is validated before the WebSocket upgrade via the RequireAuth
// middleware applied on the route group. The upgrade only proceeds if auth passes.
//
// Flow:
//  1. Upgrade the HTTP connection to WebSocket.
//  2. Open a PTY exec session inside the container.
//  3. Pump: container stdout → WS, WS input → container stdin.
//  4. On WS close or error, close the PTY and release resources.
func (h *handlers) terminal(c *gin.Context) {
	claims := middleware.GetClaims(c)
	sessionID := c.Param("id")

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

	// Upgrade the HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("websocket upgrade failed", "session_id", sessionID, "error", err)
		return
	}
	defer conn.Close()

	slog.Info("terminal connected", "session_id", sessionID, "user", claims.Sub)

	// Open a PTY shell inside the container
	shell, err := h.sessions.OpenShell(c.Request.Context(), sessionID)
	if err != nil {
		sendWSError(conn, "failed to open shell: "+err.Error())
		return
	}
	defer shell.Close()

	// container → WS pump (runs in a goroutine)
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

	// WS → container pump (main goroutine)
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
			// Treat unparseable messages as raw input (some xterm.js setups send raw bytes)
			_, _ = shell.Write(rawMsg)
			h.sessions.TouchActivity(c.Request.Context(), sessionID)
			continue
		}

		switch strings.ToLower(msg.Type) {
		case "input":
			if _, err := shell.Write([]byte(msg.Data)); err != nil {
				slog.Warn("shell write error", "session_id", sessionID, "error", err)
				return
			}
			h.sessions.TouchActivity(c.Request.Context(), sessionID)

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
