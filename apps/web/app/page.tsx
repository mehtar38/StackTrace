'use client'

// app/page.tsx
import Link from 'next/link'
import { useState } from 'react'
import { SignInButton, SignUpButton, UserButton, useAuth } from '@clerk/nextjs'

const CHALLENGES = [
  {
    id: '01-silent-write',
    title: 'The Silent Write',
    difficulty: 'intro',
    category: 'API Debugging',
    stack: ['Node.js', 'Express'],
    completed: false,
    attempted: false,
    estimatedMins: 15,
  },
]

const DIFFICULTY_META: Record<string, { label: string; color: string }> = {
  intro:  { label: 'Intro',  color: '#4ade80' },
  easy:   { label: 'Easy',   color: '#4ade80' },
  medium: { label: 'Medium', color: '#fbbf24' },
  hard:   { label: 'Hard',   color: '#f87171' },
}

const CATEGORIES = ['All', 'API Debugging', 'DevOps', 'RAG Pipeline', 'Data Pipeline', 'Error Rate']

export default function HomePage() {
  const [activeCategory, setActiveCategory] = useState('All')
  const [search, setSearch] = useState('')

  const filtered = CHALLENGES.filter(c => {
    const matchCat = activeCategory === 'All' || c.category === activeCategory
    const matchSearch = c.title.toLowerCase().includes(search.toLowerCase())
    return matchCat && matchSearch
  })

  return (
    <div style={{ minHeight: '100vh', background: 'var(--bg-base)' }}>
      <Nav />
      <main style={{ maxWidth: 900, margin: '0 auto', padding: '48px 24px' }}>
        <Header />
        <FilterBar
          categories={CATEGORIES}
          active={activeCategory}
          onCategory={setActiveCategory}
          search={search}
          onSearch={setSearch}
        />
        <ChallengeTable challenges={filtered} />
      </main>
    </div>
  )
}

function Nav() {
  const { isSignedIn } = useAuth()

  return (
    <nav style={{
      borderBottom: '1px solid var(--border)',
      padding: '0 24px',
      height: 52,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      position: 'sticky',
      top: 0,
      background: 'rgba(10,10,11,0.85)',
      backdropFilter: 'blur(12px)',
      zIndex: 100,
    }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <span style={{
          fontFamily: 'var(--font-mono)',
          fontWeight: 500,
          fontSize: 15,
          color: 'var(--text-primary)',
          letterSpacing: '-0.02em',
        }}>
          stack<span style={{ color: 'var(--accent)' }}>trace</span>
        </span>
      </div>

      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        {isSignedIn ? (
          // Clerk's built-in user button (avatar + dropdown with sign out)
          <UserButton />
        ) : (
          <>
            <SignInButton mode="modal">
              <button style={{
                background: 'transparent',
                border: '1px solid var(--border-strong)',
                color: 'var(--text-secondary)',
                padding: '6px 14px',
                borderRadius: 'var(--radius-md)',
                cursor: 'pointer',
                fontSize: 13,
                fontFamily: 'var(--font-sans)',
              }}>
                Sign in
              </button>
            </SignInButton>
            <SignUpButton mode="modal">
              <button style={{
                background: 'var(--accent-muted)',
                border: '1px solid var(--accent-border)',
                color: 'var(--accent)',
                padding: '6px 14px',
                borderRadius: 'var(--radius-md)',
                cursor: 'pointer',
                fontSize: 13,
                fontFamily: 'var(--font-sans)',
              }}>
                Sign up
              </button>
            </SignUpButton>
          </>
        )}
      </div>
    </nav>
  )
}

function Header() {
  return (
    <div style={{ marginBottom: 40 }}>
      <h1 style={{
        fontSize: 28,
        fontWeight: 500,
        color: 'var(--text-primary)',
        letterSpacing: '-0.03em',
        marginBottom: 8,
        fontFamily: 'var(--font-sans)',
      }}>
        Challenges
      </h1>
      <p style={{ color: 'var(--text-secondary)', fontSize: 14 }}>
        Real production codebases. Real bugs. No hints from Stack Overflow.
      </p>
    </div>
  )
}

function FilterBar({ categories, active, onCategory, search, onSearch }: {
  categories: string[]
  active: string
  onCategory: (c: string) => void
  search: string
  onSearch: (s: string) => void
}) {
  return (
    <div style={{ marginBottom: 24, display: 'flex', gap: 12, alignItems: 'center', flexWrap: 'wrap' }}>
      <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap' }}>
        {categories.map(cat => (
          <button
            key={cat}
            onClick={() => onCategory(cat)}
            style={{
              background: active === cat ? 'var(--accent-muted)' : 'transparent',
              border: `1px solid ${active === cat ? 'var(--accent-border)' : 'var(--border)'}`,
              color: active === cat ? 'var(--accent)' : 'var(--text-secondary)',
              padding: '5px 12px',
              borderRadius: 'var(--radius-md)',
              cursor: 'pointer',
              fontSize: 12,
              fontFamily: 'var(--font-sans)',
              transition: 'all 0.15s ease',
            }}
          >
            {cat}
          </button>
        ))}
      </div>
      <input
        value={search}
        onChange={e => onSearch(e.target.value)}
        placeholder="Search challenges..."
        style={{
          background: 'var(--bg-elevated)',
          border: '1px solid var(--border)',
          color: 'var(--text-primary)',
          padding: '5px 12px',
          borderRadius: 'var(--radius-md)',
          fontSize: 13,
          fontFamily: 'var(--font-sans)',
          outline: 'none',
          marginLeft: 'auto',
          width: 200,
        }}
      />
    </div>
  )
}

function ChallengeTable({ challenges }: { challenges: typeof CHALLENGES }) {
  if (challenges.length === 0) {
    return (
      <div style={{
        textAlign: 'center',
        padding: '80px 0',
        color: 'var(--text-muted)',
        fontSize: 13,
      }}>
        No challenges found.
      </div>
    )
  }

  return (
    <div style={{
      border: '1px solid var(--border)',
      borderRadius: 'var(--radius-lg)',
      overflow: 'hidden',
    }}>
      <table style={{ width: '100%', borderCollapse: 'collapse' }}>
        <thead>
          <tr style={{ borderBottom: '1px solid var(--border)' }}>
            {['Status', 'Title', 'Category', 'Stack', 'Difficulty', 'Est. time'].map(h => (
              <th key={h} style={{
                padding: '10px 16px',
                textAlign: 'left',
                fontSize: 11,
                fontWeight: 500,
                color: 'var(--text-muted)',
                letterSpacing: '0.06em',
                textTransform: 'uppercase',
                background: 'var(--bg-surface)',
                fontFamily: 'var(--font-sans)',
              }}>
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {challenges.map((c, i) => (
            <ChallengeRow key={c.id} challenge={c} isLast={i === challenges.length - 1} />
          ))}
        </tbody>
      </table>
    </div>
  )
}

function ChallengeRow({ challenge: c, isLast }: { challenge: typeof CHALLENGES[0]; isLast: boolean }) {
  const diff = DIFFICULTY_META[c.difficulty]

  return (
    <tr
      style={{
        borderBottom: isLast ? 'none' : '1px solid var(--border)',
        transition: 'background 0.1s ease',
        cursor: 'pointer',
      }}
      onMouseEnter={e => (e.currentTarget.style.background = 'var(--bg-elevated)')}
      onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
    >
      <td style={{ padding: '14px 16px', width: 60 }}>
        <StatusDot completed={c.completed} attempted={c.attempted} />
      </td>
      <td style={{ padding: '14px 16px' }}>
        <Link
          href={`/challenge/${c.id}`}
          style={{
            color: 'var(--text-primary)',
            textDecoration: 'none',
            fontSize: 14,
            fontWeight: 400,
            letterSpacing: '-0.01em',
          }}
          onMouseEnter={e => ((e.target as HTMLElement).style.color = 'var(--accent)')}
          onMouseLeave={e => ((e.target as HTMLElement).style.color = 'var(--text-primary)')}
        >
          {c.title}
        </Link>
      </td>
      <td style={{ padding: '14px 16px' }}>
        <span style={{ color: 'var(--text-secondary)', fontSize: 13 }}>{c.category}</span>
      </td>
      <td style={{ padding: '14px 16px' }}>
        <div style={{ display: 'flex', gap: 4 }}>
          {c.stack.map(s => (
            <span key={s} style={{
              background: 'var(--bg-active)',
              border: '1px solid var(--border)',
              color: 'var(--text-secondary)',
              padding: '2px 8px',
              borderRadius: 'var(--radius-sm)',
              fontSize: 11,
              fontFamily: 'var(--font-mono)',
            }}>
              {s}
            </span>
          ))}
        </div>
      </td>
      <td style={{ padding: '14px 16px' }}>
        <span style={{ color: diff.color, fontSize: 13 }}>{diff.label}</span>
      </td>
      <td style={{ padding: '14px 16px' }}>
        <span style={{ color: 'var(--text-muted)', fontSize: 13 }}>{c.estimatedMins}m</span>
      </td>
    </tr>
  )
}

function StatusDot({ completed, attempted }: { completed: boolean; attempted: boolean }) {
  const color = completed ? 'var(--success)' : attempted ? 'var(--warning)' : 'var(--text-muted)'
  return (
    <div style={{
      width: 8,
      height: 8,
      borderRadius: '50%',
      background: color,
      margin: '0 auto',
    }} />
  )
}