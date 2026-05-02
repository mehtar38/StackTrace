import { NextResponse } from 'next/server'
import fs from 'fs'
import path from 'path'
import type { ChallengeSummary } from '@/lib/types'

const CHALLENGES_DIR = path.join(process.cwd(), '..', '..', 'challenges')

export async function GET() {
  try {
    if (!fs.existsSync(CHALLENGES_DIR)) {
      return NextResponse.json({ data: [], error: null })
    }

    const dirs = fs.readdirSync(CHALLENGES_DIR).filter(entry => {
      const fullPath = path.join(CHALLENGES_DIR, entry)
      const jsonPath = path.join(fullPath, 'challenge.json')
      return fs.statSync(fullPath).isDirectory() && fs.existsSync(jsonPath)
    })

    const summaries: ChallengeSummary[] = dirs.map(dir => {
      const raw = fs.readFileSync(path.join(CHALLENGES_DIR, dir, 'challenge.json'), 'utf-8')
      const challenge = JSON.parse(raw)
      return {
        id: challenge.id,
        title: challenge.title,
        difficulty: challenge.difficulty,
        category: challenge.category,
        stack: challenge.stack,
        estimatedMins: challenge.estimated_minutes ?? challenge.estimatedMins,
      }
    })

    return NextResponse.json({ data: summaries, error: null })
  } catch {
    return NextResponse.json(
      { data: null, error: { code: 'READ_ERROR', message: 'Failed to list challenges' } },
      { status: 500 }
    )
  }
}