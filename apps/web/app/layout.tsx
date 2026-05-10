import type { Metadata } from 'next'
import { ClerkProvider } from '@clerk/nextjs'
// import "react-resizable-panels/styles.css"
import './globals.css'

export const metadata: Metadata = {
  title: 'StackTrace — Production Debugging Challenges',
  description: 'Debug real production codebases. Build the intuition no algorithm question can teach.',
}

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <ClerkProvider>
    <html lang="en" suppressHydrationWarning>
      <body>{children}</body>
    </html>
    </ClerkProvider>
  )
}