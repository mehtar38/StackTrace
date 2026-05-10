// Clerk middleware protects routes and handles auth redirects.
// Public routes (home page, sign-in, sign-up, API routes) are listed explicitly.
// Everything else requires authentication.

import { clerkMiddleware, createRouteMatcher } from '@clerk/nextjs/server'

const isPublicRoute = createRouteMatcher([
  '/',                        // home / challenge list
  '/sign-in(.*)',             // Clerk hosted sign-in
  '/sign-up(.*)',             // Clerk hosted sign-up
  '/api/challenges(.*)',      // challenge metadata API (public)
])

export default clerkMiddleware(async (auth, req) => {
  // If the route is not public, require authentication.
  // Clerk will redirect to /sign-in automatically if the user is not signed in.
  if (!isPublicRoute(req)) {
    await auth.protect()
  }
})

export const config = {
  // Run middleware on all routes except Next.js internals and static files
  matcher: [
    '/((?!_next|[^?]*\\.(?:html?|css|js(?!on)|jpe?g|webp|png|gif|svg|ttf|woff2?|ico|csv|docx?|xlsx?|zip|webmanifest)).*)',
    '/(api|trpc)(.*)',
  ],
}