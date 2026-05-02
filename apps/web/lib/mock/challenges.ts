/**
 * MOCK DATA — Development only
 *
 * This file exists because the Go orchestrator and database are not yet
 * connected. Every value here matches the real type interfaces exactly.
 * Replacing mock data with real API calls requires only changing the
 * data source, not the component code.
 *
 * When removing mocks:
 * 1. Delete this file
 * 2. Update lib/api/challenges.ts to call real endpoints
 * 3. Components need zero changes
 */

import type { Challenge, ChallengeSummary, FileNode, FileContent } from '@/lib/types'

export const MOCK_CHALLENGES: ChallengeSummary[] = [
  {
    id: '01-silent-write',
    title: 'The Silent Write',
    difficulty: 'intro',
    category: 'API Debugging',
    stack: ['Node.js', 'Express'],
    estimatedMins: 15,
  },
]

export const MOCK_CHALLENGE_DETAIL: Challenge = {
  id: '01-silent-write',
  title: 'The Silent Write',
  difficulty: 'intro',
  category: 'API Debugging',
  estimatedMins: 15,
  stack: ['Node.js', 'Express'],
  symptom: 'Creating a user returns 201 Created, but after a server restart the user is missing.',
  objective:
    'Find the root cause and fix the code so that created users persist across server restarts. Verify your fix by creating a user, restarting the server, and confirming the user is still returned by GET /api/users.',
  environment: {
    runtime: 'node:18',
    start_command: 'node index.js',
    port: 3000,
    needs_database: false,
  },
  hints: [
    {
      order: 1,
      cost: 1,
      text: 'The user is successfully created during the current server session. What happens to in-memory data when a process restarts?',
    },
    {
      order: 2,
      cost: 2,
      text: 'There are two separate operations: reading users and writing users. Are they both targeting the same source of truth?',
    },
    {
      order: 3,
      cost: 3,
      text: 'Look carefully at the filenames involved in read vs write operations. Exact spelling matters.',
    },
  ],
  solution: {
    type: 'file_diff',
    description:
      'Read and write operations must target the same file. The discrepancy between source filenames causes writes to be lost on restart.',
    validation:
      'POST /users followed by server restart followed by GET /users must return the created user.',
  },
  testCases: [
    {
      id: 'tc1',
      description: 'POST /api/users creates a user and returns 201',
      request: {
        method: 'POST',
        path: '/api/users',
        body: { name: 'Test User' },
        headers: { 'X-API-Key': 'foo' },
      },
      expected: { status: 201 },
    },
    {
      id: 'tc2',
      description: 'GET /api/users returns the created user after restart',
      requires_restart: true,
      request: {
        method: 'GET',
        path: '/api/users',
        headers: { 'X-API-Key': 'foo' },
      },
      expected: { status: 200, contains_created_user: true },
    },
  ],
  author: 'stacktrace-team',
  license: 'MIT',
  source: 'Based on Express.js open source codebase',
}

export const MOCK_FILE_TREE: FileNode[] = [
  {
    name: 'index.js',
    path: '/app/index.js',
    type: 'file',
    language: 'javascript',
  },
  {
    name: 'user-store.js',
    path: '/app/user-store.js',
    type: 'file',
    language: 'javascript',
  },
  {
    name: 'users.json',
    path: '/app/users.json',
    type: 'file',
    language: 'json',
  },
  {
    name: 'package.json',
    path: '/app/package.json',
    type: 'file',
    language: 'json',
  },
]

export const MOCK_FILE_CONTENTS: FileContent[] = [
  {
    path: '/app/index.js',
    language: 'javascript',
    readonly: false,
    content: `'use strict'

var express = require('express')
var userStore = require('./user-store')

var app = module.exports = express()
app.use(express.json())

function error(status, msg) {
  var err = new Error(msg)
  err.status = status
  return err
}

app.use('/api', function(req, res, next) {
  var key = req.query['api-key']
  if (!key) return next(error(400, 'api key required'))
  if (key !== 'foo') return next(error(401, 'invalid api key'))
  next()
})

app.get('/api/users', function(req, res, next) {
  var users = userStore.all()
  res.json(users)
})

app.post('/api/users', function(req, res, next) {
  var body = req.body
  if (!body.name) return next(error(400, 'name required'))
  var user = userStore.create(body.name)
  res.status(201).json(user)
})

app.use(function(err, req, res, next) {
  res.status(err.status || 500)
  res.json({ error: err.message })
})

app.listen(3000, function() {
  console.log('Express started on port 3000')
})
`,
  },
  {
    path: '/app/user-store.js',
    language: 'javascript',
    readonly: false,
    content: `'use strict'

var fs = require('fs')
var path = require('path')

var readFile = path.join(__dirname, 'users.json')
var writeFile = path.join(__dirname, 'user.json')

var users = []

try {
  users = JSON.parse(fs.readFileSync(readFile, 'utf8'))
} catch (e) {
  users = []
}

exports.all = function() {
  return users
}

exports.create = function(name) {
  var user = { id: users.length + 1, name: name }
  users.push(user)
  fs.writeFileSync(writeFile, JSON.stringify(users, null, 2))
  return user
}
`,
  },
  {
    path: '/app/users.json',
    language: 'json',
    readonly: false,
    content: `[
  { "id": 1, "name": "Alice" },
  { "id": 2, "name": "Bob" }
]
`,
  },
  {
    path: '/app/package.json',
    language: 'json',
    readonly: true,
    content: `{
  "name": "web-service",
  "version": "1.0.0",
  "dependencies": {
    "express": "^5.2.1"
  }
}
`,
  },
]