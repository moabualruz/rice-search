import type { Metadata } from 'next'
import localFont from 'next/font/local'
import './globals.css'
import Providers from './providers'

// Brand font: Terminus Nerd Font
const terminus = localFont({
  src: [
    {
      path: '../../public/fonts/TerminessNerdFont-Regular.ttf',
      weight: '400',
      style: 'normal',
    },
    {
      path: '../../public/fonts/TerminessNerdFont-Bold.ttf',
      weight: '700',
      style: 'normal',
    },
  ],
  variable: '--font-terminus',
  display: 'swap',
})

export const metadata: Metadata = {
  title: 'rice ?earch - Enterprise Intelligence',
  description: 'Local-first Neural Search Platform',
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en">
      <body className={terminus.className}>
        <Providers>{children}</Providers>
      </body>
    </html>
  )
}

