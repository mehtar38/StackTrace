'use client'

// hooks/useSession.ts
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
} from '@/lib/api/orchestrator'
import type { Session, SessionStatus } from '@/lib/types'

const ANON_TOKEN_KEY = (challengeId: string) => `st_anon_${challengeId}`
const DEBOUNCE_MS = 6000 // 6s — within the 5–8s window

function getOrCreateAnonToken(challengeId: string): string {
  const key = ANON_TOKEN_KEY(challengeId)
  const existing = localStorage.getItem(key)
  if (existing) return existing
  // Generate a v4-like UUID using the Web Crypto API (available in all modern browsers)
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
  // Called when user clicks "Start Challenge"
  startChallenge: () => Promise<void>
  // Save all dirty files and kill the container
  exitChallenge: () => Promise<void>
  // Debounced write — safe to call on every Monaco onChange
  writeFile: (path: string, content: string) => void
  // Read a single file from the container (called after session becomes active)
  readFile: (path: string) => Promise<string>
}

export function useSession(challengeId: string): UseSessionReturn {
  const { getToken } = useAuth()
  const [session, setSession] = useState<Session | null>(null)
  const [status, setStatus] = useState<SessionStatus>('idle')
  const [error, setError] = useState<string | null>(null)

  // Debounce timers keyed by file path
  const debounceTimers = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map())
  // Track if prewarm has already been initiated (StrictMode fires effects twice)
  const prewarmAttempted = useRef(false)

  // ── Prewarm on mount ────────────────────────────────────────────────────────
  useEffect(() => {
    if (prewarmAttempted.current) return
    prewarmAttempted.current = true

    const anonToken = getOrCreateAnonToken(challengeId)

    setStatus('prewarming')
    prewarmSession(challengeId, anonToken)
      .then(() => {
        // Container is warming up in the background.
        // Status stays 'prewarming' until the user clicks Start.
        // The orchestrator handles the ready state — we just fire and wait.
      })
      .catch(err => {
        // Non-fatal: prewarm failure just means Start will be slightly slower
        // (orchestrator will spin up on-demand). Log but don't block the user.
        console.warn('[useSession] prewarm failed (non-fatal):', err.message)
        setStatus('idle')
      })
  }, [challengeId])

  // ── Start challenge ─────────────────────────────────────────────────────────
  const startChallenge = useCallback(async () => {
    setError(null)
    setStatus('prewarming') // show spinner while we wait for the Start response

    try {
      const anonToken = getOrCreateAnonToken(challengeId)
      const newSession = await startSession(anonToken, getToken)
      setSession(newSession)
      setStatus('active')
      clearAnonToken(challengeId) // token is consumed; clean up localStorage
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to start session'
      setError(message)
      setStatus('error')
    }
  }, [challengeId, getToken])

  // ── Write file (debounced) ──────────────────────────────────────────────────
  const writeFile = useCallback((path: string, content: string) => {
    if (!session || status !== 'active') return

    // Cancel any pending write for this path
    const existing = debounceTimers.current.get(path)
    if (existing) clearTimeout(existing)

    const timer = setTimeout(async () => {
      debounceTimers.current.delete(path)
      try {
        await apiWriteFile(session.sessionId, path, content, getToken)
      } catch (err) {
        // Non-fatal: write failures are logged but don't interrupt the user.
        // The dirty-file set in Redis means we'll catch it on the next write.
        console.error('[useSession] write file failed:', err)
      }
    }, DEBOUNCE_MS)

    debounceTimers.current.set(path, timer)
  }, [session, status, getToken])

  // ── Read file ───────────────────────────────────────────────────────────────
  const readFile = useCallback(async (path: string): Promise<string> => {
    if (!session) throw new Error('No active session')
    return apiReadFile(session.sessionId, path, getToken)
  }, [session, getToken])

  // ── Exit challenge ──────────────────────────────────────────────────────────
  const exitChallenge = useCallback(async () => {
    if (!session) return

    // Flush all pending debounced writes before exiting
    for (const [, timer] of debounceTimers.current) {
      clearTimeout(timer)
    }
    debounceTimers.current.clear()

    try {
      await apiExitSession(session.sessionId, getToken)
      setStatus('exited')
      setSession(null)
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Exit failed'
      setError(message)
      // Don't block navigation even if exit API fails — container will expire
    }
  }, [session, getToken])

  // ── Cleanup on unmount ──────────────────────────────────────────────────────
  useEffect(() => {
    const timers = debounceTimers.current
    return () => {
      // Clear all pending debounce timers on unmount
      for (const [, timer] of timers) {
        clearTimeout(timer)
      }
    }
  }, [])

  return {
    session,
    status,
    error,
    startChallenge,
    exitChallenge,
    writeFile,
    readFile,
  }
}