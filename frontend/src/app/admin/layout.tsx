'use client';

import Link from 'next/link';
import Image from 'next/image';
import { usePathname } from 'next/navigation';

const navItems = [
  { href: '/admin', label: 'Dashboard', icon: '📊' },
  { href: '/admin/models', label: 'Models', icon: '🤖' },
  { href: '/admin/config', label: 'Config', icon: '⚙️' },
  { href: '/admin/users', label: 'Users', icon: '👥' },
  { href: '/admin/observability', label: 'Observability', icon: '📈' },
];

export default function AdminLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const pathname = usePathname();

  return (
    <div className="flex min-h-screen bg-slate-900">
      {/* Sidebar */}
      <aside className="w-64 bg-slate-800 border-r border-slate-700">
        <div className="p-6">
          <Link href="/" className="flex items-center gap-3">
            <Image 
              src="/logo.svg" 
              alt="rice ?earch" 
              width={40} 
              height={40}
              className="rounded-lg"
            />
            <div>
              <span className="text-xl font-bold text-white">rice ?earch</span>
              <p className="text-slate-400 text-sm">Admin Console</p>
            </div>
          </Link>
        </div>

        <nav className="px-4">
          {navItems.map((item) => {
            const isActive = pathname === item.href || 
              (item.href !== '/admin' && pathname.startsWith(item.href));
            
            return (
              <Link
                key={item.href}
                href={item.href}
                className={`flex items-center gap-3 px-4 py-3 rounded-lg mb-1 transition-colors ${
                  isActive
                    ? 'bg-primary/20 text-primary border border-primary/30'
                    : 'text-slate-300 hover:bg-slate-700/50'
                }`}
              >
                <span>{item.icon}</span>
                <span>{item.label}</span>
              </Link>
            );
          })}
        </nav>

        <div className="absolute bottom-4 left-4 right-4 mx-4">
          <Link
            href="/"
            className="flex items-center gap-2 px-4 py-2 text-slate-400 hover:text-white transition-colors"
          >
            ← Back to Search
          </Link>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 p-8">
        {children}
      </main>
    </div>
  );
}
