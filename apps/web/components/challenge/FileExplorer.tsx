'use client'

import { FileLanguage, FileNode } from '@/lib/types'
import { useMemo, useState } from 'react'
import { getIconForFile, getIconForFolder } from 'vscode-icons-js'


// Accept either the static FileNode shape or the orchestrator's FileTreeNode
// shape — both have name/path/type, which is all the tree builder needs.
interface FlatEntry {
  name: string
  path: string
  type: string // 'file' | 'directory'
  language?: string
}
interface TreeNode extends FileNode {
  children: TreeNode[]
}
interface FileExplorerProps {
  files: FlatEntry[]
  selectedPath: string | null
  onSelect: (file: TreeNode) => void
  isLocked: boolean
}

function getVSCodeIcon(file: { name: string; type: string }) {
  if (file.type === 'directory') {
    return getIconForFolder(file.name) || 'default_folder.svg'
  }
  return getIconForFile(file.name) || 'default_file.svg'
}

// ── Tree building ─────────────────────────────────────────────────────────────
// The orchestrator's /tree endpoint returns every file AND directory as flat
// entries (since `find` walks the whole structure). We build a real nested
// tree from that flat list, keyed by path segments.

function buildTree(flat: FlatEntry[]): TreeNode[] {
  const root: TreeNode[] = []
  const dirMap = new Map<string, TreeNode>() // path -> node, for fast parent lookup

  // Sort so parent directories are processed before their children —
  // guarantees the parent node exists in dirMap by the time a child needs it.
  const sorted = [...flat].sort((a, b) => a.path.localeCompare(b.path))

  for (const entry of sorted) {
    const parts = entry.path.split('/')
    const node: TreeNode = {
      name: entry.name,
      path: entry.path,
      type: entry.type === 'directory' ? 'directory' : 'file',
      language: (entry.language ?? 'plaintext') as FileLanguage,
      children: [],
    }

    if (node.type === 'directory') {
      dirMap.set(entry.path, node)
    }

    if (parts.length === 1) {
      // Top-level entry
      root.push(node)
    } else {
      const parentPath = parts.slice(0, -1).join('/')
      const parent = dirMap.get(parentPath)
      if (parent) {
        parent.children.push(node)
      } else {
        // Parent directory wasn't in the flat list (shouldn't normally happen
        // since find lists every level) — fall back to root so the file is
        // still visible rather than silently dropped.
        root.push(node)
      }
    }
  }

  // Directories first, then files, alphabetically within each group —
  // matches the convention every code editor uses.
  const sortNodes = (nodes: TreeNode[]): TreeNode[] => {
    const dirs = nodes.filter(n => n.type === 'directory').sort((a, b) => a.name.localeCompare(b.name))
    const fileNodes = nodes.filter(n => n.type === 'file').sort((a, b) => a.name.localeCompare(b.name))
    for (const d of dirs) d.children = sortNodes(d.children)
    return [...dirs, ...fileNodes]
  }

  return sortNodes(root)
}

export function FileExplorer({ files, selectedPath, onSelect, isLocked }: FileExplorerProps) {
  const tree = useMemo(() => buildTree(files), [files])

  // Expanded directory paths. Top-level directories start expanded so the
  // user immediately sees some structure instead of a fully collapsed list.
  const [expanded, setExpanded] = useState<Set<string>>(
    () => new Set(tree.filter(n => n.type === 'directory').map(n => n.path))
  )

  const toggleDir = (path: string) => {
    setExpanded(prev => {
      const next = new Set(prev)
      if (next.has(path)) next.delete(path)
      else next.add(path)
      return next
    })
  }

  return (
    <div style={{
      borderBottom: '1px solid var(--border)',
      background: 'var(--bg-surface)',
      flexShrink: 0,
      maxHeight: 690,
      overflowY: 'auto',
    }}>
      <div style={{
        padding: '6px 12px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        position: 'sticky',
        top: 0,
        background: 'var(--bg-surface)',
        zIndex: 1,
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
          <span style={{ fontSize: 10, color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
            locked
          </span>
        )}
      </div>

      <div style={{ paddingBottom: 4 }}>
        {tree.map(node => (
          <TreeRow
            key={node.path}
            node={node}
            depth={0}
            selectedPath={selectedPath}
            isLocked={isLocked}
            expanded={expanded}
            onToggleDir={toggleDir}
            onSelect={onSelect}
          />
        ))}
      </div>
    </div>
  )
}

function TreeRow({
  node,
  depth,
  selectedPath,
  isLocked,
  expanded,
  onToggleDir,
  onSelect,
}: {
  node: TreeNode
  depth: number
  selectedPath: string | null
  isLocked: boolean
  expanded: Set<string>
  onToggleDir: (path: string) => void
  onSelect:  (file: TreeNode) => void
}) {
  const isDir = node.type === 'directory'
  const isOpen = expanded.has(node.path)
  const isSelected = selectedPath === node.path
  const indent = depth * 14 + 12

  const handleClick = () => {
    if (isLocked) return
    if (isDir) onToggleDir(node.path)
    else onSelect(node)
  }

  return (
    <>
      <div
        onClick={handleClick}
        title={isLocked ? 'Start the challenge to access files' : node.path}
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 5,
          padding: `3px 12px 3px ${indent}px`,
          cursor: isLocked ? 'not-allowed' : 'pointer',
          background: isSelected && !isLocked ? 'var(--bg-active)' : 'transparent',
          opacity: isLocked ? 0.45 : 1,
          transition: 'background 0.1s, opacity 0.15s',
          userSelect: 'none',
        }}
        onMouseEnter={e => {
          if (!isLocked) {
            e.currentTarget.style.background = isSelected ? 'var(--bg-active)' : 'var(--bg-elevated)'
          }
        }}
        onMouseLeave={e => {
          e.currentTarget.style.background = isSelected && !isLocked ? 'var(--bg-active)' : 'transparent'
        }}
      >
        {/* Chevron for directories, fixed-width spacer for files so file icons align */}
        <span style={{
          fontSize: 9,
          color: 'var(--text-muted)',
          width: 10,
          flexShrink: 0,
          display: 'inline-block',
          transform: isDir && isOpen ? 'rotate(90deg)' : 'rotate(0deg)',
          transition: 'transform 0.12s ease',
        }}>
          {isDir ? '▸' : ''}
        </span>

      {/* VS Code icon */}
      {!isDir && (
      <img
        src={`https://raw.githubusercontent.com/vscode-icons/vscode-icons/master/icons/${getVSCodeIcon(node)}`}
        alt=""
        style={{
          width: 14,
          height: 14,
          flexShrink: 0,
          opacity: isLocked ? 0.45 : 1,
        }}
      />

      )}

      {/* Folder icon */}
      {isDir && (
      <img
        src={`https://raw.githubusercontent.com/vscode-icons/vscode-icons/master/icons/${getVSCodeIcon(node)}`}
        alt=""
        style={{
          width: 14,
          height: 14,
          flexShrink: 0,
          opacity: isLocked ? 0.45 : 1,
        }}
      />
      )}


        <span style={{
          fontSize: 12,
          color: isSelected && !isLocked ? 'var(--text-primary)' : 'var(--text-secondary)',
          fontFamily: 'var(--font-mono)',
          whiteSpace: 'nowrap',
          overflow: 'hidden',
          textOverflow: 'ellipsis',
        }}>
          {node.name}
        </span>
      </div>

      {isDir && isOpen && node.children.map(child => (
        <TreeRow
          key={child.path}
          node={child}
          depth={depth + 1}
          selectedPath={selectedPath}
          isLocked={isLocked}
          expanded={expanded}
          onToggleDir={onToggleDir}
          onSelect={onSelect}
        />
      ))}
    </>
  )
}