import type { Metadata } from 'next';
import './globals.css';

export const metadata: Metadata = {
  title: 'Rice Search',
  description: 'Hybrid semantic + keyword code search platform',
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
