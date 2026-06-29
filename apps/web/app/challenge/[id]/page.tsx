// app/challenge/[id]/page.tsx
// Server component. Loads challenge metadata and file tree only.
// No file CONTENTS loaded here — that happens client-side after session starts.
// No localStorage access here — that lives in useSession (client-side).

import { notFound, redirect } from 'next/navigation'
import { auth } from '@clerk/nextjs/server'
import { ChallengeIDE } from '@/components/challenge/ChallengeIDE'
import { getChallengeById, getChallengeFileTree } from '@/lib/api/challenges'

interface ChallengePageProps {
  params: { id: string }
}

export default async function ChallengePage({ params }: ChallengePageProps) {
  const { id } = params
  console.log('ChallengePage', id)

  // Require auth — redirect to sign-in if not authenticated
  const { userId } = await auth()
  if (!userId) {
    redirect(`/sign-in?redirect_url=/challenge/${id}`)
  }

  const [challengeRes, fileTreeRes] = await Promise.all([
    getChallengeById(id),
    getChallengeFileTree(id),
  ])

  if (challengeRes.error || !challengeRes.data) notFound()
  if (fileTreeRes.error || !fileTreeRes.data) notFound()

  // File CONTENTS are intentionally not loaded here.
  // ChallengeIDE will fetch them from the orchestrator after the session starts.
  return (
    <ChallengeIDE
      challenge={challengeRes.data}
      fileTree={fileTreeRes.data}
    />
  )
}
