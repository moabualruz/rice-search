'use client';

import Link from 'next/link';
import Image from 'next/image';
import { usePathname } from 'next/navigation';

const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8088';

const externalLinks = [
  {
    name: 'API Docs',
    description: 'Swagger API Documentation',
    url: `${API_URL}/docs`,
    icon: '📚',
  },
  {
    name: 'Metrics',
    description: 'Prometheus Metrics Endpoint',
    url: `${API_URL}/metrics`,
    icon: '📊',
  },
  {
    name: 'Health',
    description: 'API Health Status',
    url: `${API_URL}/healthz`,
    icon: '💚',
  },
];

export function Navbar() {
  const pathname = usePathname();

  return (
    <nav className="navbar">
      <div className="nav-brand">
        <Link href="/" className="brand-link">
          <Image src="/logo.png" alt="Rice Search" width={28} height={28} className="brand-logo" />
          <span className="brand-text">Rice Search</span>
        </Link>
      </div>
      <div className="nav-tabs">
        <Link href="/" className={`nav-tab ${pathname === '/' ? 'active' : ''}`}>
          🔍 Search
        </Link>
        <Link href="/admin" className={`nav-tab ${pathname.startsWith('/admin') ? 'active' : ''}`}>
          ⚙️ Admin
        </Link>
      </div>
      <div className="nav-links">
        {externalLinks.map((link) => (
          <a
            key={link.name}
            href={link.url}
            target="_blank"
            rel="noopener noreferrer"
            className="nav-link"
            title={link.description}
          >
            <span className="nav-icon">{link.icon}</span>
            <span className="nav-label">{link.name}</span>
          </a>
        ))}
      </div>
    </nav>
  );
}
