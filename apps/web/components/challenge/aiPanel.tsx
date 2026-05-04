'use client'

import { useRef, useEffect } from 'react'
import type { AIMessage } from '@/lib/types'

interface AIPanelProps {
  messages: AIMessage[]
  input: string
  onInput: (value: string) => void
  onSend: () => void
  isLoading: boolean
}

export function AIPanel({ messages, input, onInput, onSend, isLoading }: AIPanelProps) {
  const endRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      onSend()
    }
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12, height: '100%' }}>
      <ConstraintNotice />

      <div style={{
        display: 'flex',
        flexDirection: 'column',
        gap: 10,
        flex: 1,
        overflowY: 'auto',
        minHeight: 0,
        minWidth: 0,
      }}>
        {messages.map(m => <MessageBubble key={m.id} message={m} />)}
        {isLoading && <TypingIndicator />}
        <div ref={endRef} />
      </div>

      <ChatInput
        value={input}
        onChange={onInput}
        onKeyDown={handleKeyDown}
        onSend={onSend}
        disabled={isLoading}
      />
    </div>
  )
}

function ConstraintNotice() {
  return (
    <div style={{
      background: 'var(--bg-elevated)',
      border: '1px solid var(--border)',
      borderLeft: '3px solid var(--accent)',
      borderRadius: 'var(--radius-md)',
      padding: '10px 12px',
      fontSize: 11,
      color: 'var(--text-muted)',
      lineHeight: 1.5,
      flexShrink: 0,
    }}>
      The assistant understands the full codebase. It will explain any concept or error — but will never identify the bug or suggest a fix.
    </div>
  )
}

interface MessageBubbleProps {
  message: AIMessage
}

function MessageBubble({ message }: MessageBubbleProps) {
  const isUser = message.role === 'user'
  return (
    <div style={{
      alignSelf: isUser ? 'flex-end' : 'flex-start',
      maxWidth: '88%',
      background: isUser ? 'var(--accent-muted)' : 'var(--bg-elevated)',
      border: `1px solid ${isUser ? 'var(--accent-border)' : 'var(--border)'}`,
      borderRadius: 'var(--radius-md)',
      padding: '10px 12px',
      fontSize: 13,
      color: isUser ? 'var(--text-accent)' : 'var(--text-secondary)',
      lineHeight: 1.6,
    }}>
      {message.content}
    </div>
  )
}

function TypingIndicator() {
  return (
    <div style={{
      alignSelf: 'flex-start',
      background: 'var(--bg-elevated)',
      border: '1px solid var(--border)',
      borderRadius: 'var(--radius-md)',
      padding: '10px 14px',
      display: 'flex',
      gap: 4,
      alignItems: 'center',
    }}>
      {[0, 1, 2].map(i => (
        <span key={i} style={{
          width: 5,
          height: 5,
          borderRadius: '50%',
          background: 'var(--text-muted)',
          display: 'inline-block',
          animation: `pulse 1.2s ease-in-out ${i * 0.2}s infinite`,
        }} />
      ))}
      <style>{`
        @keyframes pulse {
          0%, 100% { opacity: 0.3; transform: scale(0.8); }
          50% { opacity: 1; transform: scale(1); }
        }
      `}</style>
    </div>
  )
}

interface ChatInputProps {
  value: string
  onChange: (value: string) => void
  onKeyDown: (e: React.KeyboardEvent<HTMLInputElement>) => void
  onSend: () => void
  disabled: boolean
}

function ChatInput({ value, onChange, onKeyDown, onSend, disabled }: ChatInputProps) {
  return (
    <div style={{ display: 'flex', gap: 8, flexShrink: 0 }}>
      <input
        value={value}
        onChange={e => onChange(e.target.value)}
        onKeyDown={onKeyDown}
        placeholder="Ask about the code..."
        disabled={disabled}
        style={{
          flex: 1,
          background: 'var(--bg-elevated)',
          border: '1px solid var(--border)',
          borderRadius: 'var(--radius-md)',
          padding: '8px 12px',
          color: 'var(--text-primary)',
          fontSize: 13,
          fontFamily: 'var(--font-sans)',
          outline: 'none',
          opacity: disabled ? 0.6 : 1,
        }}
      />
      <button
        onClick={onSend}
        disabled={disabled || !value.trim()}
        style={{
          background: 'var(--accent-muted)',
          border: '1px solid var(--accent-border)',
          color: 'var(--accent)',
          padding: '8px 14px',
          borderRadius: 'var(--radius-md)',
          cursor: disabled ? 'not-allowed' : 'pointer',
          fontSize: 13,
          fontFamily: 'var(--font-sans)',
          opacity: disabled || !value.trim() ? 0.5 : 1,
          transition: 'opacity 0.15s ease',
        }}
      >
        Send
      </button>
    </div>
  )
}