'use client'

import { useState } from 'react'

interface ResizableLayoutProps {
  left: React.ReactNode
  right: React.ReactNode
}

export function ResizableLayout({ left, right }: ResizableLayoutProps) {
  const [leftWidth, setLeftWidth] = useState(30) // percentage

  const handleMouseDown = (e: React.MouseEvent) => {
    e.preventDefault()
    const startX = e.clientX
    const startWidth = leftWidth

    const handleMouseMove = (e: MouseEvent) => {
      const containerWidth = window.innerWidth - 400 // approximate container width
      const delta = ((e.clientX - startX) / containerWidth) * 100
      const newWidth = Math.min(Math.max(startWidth + delta, 20), 50)
      setLeftWidth(newWidth)
    }

    const handleMouseUp = () => {
      document.removeEventListener('mousemove', handleMouseMove)
      document.removeEventListener('mouseup', handleMouseUp)
    }

    document.addEventListener('mousemove', handleMouseMove)
    document.addEventListener('mouseup', handleMouseUp)
  }

  return (
    <div style={{ display: 'flex', height: '100%', width: '100%' }}>
      <div style={{ 
        width: `${leftWidth}%`, 
        height: '100%',
        minWidth: 200,
        overflow: 'hidden'
      }}>
        {left}
      </div>
      
      <div
        onMouseDown={handleMouseDown}
        style={{
          width: 4,
          cursor: 'col-resize',
          background: 'var(--border)',
          opacity: 0.5,
          transition: 'opacity 0.15s',
          flexShrink: 0
        }}
      />
      
      <div style={{ 
        flex: 1, 
        height: '100%',
        minWidth: 0,
        overflow: 'hidden'
      }}>
        {right}
      </div>
    </div>
  )
}

interface ResizableEditorProps {
  explorer: React.ReactNode
  editor: React.ReactNode
}

export function ResizableEditor({ explorer, editor }: ResizableEditorProps) {
  const [explorerWidth, setExplorerWidth] = useState(20) // percentage

  const handleMouseDown = (e: React.MouseEvent) => {
    e.preventDefault()
    const startX = e.clientX
    const startWidth = explorerWidth

    const handleMouseMove = (e: MouseEvent) => {
      const containerWidth = window.innerWidth - 400
      const delta = ((e.clientX - startX) / containerWidth) * 100
      const newWidth = Math.min(Math.max(startWidth + delta, 12), 40)
      setExplorerWidth(newWidth)
    }

    const handleMouseUp = () => {
      document.removeEventListener('mousemove', handleMouseMove)
      document.removeEventListener('mouseup', handleMouseUp)
    }

    document.addEventListener('mousemove', handleMouseMove)
    document.addEventListener('mouseup', handleMouseUp)
  }

  return (
    <div style={{ display: 'flex', height: '100%', width: '100%' }}>
      <div style={{ 
        width: `${explorerWidth}%`, 
        height: '100%',
        minWidth: 150,
        overflow: 'hidden'
      }}>
        {explorer}
      </div>
      
      <div
        onMouseDown={handleMouseDown}
        style={{
          width: 2,
          cursor: 'col-resize',
          background: 'var(--border)',
          opacity: 0.3,
          flexShrink: 0
        }}
      />
      
      <div style={{ 
        flex: 1, 
        height: '100%',
        minWidth: 0,
        overflow: 'hidden'
      }}>
        {editor}
      </div>
    </div>
  )
}