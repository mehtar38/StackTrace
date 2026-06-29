'use client'

import { useCallback, useEffect, useState } from 'react'
import dynamic from 'next/dynamic'
import Editor from '@monaco-editor/react'
import { useRouter } from 'next/navigation'
import { ResizableEditor, ResizableLayout } from './ResizeableLayout'
import { FileExplorer } from './FileExplorer'
import { ProblemPanel } from './ProblemPanel'
import { HintsPanel } from './HintsPanel'
import { AIPanel } from './aiPanel'
import { StartChallengeOverlay } from './StartChallengeOverlay'
import { useSession } from '@/hooks/UseSession'
import type { FileTreeNode } from '@/lib/api/orchestrator'
import type { Challenge, FileNode, AIMessage } from '@/lib/types'
import { useAuth } from '@clerk/nextjs'

const TerminalPanel = dynamic(
  () => import('./TerminalPanel'),
  { ssr: false, loading: () => <TerminalPlaceholder /> }
)

interface ChallengeIDEProps {
  challenge: Challenge
  fileTree: FileNode[]
}

type LeftTab = 'problem' | 'hints' | 'ai'

export function ChallengeIDE({ challenge, fileTree }: ChallengeIDEProps) {
  const router = useRouter()
  const { getToken: getClerkToken } = useAuth()

  // ── UI state ───────────────────────────────────────────────────────────────
  const [leftTab, setLeftTab] = useState<LeftTab>('problem')
  const [selectedFile, setSelectedFile] = useState<FileNode | null>(
    fileTree.find(f => f.type === 'file') ?? null
  )
  const [isTerminalOpen, setIsTerminalOpen] = useState(false)
  const [isExiting, setIsExiting] = useState(false)

  // ── File contents (fetched from orchestrator after session starts) ─────────
  const [fileContents, setFileContents] = useState<Map<string, string>>(new Map())
  const [loadingFiles, setLoadingFiles] = useState(false)
  const [liveFileTree, setLiveFileTree] = useState<FileTreeNode[]>([])

  // ── Hints state (HintsPanel is fully controlled) ───────────────────────────
  const [revealedIndices, setRevealedIndices] = useState<number[]>([])
  const handleReveal = useCallback((index: number) => {
    setRevealedIndices(prev => prev.includes(index) ? prev : [...prev, index])
  }, [])

  // ── AI state (AIPanel is fully controlled) ─────────────────────────────────
  const [aiMessages, setAiMessages] = useState<AIMessage[]>([])
  const [aiInput, setAiInput] = useState('')
  const [aiLoading, setAiLoading] = useState(false)

  const handleAISend = useCallback(async () => {
    const text = aiInput.trim()
    if (!text || aiLoading) return
    const userMsg: AIMessage = { id: crypto.randomUUID(), role: 'user', content: text }
    setAiMessages(prev => [...prev, userMsg])
    setAiInput('')
    setAiLoading(true)
    // TODO: replace with real Groq API call via AI service
    await new Promise(r => setTimeout(r, 800))
    const assistantMsg: AIMessage = {
      id: crypto.randomUUID(),
      role: 'assistant',
      content: "Think about what happens to that data between requests — what's the lifecycle of the values you're working with?",
    }
    setAiMessages(prev => [...prev, assistantMsg])
    setAiLoading(false)
  }, [aiInput, aiLoading])

  // ── Session ────────────────────────────────────────────────────────────────
  const { session, status, error, startChallenge, exitChallenge, writeFile, readFileWithToken, getFileTree } =
    useSession(challenge.id)

  // ── Load file contents once session becomes active ─────────────────────────
  // setState is called inside the async callback, not synchronously in the
  // effect body, which avoids cascading render warnings.
  useEffect(() => {
    if (status !== 'active' || !session) return
    let cancelled = false

    async function loadFiles() {
      setLoadingFiles(true)

    const token = await getClerkToken()
    if (!token) {
      console.error('[ChallengeIDE] no auth token available, aborting file load')
      setLoadingFiles(false)
      return
    }  

      // Fetch live file tree from the container
      let tree: FileTreeNode[] = []
      try {
        tree = await getFileTree()
        setLiveFileTree(tree)
      } catch (err) {
        console.error('[ChallengeIDE] failed to fetch file tree:', err)
        // Fall back to static file tree from challenge.json
        tree = fileTree.map(f => ({ name: f.name, path: f.path, type: f.type as 'file' | 'directory', language: f.language ?? 'plaintext' }))
      }

const paths = tree.filter(f => f.type === 'file').map(f => f.path)
      const CHUNK_SIZE = 30
      const entries: [string, string][] = []

      for (let i = 0; i < paths.length; i += CHUNK_SIZE) {
        if (cancelled) break

        const chunk = paths.slice(i, i + CHUNK_SIZE)
        const freshToken = await getClerkToken()
        if (!freshToken) {
          console.error('[ChallengeIDE] lost auth token mid-load, aborting remaining files')
          break
        }

        const chunkResults = await Promise.all(
          chunk.map(async (path): Promise<[string, string]> => {
            try { return [path, await readFileWithToken(path, freshToken)] }
            catch (e) {
              console.warn('[ChallengeIDE] failed to read file:', path, e)
              return [path, '']
            }
          })
        )
        entries.push(...chunkResults)
      }

      if (!cancelled) {
        setFileContents(new Map(entries))
        setLoadingFiles(false)
      }
    }

    loadFiles()
    return () => { cancelled = true }
  }, [status, session]) // eslint-disable-line react-hooks/exhaustive-deps

  // ── Monaco onChange ────────────────────────────────────────────────────────
  const handleEditorChange = useCallback((value: string | undefined) => {
    if (!selectedFile || value === undefined || status !== 'active') return
    setFileContents(prev => new Map(prev).set(selectedFile.path, value))
    writeFile(selectedFile.path, value)
  }, [selectedFile, status, writeFile])

  const handleFileSelect = useCallback((file: FileNode) => setSelectedFile(file), [])

  // ── Save & Exit ────────────────────────────────────────────────────────────
  const handleExit = useCallback(async () => {
    if (isExiting) return
    setIsExiting(true)
    try {
      await exitChallenge()
      router.push('/')
    } catch {
      setIsExiting(false)
    }
  }, [exitChallenge, router, isExiting])

  const isLocked = status !== 'active'
  const currentContent = selectedFile ? (fileContents.get(selectedFile.path) ?? '') : ''
  const currentLanguage = selectedFile?.language ?? 'plaintext'

  return (
    <div style={{
      height: '100vh',
      display: 'flex',
      flexDirection: 'column',
      background: 'var(--bg-base)',
      overflow: 'hidden',
    }}>
      <IDEHeader
        challengeTitle={challenge.title}
        status={status}
        isExiting={isExiting}
        onExit={handleExit}
      />

      <div style={{ flex: 1, overflow: 'hidden' }}>
        <ResizableLayout
          left={
            <LeftPanel
              challenge={challenge}
              activeTab={leftTab}
              onTabChange={setLeftTab}
              revealedIndices={revealedIndices}
              onReveal={handleReveal}
              aiMessages={aiMessages}
              aiInput={aiInput}
              onAIInput={setAiInput}
              onAISend={handleAISend}
              aiLoading={aiLoading}
            />
          }
          right={
            <ResizableEditor
              explorer={
              <FileExplorer
                files={liveFileTree.length > 0 ? liveFileTree : fileTree}
                selectedPath={selectedFile?.path ?? null}
                onSelect={handleFileSelect}
                isLocked={isLocked}
              />
              }
              editor={
              <div style={{height: '100%', display: 'flex', flexDirection: 'column' }}><div style={{ flex: 1, position: 'relative', overflow: 'hidden' }}>
                  {isLocked && (
                    <StartChallengeOverlay
                      status={status === 'idle' || status === 'prewarming' || status === 'error'
                        ? status : 'idle'}
                      error={error}
                      onStart={startChallenge} />
                  )}
                  <div style={{
                    height: '100%',
                    pointerEvents: isLocked ? 'none' : 'auto',
                    opacity: isLocked ? 0.35 : 1,
                    transition: 'opacity 0.2s ease',
                  }}>
                    {loadingFiles ? <EditorLoadingState /> : (
                      <Editor
                        height="100%"
                        language={currentLanguage}
                        value={isLocked ? '' : currentContent}
                        theme="vs-dark"
                        onChange={handleEditorChange}
                        options={{
                          fontSize: 13,
                          fontFamily: '"Geist Mono", "Fira Code", monospace',
                          minimap: { enabled: false },
                          scrollBeyondLastLine: false,
                          lineNumbers: 'on',
                          readOnly: isLocked,
                          wordWrap: 'on',
                          padding: { top: 12 },
                          renderLineHighlight: 'line',
                          cursorBlinking: 'smooth',
                        }} />
                    )}
                  </div>
                </div><TerminalStrip
                    sessionId={session?.sessionId ?? null}
                    isOpen={isTerminalOpen}
                    isActive={status === 'active'}
                    onToggle={() => setIsTerminalOpen(v => !v)} /></div>
          }
          />
          }
        />
      </div>
    </div>
  )
}

// ── Sub-components ─────────────────────────────────────────────────────────────

function IDEHeader({ challengeTitle, status, isExiting, onExit }: {
  challengeTitle: string
  status: string
  isExiting: boolean
  onExit: () => void
}) {
  return (
    <div style={{
      height: 44,
      borderBottom: '1px solid var(--border)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      padding: '0 16px',
      background: 'var(--bg-surface)',
      flexShrink: 0,
    }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
        <span style={{ fontFamily: 'var(--font-mono)', fontSize: 13, color: 'var(--text-primary)', letterSpacing: '-0.02em' }}>
          stack<span style={{ color: 'var(--accent)' }}>trace</span>
        </span>
        <span style={{ color: 'var(--border)', fontSize: 12 }}>╱</span>
        <span style={{ color: 'var(--text-secondary)', fontSize: 13, fontFamily: 'var(--font-sans)' }}>
          {challengeTitle}
        </span>
      </div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <SessionBadge status={status} />
        {status === 'active' && (
          <button
            onClick={onExit}
            disabled={isExiting}
            style={{
              background: 'transparent',
              border: '1px solid var(--border-strong)',
              color: isExiting ? 'var(--text-muted)' : 'var(--text-secondary)',
              padding: '4px 12px',
              borderRadius: 'var(--radius-md)',
              cursor: isExiting ? 'not-allowed' : 'pointer',
              fontSize: 12,
              fontFamily: 'var(--font-sans)',
            }}
          >
            {isExiting ? 'Saving…' : 'Save & Exit'}
          </button>
        )}
      </div>
    </div>
  )
}

function SessionBadge({ status }: { status: string }) {
  const config: Record<string, { label: string; color: string }> = {
    idle:       { label: 'Not started', color: 'var(--text-muted)' },
    prewarming: { label: 'Starting…',   color: '#fbbf24' },
    active:     { label: 'Active',      color: '#4ade80' },
    exited:     { label: 'Exited',      color: 'var(--text-muted)' },
    expired:    { label: 'Expired',     color: '#f87171' },
    error:      { label: 'Error',       color: '#f87171' },
  }
  const c = config[status] ?? config.idle
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 5 }}>
      <div style={{
        width: 6, height: 6, borderRadius: '50%', background: c.color,
        boxShadow: status === 'active' ? `0 0 6px ${c.color}` : 'none',
      }} />
      <span style={{ color: c.color, fontSize: 11, fontFamily: 'var(--font-mono)' }}>{c.label}</span>
    </div>
  )
}

function LeftPanel({
  challenge, activeTab, onTabChange,
  revealedIndices, onReveal,
  aiMessages, aiInput, onAIInput, onAISend, aiLoading,
}: {
  challenge: Challenge
  activeTab: LeftTab
  onTabChange: (t: LeftTab) => void
  revealedIndices: number[]
  onReveal: (index: number) => void
  aiMessages: AIMessage[]
  aiInput: string
  onAIInput: (v: string) => void
  onAISend: () => void
  aiLoading: boolean
}) {
  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      <div style={{ display: 'flex', borderBottom: '1px solid var(--border)', background: 'var(--bg-surface)', flexShrink: 0 }}>
        {(['problem', 'hints', 'ai'] as LeftTab[]).map(tab => (
          <button
            key={tab}
            onClick={() => onTabChange(tab)}
            style={{
              padding: '10px 16px',
              background: 'transparent',
              border: 'none',
              borderBottom: activeTab === tab ? '1px solid var(--accent)' : '1px solid transparent',
              color: activeTab === tab ? 'var(--text-primary)' : 'var(--text-muted)',
              cursor: 'pointer',
              fontSize: 12,
              fontFamily: 'var(--font-sans)',
              textTransform: 'capitalize',
              marginBottom: -1,
              transition: 'color 0.15s',
            }}
          >
            {tab === 'ai' ? 'AI Assistant' : tab.charAt(0).toUpperCase() + tab.slice(1)}
          </button>
        ))}
      </div>
      <div style={{ flex: 1, overflow: 'auto', padding: 16 }}>
        {activeTab === 'problem' && <ProblemPanel challenge={challenge} />}
        {activeTab === 'hints' && (
          <HintsPanel
            hints={challenge.hints}
            revealedIndices={revealedIndices}
            onReveal={onReveal}
          />
        )}
        {activeTab === 'ai' && (
          <AIPanel
            messages={aiMessages}
            input={aiInput}
            onInput={onAIInput}
            onSend={onAISend}
            isLoading={aiLoading}
          />
        )}
      </div>
    </div>
  )
}

function TerminalStrip({ sessionId, isOpen, isActive, onToggle }: {
  sessionId: string | null
  isOpen: boolean
  isActive: boolean
  onToggle: () => void
}) {
  return (
    <div style={{ borderTop: '1px solid var(--border)', background: 'var(--bg-surface)', flexShrink: 0 }}>
      <div
        onClick={isActive ? onToggle : undefined}
        style={{
          height: 32, display: 'flex', alignItems: 'center',
          padding: '0 12px', gap: 8,
          cursor: isActive ? 'pointer' : 'default', userSelect: 'none',
        }}
      >
        <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: isActive ? 'var(--text-secondary)' : 'var(--text-muted)' }}>
          {isOpen ? '▾' : '▸'} terminal
        </span>
        {!isActive && (
          <span style={{ fontSize: 11, color: 'var(--text-muted)', fontFamily: 'var(--font-sans)' }}>
            — start challenge to enable
          </span>
        )}
      </div>
      {isOpen && isActive && sessionId && (
        <div style={{ height: 240 }}>
          <TerminalPanel sessionId={sessionId} isVisible={isOpen} />
        </div>
      )}
    </div>
  )
}

function EditorLoadingState() {
  return (
    <div style={{
      height: '100%', display: 'flex', alignItems: 'center',
      justifyContent: 'center', color: 'var(--text-muted)',
      fontSize: 13, fontFamily: 'var(--font-mono)',
    }}>
      <span style={{ animation: 'pulse 1.5s ease-in-out infinite' }}>loading files…</span>
      <style>{`@keyframes pulse { 0%,100%{opacity:0.4} 50%{opacity:1} }`}</style>
    </div>
  )
}

function TerminalPlaceholder() {
  return (
    <div style={{
      height: 240, background: '#0a0a0b', display: 'flex',
      alignItems: 'center', justifyContent: 'center',
      color: 'var(--text-muted)', fontSize: 12, fontFamily: 'var(--font-mono)',
    }}>
      loading terminal…
    </div>
  )
}