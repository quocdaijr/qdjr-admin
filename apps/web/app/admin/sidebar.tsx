'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { Menu } from 'lucide-react';
import { useState } from 'react';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from '@/components/ui/sheet';

import { NAV_GROUPS, isActive } from './nav';

function Wordmark() {
  return (
    <Link href="/admin" className="font-heading text-[15px] font-semibold tracking-[-0.02em]">
      qdjr<span className="text-muted-foreground">/admin</span>
    </Link>
  );
}

function NavList({ pathname, onNavigate }: { pathname: string; onNavigate?: () => void }) {
  return (
    <nav aria-label="Admin" className="flex flex-col gap-6 px-3 py-4">
      {NAV_GROUPS.map((group) => (
        <div key={group.label} className="flex flex-col gap-0.5">
          <p className="px-3 pb-1 font-mono text-[11px] uppercase tracking-[0.06em] text-muted-foreground">
            {group.label}
          </p>
          {group.items.map((item) => {
            const active = isActive(pathname, item.href);
            const Icon = item.icon;
            return (
              <Link
                key={item.href}
                href={item.href}
                onClick={onNavigate}
                aria-current={active ? 'page' : undefined}
                className={cn(
                  'relative flex h-8 items-center gap-2.5 whitespace-nowrap rounded-md px-3 text-sm transition-colors',
                  active
                    ? 'font-medium text-foreground before:absolute before:inset-y-1.5 before:left-0 before:w-0.5 before:rounded-full before:bg-primary'
                    : 'text-muted-foreground hover:bg-background hover:text-foreground'
                )}
              >
                <Icon className="size-4 shrink-0" aria-hidden="true" />
                {item.label}
              </Link>
            );
          })}
        </div>
      ))}
    </nav>
  );
}

export function Sidebar() {
  const pathname = usePathname();
  const [mobileOpen, setMobileOpen] = useState(false);

  return (
    <>
      <aside className="hidden md:sticky md:top-0 md:flex md:h-screen md:w-60 md:shrink-0 md:flex-col md:border-r md:bg-sidebar">
        <div className="flex h-14 items-center border-b px-6">
          <Wordmark />
        </div>
        <NavList pathname={pathname} />
      </aside>

      <div className="flex h-14 items-center gap-2 border-b bg-sidebar px-3 md:hidden">
        <Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
          <SheetTrigger render={<Button variant="ghost" size="icon-sm" aria-label="Open navigation" />}>
            <Menu className="size-4" />
          </SheetTrigger>
          <SheetContent side="left" className="w-64 bg-sidebar p-0">
            <SheetHeader className="h-14 justify-center border-b px-6">
              <SheetTitle className="font-heading">qdjr/admin</SheetTitle>
            </SheetHeader>
            <NavList pathname={pathname} onNavigate={() => setMobileOpen(false)} />
          </SheetContent>
        </Sheet>
        <Wordmark />
      </div>
    </>
  );
}
