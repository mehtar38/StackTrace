import type { ChallengeHint } from '@/lib/types'

interface HintsPanelProps {
  hints: ChallengeHint[]
  revealedIndices: number[]
  onReveal: (index: number) => void
}

export function HintsPanel({ hints, revealedIndices, onReveal }: HintsPanelProps) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      <p style={{ color: 'var(--text-muted)', fontSize: 12, lineHeight: 1.6, marginBottom: 4 }}>
        Hints are progressive. Reveal only what you need.
      </p>
      {hints.map((hint, i) => {
        const isRevealed = revealedIndices.includes(i)
        const isUnlockable = !isRevealed && (i === 0 || revealedIndices.includes(i - 1))
        const isLocked = !isRevealed && !isUnlockable

        return (
          <HintCard
            key={hint.order}
            hint={hint}
            index={i}
            isRevealed={isRevealed}
            isUnlockable={isUnlockable}
            isLocked={isLocked}
            onReveal={onReveal}
          />
        )
      })}
    </div>
  )
}

interface HintCardProps {
  hint: ChallengeHint
  index: number
  isRevealed: boolean
  isUnlockable: boolean
  isLocked: boolean
  onReveal: (index: number) => void
}

function HintCard({ hint, index, isRevealed, isUnlockable, isLocked, onReveal }: HintCardProps) {
  return (
    <div style={{
      background: 'var(--bg-elevated)',
      border: `1px solid ${isRevealed ? 'var(--accent-border)' : 'var(--border)'}`,
      borderRadius: 'var(--radius-md)',
      padding: '12px 14px',
      opacity: isLocked ? 0.35 : 1,
      transition: 'opacity 0.2s ease, border-color 0.2s ease',
    }}>
      <div style={{
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
        marginBottom: isRevealed ? 8 : 0,
      }}>
        <span style={{ fontSize: 11, color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
          hint {index + 1}
        </span>
        {isUnlockable && (
          <button
            onClick={() => onReveal(index)}
            style={{
              background: 'var(--accent-muted)',
              border: '1px solid var(--accent-border)',
              color: 'var(--accent)',
              padding: '3px 10px',
              borderRadius: 'var(--radius-sm)',
              cursor: 'pointer',
              fontSize: 11,
              fontFamily: 'var(--font-sans)',
            }}
          >
            Reveal
          </button>
        )}
      </div>
      {isRevealed && (
        <p style={{ color: 'var(--text-secondary)', fontSize: 13, lineHeight: 1.6, margin: 0 }}>
          {hint.text}
        </p>
      )}
    </div>
  )
}