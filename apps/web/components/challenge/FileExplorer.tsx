'use client'

import { useState } from 'react'
import type { FileNode } from '@/lib/types'

interface FileExplorerProps {
  tree: FileNode[]
  activePath: string
  onFileSelect: (path: string) => void
}

export function FileExplorer({ tree, activePath, onFileSelect }: FileExplorerProps) {
  return (
    <div style={{
      height: '100%',
      background: 'var(--bg-surface)',
      overflowY: 'auto',
      userSelect: 'none',
    }}>
      <div style={{
        padding: '8px 0 4px 12px',
        fontSize: 11,
        fontWeight: 500,
        color: 'var(--text-muted)',
        letterSpacing: '0.08em',
        textTransform: 'uppercase',
        fontFamily: 'var(--font-sans)',
      }}>
        Explorer
      </div>
      {tree.map(node => (
        <FileTreeNode
          key={node.path}
          node={node}
          depth={0}
          activePath={activePath}
          onSelect={onFileSelect}
        />
      ))}
    </div>
  )
}

interface FileTreeNodeProps {
  node: FileNode
  depth: number
  activePath: string
  onSelect: (path: string) => void
}

function FileTreeNode({ node, depth, activePath, onSelect }: FileTreeNodeProps) {
  const [expanded, setExpanded] = useState(true)
  const isActive = node.path === activePath
  const isDir = node.type === 'directory'
  const indent = 12 + depth * 16

  const handleClick = () => {
    if (isDir) setExpanded(e => !e)
    else onSelect(node.path)
  }

  return (
    <>
      <div
        onClick={handleClick}
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 4,
          padding: `2px 0 2px ${indent}px`,
          background: isActive ? 'var(--accent-muted)' : 'transparent',
          borderLeft: isActive ? '2px solid var(--accent)' : '2px solid transparent',
          cursor: 'pointer',
          fontSize: 13,
          fontFamily: 'var(--font-mono)',
          color: isActive ? 'var(--text-primary)' : 'var(--text-secondary)',
          whiteSpace: 'nowrap',
          overflow: 'hidden',
          textOverflow: 'ellipsis',
          minHeight: 22,
        }}
        onMouseEnter={e => { if (!isActive) e.currentTarget.style.background = 'var(--bg-hover)' }}
        onMouseLeave={e => { if (!isActive) e.currentTarget.style.background = 'transparent' }}
      >
        <span style={{
          fontSize: 10,
          color: 'var(--text-muted)',
          width: 12,
          flexShrink: 0,
          display: 'inline-block',
          transition: 'transform 0.15s ease',
          transform: isDir ? (expanded ? 'rotate(90deg)' : 'rotate(0deg)') : 'none',
          opacity: isDir ? 1 : 0,
        }}>
          ›
        </span>
        <FileIconDot node={node} isExpanded={expanded} />
        <span style={{ overflow: 'hidden', textOverflow: 'ellipsis' }}>{node.name}</span>
      </div>
      {isDir && expanded && node.children?.map(child => (
        <FileTreeNode
          key={child.path}
          node={child}
          depth={depth + 1}
          activePath={activePath}
          onSelect={onSelect}
        />
      ))}
    </>
  )
}

const FILE_TYPE_MAP: Record<string, string> = {
  js: '#f7df1e', jsx: '#61dafb', ts: '#3178c6', tsx: '#3178c6',
  json: '#8bc34a', md: '#7b8cde', yaml: '#f97316', yml: '#f97316',
  py: '#3572a5', go: '#00acd7', sql: '#e38c00', env: '#6a9955',
  sh: '#89d185', toml: '#9c4221',
}

function FileIconDot({ node, isExpanded }: { node: FileNode; isExpanded?: boolean }) {
  if (node.type === 'directory') {
    return (
      <span style={{ width: 14, flexShrink: 0, fontSize: 12 }}>
        {isExpanded ? '▿' : '▹'}
      </span>
    )
  }
  const ext = node.name.split('.').pop() ?? ''
  const color = FILE_TYPE_MAP[ext] ?? 'var(--text-muted)'
  return (
    <span style={{
      width: 8, height: 8, borderRadius: 2,
      background: color, flexShrink: 0, display: 'inline-block', marginRight: 2,
    }} />
  )
}