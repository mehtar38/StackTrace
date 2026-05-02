/**
 * Challenge API client
 *
 * Fetches from Next.js API routes which read directly from challenge.json files.
 * File content and file tree still use mock data until the Go orchestrator is ready.
 */

import type { ApiResponse, Challenge, ChallengeSummary, FileContent, FileNode } from '@/lib/types'
import { MOCK_FILE_CONTENTS, MOCK_FILE_TREE } from '@/lib/mock/challenges'

const BASE = '/api'

export async function getChallenges(): Promise<ApiResponse<ChallengeSummary[]>> {
  try {
    const baseUrl = process.env.NEXT_PUBLIC_APP_URL ?? 'http://localhost:3000'
    const res = await fetch(`${baseUrl}/api/challenges`, { next: { revalidate: 60 } })
    return res.json()
  } catch (e) {
    console.log('FETCH ERROR:', e)
    return { data: null, error: { code: 'FETCH_ERROR', message: 'Failed to load challenges' } }
  }
}
export async function getChallengeById(id: string): Promise<ApiResponse<Challenge>> {
  try {
    const baseUrl = process.env.NEXT_PUBLIC_APP_URL ?? 'http://localhost:3000'
    const res = await fetch(`${baseUrl}/api/challenges/${id}`, { next: { revalidate: 60 } })
    return res.json()
  } catch (e) {
    console.log('FETCH ERROR:', e)
    return { data: null, error: { code: 'FETCH_ERROR', message: 'Failed to load challenge' } }
  }
}

// File system — mock until orchestrator is ready
export async function getChallengeFileTree(challengeId: string): Promise<ApiResponse<FileNode[]>> {
  void challengeId
  return { data: MOCK_FILE_TREE, error: null }
}

export async function getFileContent(
  challengeId: string,
  filePath: string,
): Promise<ApiResponse<FileContent>> {
  void challengeId
  const file = MOCK_FILE_CONTENTS.find(f => f.path === filePath)
  if (!file) {
    return { data: null, error: { code: 'NOT_FOUND', message: `File ${filePath} not found` } }
  }
  return { data: file, error: null }
}