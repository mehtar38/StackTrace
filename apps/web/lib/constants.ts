import type { Difficulty } from '@/lib/types'

export const DIFFICULTY_META: Record<Difficulty, { label: string; color: string }> = {
  intro:  { label: 'Intro',  color: 'var(--success)' },
  easy:   { label: 'Easy',   color: 'var(--success)' },
  medium: { label: 'Medium', color: 'var(--warning)' },
  hard:   { label: 'Hard',   color: 'var(--danger)'  },
}

export const CATEGORIES = [
  'All',
  'API Debugging',
  'DevOps',
  'RAG Pipeline',
  'Data Pipeline',
  'Error Rate',
] as const