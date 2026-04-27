'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';

import { getCurrentUser, onAuthStateChange, type AuthUser } from '@/lib/auth';
import { Skeleton } from '@/components/ui/skeleton';

import { Sidebar } from './sidebar';
import { UserMenu } from './user-menu';

export default function AdminLayout({ children }: { children: ReactNode }) {
  const router = useRouter();
  const [user, setUser] = useState<AuthUser | null>(null);
  const [checking, setChecking] = useState(true);

  useEffect(() => {
    let cancelled = false;

    (async () => {
      const current = await getCurrentUser();
      if (cancelled) return;
      if (!current) {
        router.replace('/login');
        return;
      }
      setUser(current);
      setChecking(false);
    })();

    const sub = onAuthStateChange((_event, session) => {
      if (!session) {
        setUser(null);
        router.replace('/login');
      }
    });

    return () => {
      cancelled = true;
      sub.unsubscribe();
    };
  }, [router]);

  if (checking || !user) {
    return (
      <div className="flex min-h-screen flex-col gap-3 p-6">
        <Skeleton className="h-10 w-48" />
        <Skeleton className="h-4 w-full max-w-md" />
        <Skeleton className="h-4 w-full max-w-sm" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  return (
    <div className="flex min-h-screen flex-col md:flex-row">
      <Sidebar />
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-14 items-center justify-end gap-2 border-b bg-card px-4">
          <UserMenu user={user} />
        </header>
        <main className="flex-1 p-6">{children}</main>
      </div>
    </div>
  );
}
