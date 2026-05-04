import { notFound } from 'next/navigation'
import { ChallengeIDE } from '@/components/challenge/ChallengeIDE'
import { getChallengeById, getChallengeFileTree, getFileContent } from '@/lib/api/challenges'

interface ChallengePageProps {
  params: Promise<{ id: string }>
}

export default async function ChallengePage({ params }: ChallengePageProps) {
  const { id } = await params
  // console.log('PAGE HIT - id:', id)

  const [challengeRes, fileTreeRes] = await Promise.all([
    getChallengeById(id),
    getChallengeFileTree(id),
  ])

  //  console.log('challengeRes:', JSON.stringify(challengeRes))
  // console.log('fileTreeRes:', JSON.stringify(fileTreeRes))

    if (challengeRes.error || !challengeRes.data) {
    // console.log('NOTFOUND: challenge error', challengeRes.error)
    notFound()
  }

  if (challengeRes.error || !challengeRes.data) notFound()
  if (fileTreeRes.error || !fileTreeRes.data) notFound()

  const challenge = challengeRes.data
  const fileTree = fileTreeRes.data

  const fileContentsRes = await Promise.all(
    fileTree
      .filter(f => f.type === 'file')
      .map(f => getFileContent(id, f.path))
  )

  const fileContents = fileContentsRes
    .filter(r => r.data !== null)
    .map(r => r.data!)

  return (
    <ChallengeIDE
      challenge={challenge}
      fileTree={fileTree}
      fileContents={fileContents}
    />
  )
}