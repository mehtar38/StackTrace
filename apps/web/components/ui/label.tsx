interface LabelProps {
  children: React.ReactNode
}

export function Label({ children }: LabelProps) {
  return (
    <p style={{
      fontSize: 10,
      fontWeight: 500,
      color: 'var(--text-muted)',
      letterSpacing: '0.08em',
      textTransform: 'uppercase' as const,
      marginBottom: 8,
      fontFamily: 'var(--font-sans)',
    }}>
      {children}
    </p>
  )
}