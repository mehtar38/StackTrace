'use client'

// Owns the entire session lifecycle: prewarm → start → active → exit.
// ChallengeIDE consumes this hook; no session logic lives in components.

import { useCallback, useEffect, useRef, useState } from 'react'
import { useAuth } from '@clerk/nextjs'
import {
  prewarmSession,
  startSession,
  writeFile as apiWriteFile,
  exitSession as apiExitSession,
  readFile as apiReadFile,
  getFileTree as apiGetFileTree,
  FileTreeNode,
} from '@/lib/api/orchestrator'
import type { Session, SessionStatus } from '@/lib/types'

const ANON_TOKEN_KEY = (challengeId: string) => `st_anon_${challengeId}`
const DEBOUNCE_MS = 6000

function getOrCreateAnonToken(challengeId: string): string {
  const key = ANON_TOKEN_KEY(challengeId)
  const existing = localStorage.getItem(key)
  if (existing) return existing
  const uuid = crypto.randomUUID()
  localStorage.setItem(key, uuid)
  return uuid
}

function clearAnonToken(challengeId: string): void {
  localStorage.removeItem(ANON_TOKEN_KEY(challengeId))
}

interface UseSessionReturn {
  session: Session | null
  status: SessionStatus
  error: string | null
  startChallenge: () => Promise<void>
  exitChallenge: () => Promise<void>
  writeFile: (path: string, content: string) => void
  // readFile: (path: string) => Promise<string>
  readFileWithToken: (path: string, token: string) => Promise<string>
  getFileTree: () => Promise<FileTreeNode[]>
}

export function useSession(challengeId: string): UseSessionReturn {
  const { getToken, isLoaded, isSignedIn } = useAuth()
  const [session, setSession] = useState<Session | null>(null)
  const [status, setStatus] = useState<SessionStatus>('idle')
  const [error, setError] = useState<string | null>(null)

  const debounceTimers = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map())
  const prewarmAttempted = useRef(false)

  // ── Prewarm on mount ────────────────────────────────────────────────────────
  // Fires silently in the background. Does NOT touch status at all —
  // the overlay must show a normal button, not a spinner, until the user clicks.
  useEffect(() => {
    if (!isLoaded || !isSignedIn) return
    if (prewarmAttempted.current) return
    prewarmAttempted.current = true

    const anonToken = getOrCreateAnonToken(challengeId)

    prewarmSession(challengeId, anonToken)
      .then(() => {
        // Container is ready in the background. Status intentionally stays 'idle'.
      })
      .catch(err => {
        // Non-fatal — Start will fall back to on-demand spin-up.
        clearAnonToken(challengeId)
        prewarmAttempted.current = false
        console.warn('[useSession] prewarm failed (non-fatal):', err.message)
      })
  }, [isLoaded, isSignedIn, challengeId])

  // ── Start challenge ─────────────────────────────────────────────────────────
  const startChallenge = useCallback(async () => {
    setError(null)
    setStatus('prewarming') // spinner only appears NOW, when user clicked

    try {
      const anonToken = getOrCreateAnonToken(challengeId)
      const newSession = await startSession(anonToken, challengeId, getToken)
      setSession(newSession)
      setStatus('active')
      clearAnonToken(challengeId)
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to start session'
      setError(message)
      setStatus('error')
    }
  }, [challengeId, getToken])

  // ── Write file (debounced) ──────────────────────────────────────────────────
  const writeFile = useCallback((path: string, content: string) => {
    if (!session || status !== 'active') return

    const existing = debounceTimers.current.get(path)
    if (existing) clearTimeout(existing)

    const timer = setTimeout(async () => {
      debounceTimers.current.delete(path)
      try {
        await apiWriteFile(session.sessionId, path, content, getToken)
      } catch (err) {
        console.error('[useSession] write file failed:', err)
      }
    }, DEBOUNCE_MS)

    debounceTimers.current.set(path, timer)
  }, [session, status, getToken])

  // ── Read file ───────────────────────────────────────────────────────────────
  // const readFile = useCallback(async (path: string): Promise<string> => {
  //   if (!session) throw new Error('No active session')
  //   return apiReadFile(session.sessionId, path, getToken)
  // }, [session, getToken])

  const readFileWithToken = useCallback(async (path: string, token: string): Promise<string> => {
  if (!session) throw new Error('No active session')
  return apiReadFile(session.sessionId, path, async () => token)
}, [session])

  const getFileTree = useCallback(async () => {
  if (!session) throw new Error('No active session')
  return apiGetFileTree(session.sessionId, getToken)
  }, [session, getToken])

  // ── Exit challenge ──────────────────────────────────────────────────────────
  const exitChallenge = useCallback(async () => {
    if (!session) return

    for (const [, timer] of debounceTimers.current) clearTimeout(timer)
    debounceTimers.current.clear()

    try {
      await apiExitSession(session.sessionId, getToken)
      setStatus('exited')
      setSession(null)
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Exit failed'
      setError(message)
    }
  }, [session, getToken])

  // ── Cleanup on unmount ──────────────────────────────────────────────────────
  useEffect(() => {
    const timers = debounceTimers.current
    return () => {
      for (const [, timer] of timers) clearTimeout(timer)
    }
  }, [])

  return { session, status, error, startChallenge, exitChallenge, writeFile, readFileWithToken, getFileTree }
}