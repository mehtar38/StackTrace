'use client'

// components/challenge/FileExplorer.tsx
// VS Code-style file tree. Renders file names from the tree at all times.
// When isLocked=true, selection is disabled and the tree is visually dimmed
// so the user can see the structure but not access contents.

import type { FileNode } from '@/lib/types'

interface FileExplorerProps {
  files: FileNode[]
  selectedPath: string | null
  onSelect: (file: FileNode) => void
  isLocked: boolean
}

const LANGUAGE_ICON: Record<string, string> = {
  javascript:  '󰌞',
  typescript:  '󰛦',
  json:        '󰘦',
  markdown:    '󰍔',
  python:      '󰌠',
  go:          '󰟓',
  plaintext:   '󰈙',
}

const EXTENSION_ICON: Record<string, string> = {
  '.js':   '󰌞',
  '.ts':   '󰛦',
  '.jsx':  '󰌞',
  '.tsx':  '󰛦',
  '.json': '󰘦',
  '.md':   '󰍔',
  '.py':   '󰌠',
  '.go':   '󰟓',
  '.sh':   '󰆍',
  '.env':  '󰙪',
}

function getIcon(file: FileNode): string {
  if (file.type === 'directory') return '󰉋'
  if (file.language && LANGUAGE_ICON[file.language]) return LANGUAGE_ICON[file.language]
  const ext = '.' + file.name.split('.').pop()
  return EXTENSION_ICON[ext] ?? '󰈙'
}

export function FileExplorer({ files, selectedPath, onSelect, isLocked }: FileExplorerProps) {
  return (
    <div style={{
      borderBottom: '1px solid var(--border)',
      background: 'var(--bg-surface)',
      flexShrink: 0,
      maxHeight: 200,
      overflowY: 'auto',
    }}>
      {/* Header */}
      <div style={{
        padding: '6px 12px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
      }}>
        <span style={{
          fontSize: 10,
          fontWeight: 600,
          letterSpacing: '0.08em',
          textTransform: 'uppercase',
          color: 'var(--text-muted)',
          fontFamily: 'var(--font-sans)',
        }}>
          Explorer
        </span>
        {isLocked && (
          <span style={{
            fontSize: 10,
            color: 'var(--text-muted)',
            fontFamily: 'var(--font-mono)',
          }}>
            locked
          </span>
        )}
      </div>

      {/* File list */}
      <div style={{ paddingBottom: 4 }}>
        {files.map(file => (
          <FileRow
            key={file.path}
            file={file}
            isSelected={selectedPath === file.path}
            isLocked={isLocked}
            onSelect={onSelect}
          />
        ))}
      </div>
    </div>
  )
}

function FileRow({
  file,
  isSelected,
  isLocked,
  onSelect,
}: {
  file: FileNode
  isSelected: boolean
  isLocked: boolean
  onSelect: (f: FileNode) => void
}) {
  const depth = file.path.split('/').length - 2 // indent by directory depth
  const indent = Math.max(0, depth) * 12 + 12

  return (
    <div
      onClick={() => {
        if (!isLocked && file.type === 'file') onSelect(file)
      }}
      title={isLocked ? 'Start the challenge to access files' : file.path}
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 6,
        padding: `4px 12px 4px ${indent}px`,
        cursor: isLocked ? 'not-allowed' : file.type === 'file' ? 'pointer' : 'default',
        background: isSelected && !isLocked ? 'var(--bg-active)' : 'transparent',
        opacity: isLocked ? 0.45 : 1,
        transition: 'background 0.1s, opacity 0.15s',
        userSelect: 'none',
      }}
      onMouseEnter={e => {
        if (!isLocked && file.type === 'file') {
          e.currentTarget.style.background = isSelected
            ? 'var(--bg-active)'
            : 'var(--bg-elevated)'
        }
      }}
      onMouseLeave={e => {
        e.currentTarget.style.background = isSelected && !isLocked
          ? 'var(--bg-active)'
          : 'transparent'
      }}
    >
      <span style={{
        fontSize: 12,
        color: file.type === 'directory' ? '#fbbf24' : 'var(--text-muted)',
        fontFamily: 'var(--font-mono)',
        lineHeight: 1,
        flexShrink: 0,
      }}>
        {getIcon(file)}
      </span>
      <span style={{
        fontSize: 12,
        color: isSelected && !isLocked ? 'var(--text-primary)' : 'var(--text-secondary)',
        fontFamily: 'var(--font-mono)',
        whiteSpace: 'nowrap',
        overflow: 'hidden',
        textOverflow: 'ellipsis',
      }}>
        {file.name}
      </span>
    </div>
  )
}