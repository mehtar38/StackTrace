import { NextResponse } from 'next/server'
import fs from 'fs'
import path from 'path'
import type { Challenge } from '@/lib/types'

// Resolves to /challenges directory at monorepo root
const CHALLENGES_DIR = path.join(process.cwd(), '..', '..', 'challenges')

export async function GET(
  _req: Request,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params
console.log('DIRS:', fs.readdirSync(CHALLENGES_DIR))
const challengePath = path.join(CHALLENGES_DIR, id, 'challenge.json')
console.log('LOOKING FOR:', challengePath)
console.log('FILE EXISTS:', fs.existsSync(challengePath))
  try {
    const challengePath = path.join(CHALLENGES_DIR, id, 'challenge.json')

    if (!fs.existsSync(challengePath)) {
      return NextResponse.json(
        { data: null, error: { code: 'NOT_FOUND', message: `Challenge ${id} not found` } },
        { status: 404 }
      )
    }

    const raw = fs.readFileSync(challengePath, 'utf-8')
    const challenge: Challenge = JSON.parse(raw)

    return NextResponse.json({ data: challenge, error: null })
  } catch {
    return NextResponse.json(
      { data: null, error: { code: 'PARSE_ERROR', message: 'Failed to read challenge' } },
      { status: 500 }
    )
  }
}