'use client'

// components/challenge/StartChallengeOverlay.tsx
// Renders over the editor area when the session is not yet active.
// The editor behind it is pointer-events:none so files can't be accessed.

interface StartChallengeOverlayProps {
  status: 'idle' | 'prewarming' | 'error'
  error: string | null
  onStart: () => void
}

export function StartChallengeOverlay({ status, error, onStart }: StartChallengeOverlayProps) {
  const isLoading = status === 'prewarming'

  return (
    <div style={{
      position: 'absolute',
      inset: 0,
      zIndex: 20,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      // Frosted glass over the editor
      background: 'rgba(10, 10, 11, 0.72)',
      backdropFilter: 'blur(6px)',
      WebkitBackdropFilter: 'blur(6px)',
    }}>
      <div style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        gap: 16,
        padding: '32px 40px',
        background: 'var(--bg-surface)',
        border: '1px solid var(--border)',
        borderRadius: 'var(--radius-lg)',
        maxWidth: 360,
        textAlign: 'center',
      }}>
        {/* Lock icon */}
        <div style={{
          width: 40,
          height: 40,
          borderRadius: '50%',
          background: 'var(--bg-elevated)',
          border: '1px solid var(--border)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          fontSize: 18,
        }}>
          🔒
        </div>

        <div>
          <p style={{
            color: 'var(--text-primary)',
            fontSize: 14,
            fontWeight: 500,
            marginBottom: 6,
            fontFamily: 'var(--font-sans)',
          }}>
            Files are locked
          </p>
          <p style={{
            color: 'var(--text-muted)',
            fontSize: 12,
            fontFamily: 'var(--font-sans)',
            lineHeight: 1.5,
          }}>
            Start the challenge to access the codebase.
            {status === 'prewarming' && !isLoading
              ? ' Your container is ready.'
              : ''}
          </p>
        </div>

        {error && (
          <p style={{
            color: '#f87171',
            fontSize: 12,
            fontFamily: 'var(--font-mono)',
            background: 'rgba(248,113,113,0.08)',
            border: '1px solid rgba(248,113,113,0.2)',
            borderRadius: 'var(--radius-sm)',
            padding: '6px 10px',
            width: '100%',
          }}>
            {error}
          </p>
        )}

        <button
          onClick={onStart}
          disabled={isLoading}
          style={{
            background: isLoading ? 'var(--bg-elevated)' : 'var(--accent)',
            color: isLoading ? 'var(--text-muted)' : '#000',
            border: 'none',
            padding: '9px 24px',
            borderRadius: 'var(--radius-md)',
            cursor: isLoading ? 'not-allowed' : 'pointer',
            fontSize: 13,
            fontWeight: 500,
            fontFamily: 'var(--font-sans)',
            transition: 'all 0.15s ease',
            display: 'flex',
            alignItems: 'center',
            gap: 8,
            minWidth: 160,
            justifyContent: 'center',
          }}
        >
          {isLoading ? (
            <>
              <Spinner />
              Starting…
            </>
          ) : (
            '▶ Start Challenge'
          )}
        </button>
      </div>
    </div>
  )
}

function Spinner() {
  return (
    <svg
      width="14"
      height="14"
      viewBox="0 0 14 14"
      style={{ animation: 'spin 0.8s linear infinite' }}
    >
      <style>{`@keyframes spin { to { transform: rotate(360deg) } }`}</style>
      <circle
        cx="7" cy="7" r="5"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeDasharray="20"
        strokeDashoffset="10"
        strokeLinecap="round"
      />
    </svg>
  )
}