// lib/api/orchestrator.ts
// All HTTP calls to the Go orchestrator.
// Never imported server-side — orchestrator is called client-side only
// because it requires a Clerk session token from the browser.

import type { Session } from '@/lib/types'

const ORCHESTRATOR_URL = process.env.NEXT_PUBLIC_ORCHESTRATOR_URL ?? 'http://localhost:8080'

// Derive WebSocket base URL from the HTTP base URL.
// http://  → ws://
// https:// → wss://
export function getOrchestratorWSBase(): string {
  return ORCHESTRATOR_URL.replace('https://', 'wss://').replace('http://', 'ws://')
}

// ── Auth header helper ────────────────────────────────────────────────────────

async function authHeaders(getToken: () => Promise<string | null>): Promise<HeadersInit> {
  const token = await getToken()
  if (!token) throw new Error('No Clerk session token available')
  return {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${token}`,
  }
}

// ── Prewarm ───────────────────────────────────────────────────────────────────
// No auth required. Called as soon as the challenge page mounts.

export async function prewarmSession(
  challengeId: string,
  anonToken: string
): Promise<{ sessionId: string }> {
  const res = await fetch(`${ORCHESTRATOR_URL}/prewarm`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ challenge_id: challengeId, anon_token: anonToken }),
  })

  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error ?? `Prewarm failed: HTTP ${res.status}`)
  }

  const data = await res.json()
  return { sessionId: data.session_id }
}

// ── Start session ─────────────────────────────────────────────────────────────
// Promotes the pre-warmed container to a real session. Requires Clerk token.

export async function startSession(
  anonToken: string,
  challengeId: string,
  getToken: () => Promise<string | null>
): Promise<Session> {
  const res = await fetch(`${ORCHESTRATOR_URL}/sessions`, {
    method: 'POST',
    headers: await authHeaders(getToken),
    body: JSON.stringify({ anon_token: anonToken, challenge_id: challengeId }),
  })

  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error ?? `Start session failed: HTTP ${res.status}`)
  }

  const data = await res.json()
  return {
    sessionId: data.session_id,
    containerHost: data.container_host,
    challengeId: data.challenge_id,
    terminalWSURL: data.terminal_ws_url,
  }
}

// ── Write file ────────────────────────────────────────────────────────────────
// Called by Monaco's debounced onChange. Full file content each time.

export async function writeFile(
  sessionId: string,
  filePath: string,
  content: string,
  getToken: () => Promise<string | null>
): Promise<void> {
  const res = await fetch(`${ORCHESTRATOR_URL}/sessions/${sessionId}/files`, {
    method: 'POST',
    headers: await authHeaders(getToken),
    body: JSON.stringify({ file_path: filePath, content }),
  })

  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error ?? `Write file failed: HTTP ${res.status}`)
  }
}

// ── Read file ─────────────────────────────────────────────────────────────────
// Called once per file when the session becomes active (editor hydration).

export async function readFile(
  sessionId: string,
  filePath: string,
  getToken: () => Promise<string | null>
): Promise<string> {
  const params = new URLSearchParams({ path: filePath })
  const res = await fetch(`${ORCHESTRATOR_URL}/sessions/${sessionId}/files?${params}`, {
    method: 'GET',
    headers: await authHeaders(getToken),
  })

  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error ?? `Read file failed: HTTP ${res.status}`)
  }

  const data = await res.json()
  return data.content as string
}

// ── Exit session ──────────────────────────────────────────────────────────────
// Save & Exit — saves dirty files to Supabase, kills the container.

export async function exitSession(
  sessionId: string,
  getToken: () => Promise<string | null>
): Promise<{ savedFiles: string[] }> {
  const res = await fetch(`${ORCHESTRATOR_URL}/sessions/${sessionId}`, {
    method: 'DELETE',
    headers: await authHeaders(getToken),
  })

  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error ?? `Exit session failed: HTTP ${res.status}`)
  }

  const data = await res.json()
  return { savedFiles: data.saved_files ?? [] }
}

// ── Resume session ────────────────────────────────────────────────────────────

export async function resumeSession(
  previousSessionId: string,
  challengeId: string,
  getToken: () => Promise<string | null>
): Promise<Session> {
  const res = await fetch(`${ORCHESTRATOR_URL}/sessions/${previousSessionId}/resume`, {
    method: 'POST',
    headers: await authHeaders(getToken),
    body: JSON.stringify({ challenge_id: challengeId }),
  })

  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error ?? `Resume session failed: HTTP ${res.status}`)
  }

  const data = await res.json()
  return {
    sessionId: data.session_id,
    containerHost: data.container_host,
    challengeId: data.challenge_id,
    terminalWSURL: data.terminal_ws_url,
  }
}

// ── File tree ─────────────────────────────────────────────────────────────────
// Fetches the live file tree from inside the container.
// Returns paths relative to /app (e.g. "examples/web-service/index.js")

export interface FileTreeNode {
  name: string
  path: string
  type: 'file' | 'directory'
  language: string
}

export async function getFileTree(
  sessionId: string,
  getToken: () => Promise<string | null>
): Promise<FileTreeNode[]> {
  const res = await fetch(`${ORCHESTRATOR_URL}/sessions/${sessionId}/tree`, {
    method: 'GET',
    headers: await authHeaders(getToken),
  })

  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error ?? `File tree failed: HTTP ${res.status}`)
  }

  const data = await res.json()
  return data.files as FileTreeNode[]
}