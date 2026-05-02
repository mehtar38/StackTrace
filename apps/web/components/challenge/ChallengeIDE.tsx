'use client'

import { useState, useCallback, useEffect } from 'react'
import dynamic from 'next/dynamic'
import type { Challenge, FileContent, FileNode, AIMessage } from '@/lib/types'
import { DIFFICULTY_META } from '@/lib/constants'
import { ProblemPanel } from '@/components/challenge/problemPanel'
import { HintsPanel } from '@/components/challenge/HintsPanel'
import { AIPanel } from '@/components/challenge/aiPanel'
import { FileExplorer } from '@/components/challenge/FileExplorer'
import { ResizableLayout, ResizableEditor } from './resizeableLayout'
import Link from 'next/link'

const MonacoEditor = dynamic(() => import('@monaco-editor/react'), {
  ssr: false,
  loading: () => <EditorSkeleton />,
})

// ─── Types ─────────────────────────────────────────────────────────────────────

type LeftPanel = 'problem' | 'hints' | 'ai'

interface ChallengeIDEProps {
  challenge: Challenge
  fileTree: FileNode[]
  fileContents: FileContent[]
}

// ─── Main Component ────────────────────────────────────────────────────────────

export function ChallengeIDE({ challenge, fileTree, fileContents }: ChallengeIDEProps) {
  const [leftPanel, setLeftPanel] = useState<LeftPanel>('problem')
  const [activePath, setActivePath] = useState<string>(fileTree[0]?.path ?? '')
  const [editedContents, setEditedContents] = useState<Record<string, string>>(
    Object.fromEntries(fileContents.map(f => [f.path, f.content]))
  )
  const [revealedHints, setRevealedHints] = useState<number[]>([])
  const [aiMessages, setAiMessages] = useState<AIMessage[]>([initialAIMessage()])
  const [aiInput, setAiInput] = useState('')
  const [aiLoading, setAiLoading] = useState(false)
  const [isFullscreen, setIsFullscreen] = useState(false)
  const [terminalOpen, setTerminalOpen] = useState(true)
  const [elapsed, setElapsed] = useState(0)

  useEffect(() => {
    const timer = setInterval(() => setElapsed(e => e + 1), 1000)
    return () => clearInterval(timer)
  }, [])

  const activeFile = fileContents.find(f => f.path === activePath)

  const handleFileChange = useCallback((value: string | undefined) => {
    if (!activePath || value === undefined) return
    setEditedContents(prev => ({ ...prev, [activePath]: value }))
  }, [activePath])

  const handleRevealHint = useCallback((index: number) => {
    setRevealedHints(prev => (prev.includes(index) ? prev : [...prev, index]))
  }, [])

  const handleAISend = useCallback(async () => {
    const content = aiInput.trim()
    if (!content || aiLoading) return

    const userMessage: AIMessage = {
      id: crypto.randomUUID(),
      role: 'user',
      content,
      timestamp: new Date().toISOString(),
    }

    setAiInput('')
    setAiMessages(prev => [...prev, userMessage])
    setAiLoading(true)

    // TODO: replace with real API call to Python AI service
    await new Promise(resolve => setTimeout(resolve, 800))
    const assistantMessage: AIMessage = {
      id: crypto.randomUUID(),
      role: 'assistant',
      content: "That's a good question. Think about the lifecycle of a Node.js process — what exists only in memory versus what gets written to disk. What do you think happens to a JavaScript array when the process exits?",
      timestamp: new Date().toISOString(),
    }
    setAiMessages(prev => [...prev, assistantMessage])
    setAiLoading(false)
  }, [aiInput, aiLoading])

  const diff = DIFFICULTY_META[challenge.difficulty]

  if (isFullscreen) {
    return (
      <FullscreenEditor
        fileTree={fileTree}
        activePath={activePath}
        content={editedContents[activePath] ?? ''}
        language={activeFile?.language ?? 'plaintext'}
        readonly={activeFile?.readonly ?? false}
        onFileSelect={setActivePath}
        onChange={handleFileChange}
        onExit={() => setIsFullscreen(false)}
      />
    )
  }

  return (
    <div style={{ height: '100vh', display: 'flex', flexDirection: 'column', background: 'var(--bg-base)', overflow: 'hidden' }}>
      <TopBar
        title={challenge.title}
        category={challenge.category}
        diffLabel={diff.label}
        diffColor={diff.color}
        elapsed={elapsed}
        estimatedMins={challenge.estimatedMins}
      />

      <ResizableLayout
  left={
    <LeftPanel
      panel={leftPanel}
      onPanelChange={setLeftPanel}
      challenge={challenge}
      revealedHints={revealedHints}
      onRevealHint={handleRevealHint}
      aiMessages={aiMessages}
      aiInput={aiInput}
      aiLoading={aiLoading}
      onAIInput={setAiInput}
      onAISend={handleAISend}
      totalHints={challenge.hints.length}
    />
  }
        right={
    <ResizableEditor
            explorer={
              <FileExplorer
                tree={fileTree}
                activePath={activePath}
                onFileSelect={setActivePath}
              />
            }
            editor={
              <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
                <EditorArea
                  fileTree={fileTree}
                  activePath={activePath}
                  content={editedContents[activePath] ?? ''}
                  language={activeFile?.language ?? 'plaintext'}
                  readonly={activeFile?.readonly ?? false}
                  onFileSelect={setActivePath}
                  onChange={handleFileChange}
                  onFullscreen={() => setIsFullscreen(true)}
                />
                <TerminalStrip open={terminalOpen} onToggle={() => setTerminalOpen(o => !o)} />
              </div>
            }
          />
        }
      />

      <StatusBar runtime={challenge.environment.runtime} port={challenge.environment.port} />
    </div>
  )
}

// ─── Sub-components ────────────────────────────────────────────────────────────

function TopBar({ title, category, diffLabel, diffColor, elapsed, estimatedMins }: {
  title: string
  category: string
  diffLabel: string
  diffColor: string
  elapsed: number
  estimatedMins: number
}) {
  const minutes = Math.floor(elapsed / 60).toString().padStart(2, '0')
  const seconds = (elapsed % 60).toString().padStart(2, '0')

  return (
    <header style={{
      height: 48,
      background: 'var(--bg-surface)',
      borderBottom: '1px solid var(--border)',
      display: 'flex',
      alignItems: 'center',
      padding: '0 16px',
      gap: 12,
      flexShrink: 0,
    }}>
    <Link href="/" style={{ textDecoration: 'none', color: 'inherit' }}>
      stack<span style={{ color: 'var(--accent)' }}>trace</span>
    </Link>
      <span style={{ color: 'var(--border-strong)', fontSize: 16, userSelect: 'none' }}>/</span>
      <span style={{ color: 'var(--text-primary)', fontSize: 13 }}>{title}</span>
      <span style={{ color: diffColor, fontSize: 12 }}>{diffLabel}</span>
      <span style={{
        background: 'var(--bg-active)',
        border: '1px solid var(--border)',
        color: 'var(--text-secondary)',
        padding: '2px 8px',
        borderRadius: 'var(--radius-sm)',
        fontSize: 11,
        fontFamily: 'var(--font-mono)',
      }}>
        {category}
      </span>

      <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: 16 }}>
        <span style={{ fontFamily: 'var(--font-mono)', fontSize: 13, color: 'var(--text-secondary)' }}>
          {minutes}:{seconds}
          <span style={{ color: 'var(--text-muted)' }}> / {estimatedMins}m</span>
        </span>
        <button style={{
          background: 'var(--accent-muted)',
          border: '1px solid var(--accent-border)',
          color: 'var(--accent)',
          padding: '6px 16px',
          borderRadius: 'var(--radius-md)',
          cursor: 'pointer',
          fontSize: 13,
          fontFamily: 'var(--font-sans)',
        }}>
          Submit fix
        </button>
      </div>
    </header>
  )
}

function LeftPanel({ panel, onPanelChange, challenge, revealedHints, onRevealHint,
  aiMessages, aiInput, aiLoading, onAIInput, onAISend, totalHints }: {
  panel: LeftPanel
  onPanelChange: (p: LeftPanel) => void
  challenge: Challenge
  revealedHints: number[]
  onRevealHint: (i: number) => void
  aiMessages: AIMessage[]
  aiInput: string
  aiLoading: boolean
  onAIInput: (v: string) => void
  onAISend: () => void
  totalHints: number
}) {
  const tabs: { id: LeftPanel; label: string }[] = [
    { id: 'problem', label: 'Problem' },
    { id: 'hints',   label: `Hints (${revealedHints.length}/${totalHints})` },
    { id: 'ai',      label: 'AI Assistant' },
  ]

  return (
    <aside style={{
      width: 380,
      minWidth: 300,
      borderRight: '1px solid var(--border)',
      display: 'flex',
      flexDirection: 'column',
      overflow: 'hidden',
    }}>
      <nav style={{
        display: 'flex',
        borderBottom: '1px solid var(--border)',
        background: 'var(--bg-surface)',
        flexShrink: 0,
      }}>
        {tabs.map(tab => (
          <button
            key={tab.id}
            onClick={() => onPanelChange(tab.id)}
            style={{
              flex: 1,
              padding: '10px 0',
              fontSize: 12,
              fontFamily: 'var(--font-sans)',
              cursor: 'pointer',
              border: 'none',
              background: 'transparent',
              color: panel === tab.id ? 'var(--text-primary)' : 'var(--text-muted)',
              borderBottom: panel === tab.id ? '1px solid var(--accent)' : '1px solid transparent',
              transition: 'color 0.15s ease',
            }}
          >
            {tab.label}
          </button>
        ))}
      </nav>

      <div style={{ flex: 1, overflowY: 'auto', padding: 20, minHeight: 0 }}>
        {panel === 'problem' && <ProblemPanel challenge={challenge} />}
        {panel === 'hints' && (
          <HintsPanel
            hints={challenge.hints}
            revealedIndices={revealedHints}
            onReveal={onRevealHint}
          />
        )}
        {panel === 'ai' && (
          <AIPanel
            messages={aiMessages}
            input={aiInput}
            onInput={onAIInput}
            onSend={onAISend}
            isLoading={aiLoading}
          />
        )}
      </div>
    </aside>
  )
}

function EditorArea({ fileTree, activePath, content, language, readonly, onFileSelect, onChange, onFullscreen }: {
  fileTree: FileNode[]
  activePath: string
  content: string
  language: string
  readonly: boolean
  onFileSelect: (path: string) => void
  onChange: (value: string | undefined) => void
  onFullscreen: () => void
}) {
  return (
    <div style={{ flex: 1, display: 'flex', overflow: 'hidden' }}>
      <FileExplorer
        tree={fileTree}
        activePath={activePath}
        onFileSelect={onFileSelect}
      />
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        <EditorToolbar activePath={activePath} readonly={readonly} onFullscreen={onFullscreen} />
        <div style={{ flex: 1 }}>
          <MonacoEditor
            height="100%"
            language={language}
            value={content}
            onChange={onChange}
            theme="vs-dark"
            options={{
              fontSize: 13,
              fontFamily: "'Geist Mono', 'Fira Code', monospace",
              fontLigatures: true,
              minimap: { enabled: false },
              scrollBeyondLastLine: false,
              lineNumbers: 'on',
              renderLineHighlight: 'line',
              padding: { top: 16 },
              tabSize: 2,
              readOnly: readonly,
            }}
          />
        </div>
      </div>
    </div>
  )
}

function EditorToolbar({ activePath, readonly, onFullscreen }: {
  activePath: string
  readonly: boolean
  onFullscreen: () => void
}) {
  const fileName = activePath.split('/').pop() ?? ''
  return (
    <div style={{
      height: 34,
      background: 'var(--bg-surface)',
      borderBottom: '1px solid var(--border)',
      display: 'flex',
      alignItems: 'center',
      padding: '0 12px',
      gap: 8,
      flexShrink: 0,
    }}>
      <span style={{ fontSize: 12, fontFamily: 'var(--font-mono)', color: 'var(--text-secondary)' }}>
        {fileName}
      </span>
      {readonly && (
        <span style={{
          fontSize: 10,
          color: 'var(--text-muted)',
          background: 'var(--bg-active)',
          border: '1px solid var(--border)',
          padding: '1px 6px',
          borderRadius: 'var(--radius-sm)',
          fontFamily: 'var(--font-mono)',
        }}>
          readonly
        </span>
      )}
      <button
        onClick={onFullscreen}
        title="Full screen editor"
        style={{
          marginLeft: 'auto',
          background: 'transparent',
          border: '1px solid var(--border)',
          color: 'var(--text-muted)',
          padding: '3px 10px',
          borderRadius: 'var(--radius-sm)',
          cursor: 'pointer',
          fontSize: 11,
          fontFamily: 'var(--font-mono)',
        }}
      >
        ⛶ fullscreen
      </button>
    </div>
  )
}

function FullscreenEditor({ fileTree, activePath, content, language, readonly, onFileSelect, onChange, onExit }: {
  fileTree: FileNode[]
  activePath: string
  content: string
  language: string
  readonly: boolean
  onFileSelect: (path: string) => void
  onChange: (value: string | undefined) => void
  onExit: () => void
}) {
  return (
    <div style={{ height: '100vh', display: 'flex', flexDirection: 'column', background: 'var(--bg-base)' }}>
      <div style={{
        height: 34,
        background: 'var(--bg-surface)',
        borderBottom: '1px solid var(--border)',
        display: 'flex',
        alignItems: 'center',
        padding: '0 12px',
        gap: 8,
        flexShrink: 0,
      }}>
        <button onClick={onExit} style={{
          background: 'transparent',
          border: '1px solid var(--border)',
          color: 'var(--text-muted)',
          padding: '3px 10px',
          borderRadius: 'var(--radius-sm)',
          cursor: 'pointer',
          fontSize: 11,
          fontFamily: 'var(--font-mono)',
        }}>
          ⊠ exit fullscreen
        </button>
      </div>
      <div style={{ flex: 1, display: 'flex', overflow: 'hidden' }}>
        <FileExplorer tree={fileTree} activePath={activePath} onFileSelect={onFileSelect} />
        <div style={{ flex: 1 }}>
          <MonacoEditor
            height="100%"
            language={language}
            value={content}
            onChange={onChange}
            theme="vs-dark"
            options={{
              fontSize: 13,
              fontFamily: "'Geist Mono', 'Fira Code', monospace",
              fontLigatures: true,
              minimap: { enabled: false },
              scrollBeyondLastLine: false,
              lineNumbers: 'on',
              renderLineHighlight: 'line',
              padding: { top: 16 },
              tabSize: 2,
              readOnly: readonly,
            }}
          />
        </div>
      </div>
    </div>
  )
}

function TerminalStrip({ open, onToggle }: { open: boolean; onToggle: () => void }) {
  return (
    <div style={{
      borderTop: '1px solid var(--border)',
      background: 'var(--bg-surface)',
      height: open ? 180 : 32,
      transition: 'height 0.2s ease',
      display: 'flex',
      flexDirection: 'column',
      flexShrink: 0,
    }}>
      <button
        onClick={onToggle}
        style={{
          height: 32,
          display: 'flex',
          alignItems: 'center',
          padding: '0 12px',
          gap: 8,
          cursor: 'pointer',
          background: 'transparent',
          border: 'none',
          borderBottom: open ? '1px solid var(--border)' : 'none',
          width: '100%',
          textAlign: 'left',
          flexShrink: 0,
        }}
      >
        <span style={{ fontSize: 11, color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
          terminal
        </span>
        <span style={{ fontSize: 10, color: 'var(--text-muted)', marginLeft: 'auto' }}>
          {open ? '▾' : '▸'}
        </span>
      </button>
      {open && (
        <div style={{
          flex: 1,
          padding: '8px 12px',
          fontFamily: 'var(--font-mono)',
          fontSize: 12,
          color: 'var(--text-secondary)',
          overflowY: 'auto',
          lineHeight: 1.6,
        }}>
          <span style={{ color: 'var(--success)' }}>$</span>
          <span style={{ color: 'var(--text-muted)' }}> Terminal connects when your session starts...</span>
        </div>
      )}
    </div>
  )
}

function StatusBar({ runtime, port }: { runtime: string; port: number }) {
  return (
    <div style={{
      height: 24,
      background: 'var(--accent-dim)',
      display: 'flex',
      alignItems: 'center',
      padding: '0 12px',
      gap: 16,
      flexShrink: 0,
    }}>
      {[runtime, `Port ${port}`, 'Session active'].map(item => (
        <span key={item} style={{
          fontSize: 11,
          color: 'rgba(255,255,255,0.7)',
          fontFamily: 'var(--font-mono)',
        }}>
          {item}
        </span>
      ))}
    </div>
  )
}

function EditorSkeleton() {
  return (
    <div style={{
      flex: 1,
      background: '#1e1e1e',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
    }}>
      <span style={{ color: 'var(--text-muted)', fontSize: 12, fontFamily: 'var(--font-mono)' }}>
        Loading editor...
      </span>
    </div>
  )
}

function initialAIMessage(): AIMessage {
  return {
    id: crypto.randomUUID(),
    role: 'assistant',
    content: "I'm here to help you think through this. I won't tell you where the bug is, but I can explain what any part of the code does, what error messages mean, or how Node.js file I/O works. What would you like to understand?",
    timestamp: new Date().toISOString(),
  }
}