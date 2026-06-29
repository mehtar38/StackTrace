import type { Challenge } from '@/lib/types'
import { Label } from '@/components/ui/label'
import { StackBadge } from '@/components/ui/stackbadge'

interface ProblemPanelProps {
  challenge: Challenge
}

export function ProblemPanel({ challenge }: ProblemPanelProps) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>

      <div>
        <div style={{ display: 'flex', gap: 6, marginBottom: 16, flexWrap: 'wrap' as const }}>
          {challenge.stack.map(s => <StackBadge key={s} name={s} />)}
        </div>
        <h2 style={{
          fontSize: 16,
          fontWeight: 500,
          color: 'var(--text-primary)',
          marginBottom: 0,
          letterSpacing: '-0.02em',
          fontFamily: 'var(--font-sans)',
        }}>
          {challenge.title}
        </h2>
      </div>

      <section>
        <Label>Symptom</Label>
        <div style={{
          background: 'var(--bg-elevated)',
          border: '1px solid var(--border)',
          borderLeft: '3px solid var(--danger)',
          borderRadius: 'var(--radius-md)',
          padding: '12px 14px',
          color: 'var(--text-primary)',
          fontSize: 13,
          lineHeight: 1.6,
          fontFamily: 'var(--font-mono)',
        }}>
          {challenge.symptom}
        </div>
      </section>

      <section>
        <Label>Objective</Label>
        <p style={{ color: 'var(--text-secondary)', fontSize: 13, lineHeight: 1.7 }}>
          {challenge.objective}
        </p>
      </section>

      <section>
        <Label>Environment</Label>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          {[
            ['Runtime',  challenge.environment.runtime],
            ['Port',     String(challenge.environment.port)],
            ['Database', challenge.environment.needs_database ? 'Yes' : 'None'],
          ].map(([key, value]) => (
            <div key={key} style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12 }}>
              <span style={{ color: 'var(--text-muted)' }}>{key}</span>
              <span style={{ color: 'var(--text-secondary)', fontFamily: 'var(--font-mono)' }}>
                {value}
              </span>
            </div>
          ))}
        </div>
      </section>

      <section>
        <Label>Quick commands</Label>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          {[
            `curl -X POST http://localhost:${challenge.environment.port}/api/users \\`,
            `curl "http://localhost:${challenge.environment.port}/api/users?api-key=foo"`,
          ].map((cmd, i) => (
            <code key={i} style={{
              display: 'block',
              background: 'var(--bg-elevated)',
              border: '1px solid var(--border)',
              borderRadius: 'var(--radius-sm)',
              padding: '8px 10px',
              fontSize: 11,
              fontFamily: 'var(--font-mono)',
              color: 'var(--text-secondary)',
              wordBreak: 'break-all' as const,
              lineHeight: 1.5,
            }}>
              {cmd}
            </code>
          ))}
        </div>
      </section>

    </div>
  )
}