'use client'

import { Group, Panel, Separator } from "react-resizable-panels"

interface ResizableLayoutProps {
  left: React.ReactNode
  right: React.ReactNode
}

export function ResizableLayout({ left, right }: ResizableLayoutProps) {
  return (
     <Group style={{ display: "flex", flexDirection: "row", flex: 1, overflow: "hidden" }}>
      <Panel defaultSize={30} minSize={20} maxSize={50}>
        {left}
      </Panel>

      <Separator style={{
        width: 4,
        background: 'var(--border)',
        cursor: 'col-resize',
        // transition: 'background 0.15s ease',
        flexShrink: 0,
      }}
      />

      <Panel defaultSize={70} minSize={40}>
        {right}
      </Panel>
    </Group>
  )
}

interface ResizableEditorProps {
  explorer: React.ReactNode
  editor: React.ReactNode
}

export function ResizableEditor({ explorer, editor }: ResizableEditorProps) {
  return (
    <Group style={{ display: "flex", flexDirection: "row", flex: 1, overflow: "hidden" }}>
      <Panel defaultSize={20} minSize={12} maxSize={40}>
        {explorer}
      </Panel>

      <Separator style={{
        width: 1,
        background: 'var(--border)',
        cursor: 'col-resize',
        flexShrink: 0,
      }} />

      <Panel defaultSize={80} minSize={40}>
        {editor}
      </Panel>
    </Group>
  )
}