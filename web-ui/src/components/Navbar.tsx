'use client';

import { useState, useEffect } from 'react';
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
];

type ConnectionStatus = 'checking' | 'connected' | 'disconnected';

export function Navbar() {
  const pathname = usePathname();
  const [apiStatus, setApiStatus] = useState<ConnectionStatus>('checking');
  const [lastCheck, setLastCheck] = useState<string>('');

  useEffect(() => {
    const checkApiHealth = async () => {
      try {
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), 5000);
        
        const response = await fetch('/api/healthz', {
          signal: controller.signal,
        });
        clearTimeout(timeoutId);
        
        if (response.ok) {
          setApiStatus('connected');
        } else {
          setApiStatus('disconnected');
        }
      } catch {
        setApiStatus('disconnected');
      }
      setLastCheck(new Date().toLocaleTimeString());
    };

    // Check immediately
    checkApiHealth();
    
    // Re-check every 30 seconds
    const interval = setInterval(checkApiHealth, 30000);
    return () => clearInterval(interval);
  }, []);

  const statusConfig = {
    checking: { icon: '⏳', color: 'var(--color-text-muted)', label: 'Checking...' },
    connected: { icon: '●', color: 'var(--accent-green)', label: 'API Connected' },
    disconnected: { icon: '●', color: 'var(--accent-red)', label: 'API Disconnected' },
  };

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
        <Link href="/admin" className={`nav-tab ${pathname === '/admin' || pathname.startsWith('/admin/stores') ? 'active' : ''}`}>
          ⚙️ Admin
        </Link>
        <Link href="/admin/observability" className={`nav-tab ${pathname === '/admin/observability' ? 'active' : ''}`}>
          📊 Observability
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
      <div 
        className="api-status" 
        title={`${statusConfig[apiStatus].label}${lastCheck ? ` (checked ${lastCheck})` : ''}`}
      >
        <span 
          className="api-status-dot" 
          style={{ color: statusConfig[apiStatus].color }}
        >
          {statusConfig[apiStatus].icon}
        </span>
        <span className="api-status-label">{apiStatus === 'disconnected' ? 'API Offline' : 'API'}</span>
      </div>
    </nav>
  );
}
