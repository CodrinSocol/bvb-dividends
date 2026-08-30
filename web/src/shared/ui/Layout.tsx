import type { ReactNode } from 'react';
import { Link } from '@tanstack/react-router';

export function Layout({ children }: { children: ReactNode }) {
  return (
    <div className="min-h-screen bg-base-200">
      <header className="navbar bg-base-100 shadow-sm">
        <div className="mx-auto flex w-full max-w-6xl items-center gap-4 px-4">
          <Link to="/" className="text-lg font-semibold">
            BVB Dividends
          </Link>
          <span className="text-sm opacity-60">Bursa de Valori București</span>
          <a href="/api/docs" className="btn btn-ghost btn-sm ml-auto">
            API docs
          </a>
        </div>
      </header>
      <main className="mx-auto w-full max-w-6xl px-4 py-8">{children}</main>
    </div>
  );
}
