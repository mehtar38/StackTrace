'use client'

// components/challenge/TerminalPanel.tsx
// Xterm.js terminal connected to the orchestrator WebSocket.
// Must be 'use client' — Xterm.js uses the DOM.
// Imported in ChallengeIDE via dynamic() with ssr: false.

import { useEffect, useRef } from 'react'
import { getOrchestratorWSBase } from '@/lib/api/orchestrator'
import { useAuth } from '@clerk/nextjs'

interface TerminalPanelProps {
  sessionId: string
  isVisible: boolean
}

export default function TerminalPanel({ sessionId, isVisible }: TerminalPanelProps) {
  const terminalRef = useRef<HTMLDivElement>(null)
  const xtermRef = useRef<import('@xterm/xterm').Terminal | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const { getToken } = useAuth()

  useEffect(() => {
    if (!isVisible || !terminalRef.current || xtermRef.current) return

    let terminal: import('@xterm/xterm').Terminal
    let ws: WebSocket

    async function init() {
      // Dynamic imports — Xterm.js is large, only load when terminal is visible
      const [{ Terminal }, { FitAddon }, { WebLinksAddon }] = await Promise.all([
        import('@xterm/xterm'),
        import('@xterm/addon-fit'),
        import('@xterm/addon-web-links'),
      ])

      terminal = new Terminal({
        theme: {
          background: '#0a0a0b',
          foreground: '#e4e4e7',
          cursor: '#e4e4e7',
          selectionBackground: 'rgba(255,255,255,0.15)',
          black: '#18181b',
          red: '#f87171',
          green: '#4ade80',
          yellow: '#fbbf24',
          blue: '#60a5fa',
          magenta: '#c084fc',
          cyan: '#22d3ee',
          white: '#e4e4e7',
          brightBlack: '#3f3f46',
          brightRed: '#fca5a5',
          brightGreen: '#86efac',
          brightYellow: '#fde68a',
          brightBlue: '#93c5fd',
          brightMagenta: '#d8b4fe',
          brightCyan: '#67e8f9',
          brightWhite: '#f4f4f5',
        },
        fontFamily: '"Geist Mono", "Fira Code", "Cascadia Code", monospace',
        fontSize: 13,
        lineHeight: 1.5,
        cursorBlink: true,
        cursorStyle: 'block',
        scrollback: 5000,
        // Allow 256-color and true-color sequences
        allowProposedApi: true,
      })

      const fitAddon = new FitAddon()
      const webLinksAddon = new WebLinksAddon()
      terminal.loadAddon(fitAddon)
      terminal.loadAddon(webLinksAddon)

      if (!terminalRef.current) return
      terminal.open(terminalRef.current)
      fitAddon.fit()
      xtermRef.current = terminal

      // Resize observer — refit when the panel resizes
      const resizeObserver = new ResizeObserver(() => {
        try { fitAddon.fit() } catch {}
      })
      resizeObserver.observe(terminalRef.current)

      // ── WebSocket connection ──────────────────────────────────────────────
      const wsBase = getOrchestratorWSBase()
      const token = await getToken()
      if (!token) {
        terminal.writeln('\r\n\x1b[31m✗ Authentication error — no session token\x1b[0m')
        return
      }

      // The Clerk token goes in the Authorization header.
      // The WebSocket API doesn't support custom headers directly in the browser,
      // so we pass it as the second WebSocket subprotocol parameter.
      // The orchestrator reads it from Sec-WebSocket-Protocol.
      // Format: "bearer.<token>" — the orchestrator strips "bearer." prefix.
      ws = new WebSocket(`${wsBase}/sessions/${sessionId}/terminal`, [`bearer.${token}`])
      wsRef.current = ws

      terminal.writeln('\x1b[2m— connecting to container —\x1b[0m')

      ws.onopen = () => {
        terminal.writeln('\x1b[32m✓ connected\x1b[0m\r\n')
        // Send initial terminal size
        const dims = { type: 'resize', rows: terminal.rows, cols: terminal.cols }
        ws.send(JSON.stringify(dims))
      }

      ws.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data as string)
          if (msg.type === 'output') {
            terminal.write(msg.data)
          } else if (msg.type === 'error') {
            terminal.writeln(`\r\n\x1b[31m✗ ${msg.data}\x1b[0m`)
          }
        } catch {
          // Not JSON — treat as raw output
          terminal.write(event.data as string)
        }
      }

      ws.onclose = (event) => {
        const reason = event.reason || 'connection closed'
        terminal.writeln(`\r\n\x1b[2m— ${reason} —\x1b[0m`)
      }

      ws.onerror = () => {
        terminal.writeln('\r\n\x1b[31m✗ WebSocket error — check orchestrator logs\x1b[0m')
      }

      // ── Input: terminal → WebSocket ───────────────────────────────────────
      terminal.onData((data) => {
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ type: 'input', data }))
        }
      })

      // ── Resize: notify orchestrator on PTY resize ─────────────────────────
      terminal.onResize(({ rows, cols }) => {
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ type: 'resize', rows, cols }))
        }
      })

      // Cleanup refs for the ResizeObserver
      return () => resizeObserver.disconnect()
    }

    init().catch(console.error)

    return () => {
      ws?.close()
      terminal?.dispose()
      xtermRef.current = null
      wsRef.current = null
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isVisible, sessionId])

  return (
    <div
      ref={terminalRef}
      style={{
        width: '100%',
        height: '100%',
        background: '#0a0a0b',
        padding: '8px 4px',
        boxSizing: 'border-box',
      }}
    />
  )
}