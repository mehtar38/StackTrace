'use client'

import { useState, useCallback, useEffect, useRef } from 'react'
import dynamic from 'next/dynamic'
import type { Challenge, FileContent, FileNode, AIMessage } from '@/lib/types'
import { DIFFICULTY_META } from '@/lib/constants'
import { HintsPanel } from '@/components/challenge/HintsPanel'
import { ProblemPanel } from '@/components/challenge/ProblemPanel'
import { AIPanel } from '@/components/challenge/aiPanel'
import { FileExplorer } from '@/components/challenge/FileExplorer'
import { ResizableLayout, ResizableEditor } from './ResizeableLayout'
import Link from 'next/link'

const MonacoEditor = dynamic(() => import('@monaco-editor/react'), {
  ssr: false,
  loading: () => <div style={{ flex: 1, background: '#1e1e1e' }} />,
})

type LeftPanel = 'problem' | 'hints' | 'ai'

interface ChallengeIDEProps {
  challenge: Challenge
  fileTree: FileNode[]
  fileContents: FileContent[]
}

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
  const diff = DIFFICULTY_META[challenge.difficulty]

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
      id: crypto.randomUUID(), role: 'user', content, timestamp: new Date().toISOString(),
    }
    setAiInput('')
    setAiMessages(prev => [...prev, userMessage])
    setAiLoading(true)
    await new Promise(resolve => setTimeout(resolve, 800))
    const assistantMessage: AIMessage = {
      id: crypto.randomUUID(), role: 'assistant', content: "That's a good question...", timestamp: new Date().toISOString(),
    }
    setAiMessages(prev => [...prev, assistantMessage])
    setAiLoading(false)
  }, [aiInput, aiLoading])

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
    <div style={{
      height: '100vh',
      display: 'flex',
      flexDirection: 'column',
      background: 'var(--bg-base)',
      overflow: 'hidden',
      position: 'relative'
    }}>
      <TopBar
        title={challenge.title}
        category={challenge.category}
        diffLabel={diff.label}
        diffColor={diff.color}
        elapsed={elapsed}
        estimatedMins={challenge.estimatedMins}
      />

      {/* 🔑 Middle area: explicit flex-1, min-h-0, overflow-hidden */}
      <div style={{ flex: 1, minHeight: 0, display: 'flex', overflow: 'hidden' }}>
<ResizableLayout
  left={
    <div style={{ height: '100vh', overflow: 'hidden' }}>
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
    </div>
  }
  right={
    <div style={{ height: '100vh', overflow: 'hidden' }}>
      <ResizableEditor
        explorer={
          <div style={{ height: '100%' }}>
            <FileExplorer tree={fileTree} activePath={activePath} onFileSelect={setActivePath} />
          </div>
        }
        editor={
          <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
            <EditorArea
              activePath={activePath}
              content={editedContents[activePath] ?? ''}
              language={activeFile?.language ?? 'plaintext'}
              readonly={activeFile?.readonly ?? false}
              onFullscreen={() => setIsFullscreen(true)}
              onChange={handleFileChange}
            />
            <TerminalStrip open={terminalOpen} onToggle={() => setTerminalOpen(o => !o)} />
          </div>
        }
              />
    </div>
  }
        />
      </div>

      <StatusBar runtime={challenge.environment.runtime} port={challenge.environment.port} />
    </div>
  )
}

// ─── Sub-components ────────────────────────────────────────────────────────────

function TopBar({ title, category, diffLabel, diffColor, elapsed, estimatedMins }: {
  title: string; category: string; diffLabel: string; diffColor: string; elapsed: number; estimatedMins: number
}) {
  const minutes = Math.floor(elapsed / 60).toString().padStart(2, '0')
  const seconds = (elapsed % 60).toString().padStart(2, '0')
  return (
    <header style={{ height: 48, background: 'var(--bg-surface)', borderBottom: '1px solid var(--border)', display: 'flex', alignItems: 'center', padding: '0 16px', gap: 12, flexShrink: 0 }}>
      <Link href="/" style={{ textDecoration: 'none', color: 'inherit' }}>stack<span style={{ color: 'var(--accent)' }}>trace</span></Link>
      <span style={{ color: 'var(--border-strong)', fontSize: 16, userSelect: 'none' }}>/</span>
      <span style={{ color: 'var(--text-primary)', fontSize: 13 }}>{title}</span>
      <span style={{ color: diffColor, fontSize: 12 }}>{diffLabel}</span>
      <span style={{ background: 'var(--bg-active)', border: '1px solid var(--border)', color: 'var(--text-secondary)', padding: '2px 8px', borderRadius: 'var(--radius-sm)', fontSize: 11, fontFamily: 'var(--font-mono)' }}>{category}</span>
      <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: 16 }}>
        <span style={{ fontFamily: 'var(--font-mono)', fontSize: 13, color: 'var(--text-secondary)' }}>{minutes}:{seconds}<span style={{ color: 'var(--text-muted)' }}> / {estimatedMins}m</span></span>
        <button style={{ background: 'var(--accent-muted)', border: '1px solid var(--accent-border)', color: 'var(--accent)', padding: '6px 16px', borderRadius: 'var(--radius-md)', cursor: 'pointer', fontSize: 13 }}>Submit fix</button>
      </div>
    </header>
  )
}

function LeftPanel({ panel, onPanelChange, challenge, revealedHints, onRevealHint, aiMessages, aiInput, aiLoading, onAIInput, onAISend, totalHints }: {
  panel: LeftPanel; onPanelChange: (p: LeftPanel) => void; challenge: Challenge; revealedHints: number[];
  onRevealHint: (i: number) => void; aiMessages: AIMessage[]; aiInput: string; aiLoading: boolean;
  onAIInput: (v: string) => void; onAISend: () => void; totalHints: number
}) {
  return (
    <aside style={{ height: '100%', display: 'flex', flexDirection: 'column', borderRight: '1px solid var(--border)', background: 'var(--bg-surface)' }}>
      <nav style={{ display: 'flex', borderBottom: '1px solid var(--border)', background: 'var(--bg-surface)', flexShrink: 0 }}>
        {['problem', 'hints', 'ai'].map(id => (
          <button key={id} onClick={() => onPanelChange(id as LeftPanel)} style={{
            flex: 1, padding: '10px 0', fontSize: 12, cursor: 'pointer', border: 'none', background: 'transparent',
            color: panel === id ? 'var(--text-primary)' : 'var(--text-muted)',
            borderBottom: panel === id ? '1px solid var(--accent)' : '1px solid transparent'
          }}>
            {id.charAt(0).toUpperCase() + id.slice(1)} {id === 'hints' && `(${revealedHints.length}/${totalHints})`}
          </button>
        ))}
      </nav>
      <div style={{ flex: 1, overflowY: 'auto', padding: 20 }}>
        {panel === 'problem' && <ProblemPanel challenge={challenge} />}
        {panel === 'hints' && <HintsPanel hints={challenge.hints} revealedIndices={revealedHints} onReveal={onRevealHint} />}
        {panel === 'ai' && <AIPanel messages={aiMessages} input={aiInput} onInput={onAIInput} onSend={onAISend} isLoading={aiLoading} />}
      </div>
    </aside>
  )
}

// 🔑 Monaco wrapper with bulletproof layout sync
function EditorArea({ activePath, content, language, readonly, onFullscreen, onChange }: {
  activePath: string; content: string; language: string; readonly: boolean; onFullscreen: () => void; onChange: (v?: string) => void
}) {
  const containerRef = useRef<HTMLDivElement>(null)
  const editorRef = useRef<any>(null)

  // Force layout on mount & resize
  useEffect(() => {
    const el = containerRef.current
    if (!el || !editorRef.current) return

    const observer = new ResizeObserver(() => {
      editorRef.current.layout()
    })
    observer.observe(el)
    return () => observer.disconnect()
  }, [activePath])

  return (
    <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
      <EditorToolbar activePath={activePath} readonly={readonly} onFullscreen={onFullscreen} />
      {/* 🔑 Monaco container: explicit relative, overflow-hidden, flex-1, min-h-0 */}
      <div ref={containerRef} style={{ flex: 1, position: 'relative', minHeight: 0, minWidth: 0, overflow: 'hidden' }}>
        <MonacoEditor
          height="100%"
          width="100%"
          language={language}
          value={content}
          onChange={onChange}
          theme="vs-dark"
          onMount={(editor) => {
            editorRef.current = editor
            // Microtask ensures DOM is painted before layout
            queueMicrotask(() => editor.layout())
          }}
          options={{
            automaticLayout: true,
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
  )
}

function EditorToolbar({ activePath, readonly, onFullscreen }: {
  activePath: string; readonly: boolean; onFullscreen: () => void
}) {
  const fileName = activePath.split('/').pop() ?? ''
  return (
    <div style={{ display: 'flex', alignItems: 'center', padding: '6px 12px', gap: 8, borderBottom: '1px solid var(--border)', background: 'var(--bg-surface)', flexShrink: 0 }}>
      <span style={{ fontSize: 12, fontFamily: 'var(--font-mono)', color: 'var(--text-secondary)' }}>{fileName}</span>
      {readonly && <span style={{ fontSize: 10, color: 'var(--text-muted)', background: 'var(--bg-active)', border: '1px solid var(--border)', padding: '1px 6px', borderRadius: 'var(--radius-sm)' }}>readonly</span>}
      <button onClick={onFullscreen} style={{ marginLeft: 'auto', background: 'transparent', border: '1px solid var(--border)', color: 'var(--text-muted)', padding: '3px 10px', borderRadius: 'var(--radius-sm)', cursor: 'pointer', fontSize: 11 }}>⛶ fullscreen</button>
    </div>
  )
}

function FullscreenEditor({ fileTree, activePath, content, language, readonly, onFileSelect, onChange, onExit }: {
  fileTree: FileNode[]; activePath: string; content: string; language: string; readonly: boolean;
  onFileSelect: (path: string) => void; onChange: (v?: string) => void; onExit: () => void
}) {
  return (
    <div style={{ height: '100vh', display: 'flex', flexDirection: 'column', background: 'var(--bg-base)' }}>
      <div style={{ height: 34, background: 'var(--bg-surface)', borderBottom: '1px solid var(--border)', display: 'flex', alignItems: 'center', padding: '0 12px', gap: 8, flexShrink: 0 }}>
        <button onClick={onExit} style={{ background: 'transparent', border: '1px solid var(--border)', color: 'var(--text-muted)', padding: '3px 10px', borderRadius: 'var(--radius-sm)', cursor: 'pointer', fontSize: 11 }}>⊠ exit fullscreen</button>
      </div>
      <div style={{ flex: 1, display: 'flex', overflow: 'hidden' }}>
        <FileExplorer tree={fileTree} activePath={activePath} onFileSelect={onFileSelect} />
        <div style={{ flex: 1, position: 'relative', overflow: 'hidden' }}>
          <MonacoEditor height="100%" language={language} value={content} onChange={onChange} theme="vs-dark" options={{ automaticLayout: true, fontSize: 13, minimap: { enabled: false }, readOnly: readonly }} />
        </div>
      </div>
    </div>
  )
}

function TerminalStrip({ open, onToggle }: { open: boolean; onToggle: () => void }) {
  return (
    <div style={{ borderTop: '1px solid var(--border)', background: 'var(--bg-surface)', height: open ? 180 : 32, transition: 'height 0.2s ease', display: 'flex', flexDirection: 'column', flexShrink: 0 }}>
      <button onClick={onToggle} style={{ height: 32, display: 'flex', alignItems: 'center', padding: '0 12px', gap: 8, cursor: 'pointer', background: 'transparent', border: 'none', borderBottom: open ? '1px solid var(--border)' : 'none', width: '100%', textAlign: 'left' }}>
        <span style={{ fontSize: 11, color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>terminal</span>
        <span style={{ fontSize: 10, color: 'var(--text-muted)', marginLeft: 'auto' }}>{open ? '▾' : '▸'}</span>
      </button>
      {open && <div style={{ flex: 1, padding: '8px 12px', fontFamily: 'var(--font-mono)', fontSize: 12, color: 'var(--text-secondary)', overflowY: 'auto', lineHeight: 1.6 }}>
        <span style={{ color: 'var(--success)' }}>$</span>
        <span style={{ color: 'var(--text-muted)' }}> Terminal connects when your session starts...</span>
      </div>}
    </div>
  )
}

function StatusBar({ runtime, port }: { runtime: string; port: number }) {
  return (
    <div style={{ height: 24, background: 'var(--accent-dim)', display: 'flex', alignItems: 'center', padding: '0 12px', gap: 16, flexShrink: 0 }}>
      {[runtime, `Port ${port}`, 'Session active'].map(item => (
        <span key={item} style={{ fontSize: 11, color: 'rgba(255,255,255,0.7)', fontFamily: 'var(--font-mono)' }}>{item}</span>
      ))}
    </div>
  )
}

function initialAIMessage(): AIMessage {
  return { id: crypto.randomUUID(), role: 'assistant', content: "I'm here to help you think through this...", timestamp: new Date().toISOString() }
}