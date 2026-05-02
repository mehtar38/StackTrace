interface StackBadgeProps {
  name: string
}

export function StackBadge({ name }: StackBadgeProps) {
  return (
    <span style={{
      background: 'var(--bg-active)',
      border: '1px solid var(--border)',
      color: 'var(--text-secondary)',
      padding: '2px 8px',
      borderRadius: 'var(--radius-sm)',
      fontSize: 11,
      fontFamily: 'var(--font-mono)',
      whiteSpace: 'nowrap' as const,
    }}>
      {name}
    </span>
  )
}