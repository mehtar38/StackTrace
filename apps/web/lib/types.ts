// ─── Challenge Domain ──────────────────────────────────────────────────────────

export type Difficulty = 'intro' | 'easy' | 'medium' | 'hard'
export type Category =
  | 'API Debugging'
  | 'DevOps'
  | 'RAG Pipeline'
  | 'Data Pipeline'
  | 'Error Rate'

export interface ChallengeHint {
  order: number
  cost: number
  text: string
}

export interface TestCase {
  id: string
  description: string
  request: {
    method: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH'
    path: string
    body?: Record<string, unknown>
    headers?: Record<string, string>
  }
  expected: {
    status: number
    contains_created_user?: boolean
  }
  requires_restart?: boolean
}

export interface ChallengeEnvironment {
  runtime: string
  start_command: string
  port: number
  needs_database: boolean
}

export interface ChallengeSolution {
  type: 'file_diff' | 'config_change' | 'env_change'
  description: string
  validation: string
}

/** Full challenge object — matches challenge.json schema exactly */
export interface Challenge {
  id: string
  title: string
  difficulty: Difficulty
  category: Category
  estimatedMins: number
  stack: string[]
  symptom: string
  objective: string
  environment: ChallengeEnvironment
  hints: ChallengeHint[]
  solution: ChallengeSolution
  testCases: TestCase[]
  author: string
  license: string
  source: string
}

/** Lightweight version for the challenge list page */
export interface ChallengeSummary {
  id: string
  title: string
  difficulty: Difficulty
  category: Category
  stack: string[]
  estimatedMins: number
}

// ─── Session Domain ────────────────────────────────────────────────────────────

export type SessionStatus = 'pending' | 'active' | 'submitted' | 'expired'

export interface Session {
  id: string
  challengeId: string
  userId: string
  status: SessionStatus
  startedAt: string
  expiresAt: string
  hintsUsed: number[]
  containerEndpoint: string | null
  websocketUrl: string | null
}

// ─── File System Domain ────────────────────────────────────────────────────────

export type FileLanguage =
  | 'javascript'
  | 'typescript'
  | 'json'
  | 'yaml'
  | 'markdown'
  | 'plaintext'
  | 'python'
  | 'go'
  | 'sql'

export interface FileNode {
  name: string
  path: string
  type: 'file' | 'directory'
  language?: FileLanguage
  children?: FileNode[]
}

export interface FileContent {
  path: string
  content: string
  language: FileLanguage
  readonly: boolean
}

// ─── AI Assistant Domain ───────────────────────────────────────────────────────

export type MessageRole = 'user' | 'assistant'

export interface AIMessage {
  id: string
  role: MessageRole
  content: string
  timestamp: string
}

export interface AIConversation {
  sessionId: string
  messages: AIMessage[]
}

// ─── User Domain ───────────────────────────────────────────────────────────────

export interface User {
  id: string
  email: string
  name: string
  avatarUrl: string | null
  createdAt: string
}

export interface UserProgress {
  userId: string
  challengeId: string
  status: 'not_started' | 'attempted' | 'completed'
  attempts: number
  completedAt: string | null
  bestTimeSeconds: number | null
}

// ─── API Response Wrappers ─────────────────────────────────────────────────────

export interface ApiSuccess<T> {
  data: T
  error: null
}

export interface ApiError {
  data: null
  error: {
    code: string
    message: string
  }
}

export type ApiResponse<T> = ApiSuccess<T> | ApiError