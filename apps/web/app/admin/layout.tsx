'use client';

import { useCallback, useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { Plus, Search } from 'lucide-react';

import { getCurrentUser, onAuthStateChange, type AuthUser } from '@/lib/auth';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';

import { CommandPalette, useCommandPaletteHotkey } from './command-palette';
import { Sidebar } from './sidebar';
import { UserMenu } from './user-menu';

export default function AdminLayout({ children }: { children: ReactNode }) {
  const router = useRouter();
  const [user, setUser] = useState<AuthUser | null>(null);
  const [checking, setChecking] = useState(true);
  const [paletteOpen, setPaletteOpen] = useState(false);
  const openPalette = useCallback(() => setPaletteOpen(true), []);
  useCommandPaletteHotkey(openPalette);

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
      <div className="mx-auto flex min-h-screen w-full max-w-[1200px] flex-col gap-3 p-8">
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
        <header className="sticky top-0 z-30 flex h-14 items-center gap-3 border-b bg-background/90 px-4 backdrop-blur-sm md:px-8">
          <button
            type="button"
            onClick={openPalette}
            className="flex h-8 min-w-0 max-w-sm flex-1 items-center gap-2 rounded-md border bg-card px-2.5 text-sm text-muted-foreground transition-colors hover:border-primary/50"
          >
            <Search className="size-4 shrink-0" aria-hidden="true" />
            <span className="truncate">Search or jump to…</span>
            <kbd className="ml-auto hidden rounded border px-1.5 font-mono text-[11px] sm:inline">⌘K</kbd>
          </button>
          <div className="ml-auto flex shrink-0 items-center gap-2">
            <Button size="sm" nativeButton={false} render={<Link href="/admin/posts/new" />} aria-label="New post">
              <Plus className="size-4" aria-hidden="true" />
              <span className="hidden sm:inline">New post</span>
            </Button>
            <UserMenu user={user} />
          </div>
        </header>
        <main className="mx-auto flex w-full max-w-[1200px] flex-1 flex-col gap-6 px-4 py-6 md:px-8 md:py-8">
          {children}
        </main>
      </div>
      <CommandPalette open={paletteOpen} onOpenChange={setPaletteOpen} />
    </div>
  );
}
