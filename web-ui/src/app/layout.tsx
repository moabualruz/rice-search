import type { Metadata } from 'next';
import { Navbar } from '@/components';
import './globals.css';

const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8088';

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
      <body>
        <div className="app">
          <Navbar />
          {children}
          <footer className="footer">
            <p>Rice Search Platform - Intelligent Hybrid Code Search</p>
            <p className="footer-links">
              <a href={`${API_URL}/docs`} target="_blank" rel="noopener noreferrer">
                API Docs
              </a>
              <span className="footer-sep">-</span>
              <a href={`${API_URL}/metrics`} target="_blank" rel="noopener noreferrer">
                Metrics
              </a>
              <span className="footer-sep">-</span>
              <a href="https://github.com" target="_blank" rel="noopener noreferrer">
                GitHub
              </a>
            </p>
          </footer>
        </div>
      </body>
    </html>
  );
}
