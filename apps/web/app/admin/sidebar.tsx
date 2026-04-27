'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import {
  Newspaper,
  Folder,
  Tags,
  Image as ImageIcon,
  User as UserIcon,
  Settings,
  Inbox,
  Users,
  Menu,
} from 'lucide-react';
import type { ComponentType, SVGProps } from 'react';
import { useState } from 'react';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet';

interface NavItem {
  href: string;
  label: string;
  icon: ComponentType<SVGProps<SVGSVGElement>>;
}

const NAV: NavItem[] = [
  { href: '/admin/posts', label: 'Posts', icon: Newspaper },
  { href: '/admin/categories', label: 'Categories', icon: Folder },
  { href: '/admin/tags', label: 'Tags', icon: Tags },
  { href: '/admin/media', label: 'Media', icon: ImageIcon },
  { href: '/admin/profile', label: 'Profile', icon: UserIcon },
  { href: '/admin/settings', label: 'Settings', icon: Settings },
  { href: '/admin/inbox', label: 'Inbox', icon: Inbox },
  { href: '/admin/users', label: 'Users', icon: Users },
];

interface NavListProps {
  pathname: string;
  onNavigate?: () => void;
}

function NavList({ pathname, onNavigate }: NavListProps) {
  return (
    <nav className="flex flex-col gap-1 p-2">
      {NAV.map((item) => {
        const isActive =
          pathname === item.href || pathname.startsWith(item.href + '/');
        const Icon = item.icon;
        return (
          <Link
            key={item.href}
            href={item.href}
            onClick={onNavigate}
            className={cn(
              'flex items-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition-colors',
              isActive
                ? 'bg-muted text-foreground'
                : 'text-muted-foreground hover:bg-muted hover:text-foreground'
            )}
          >
            <Icon className="size-4" aria-hidden="true" />
            <span>{item.label}</span>
          </Link>
        );
      })}
    </nav>
  );
}

export function Sidebar() {
  const pathname = usePathname();
  const [mobileOpen, setMobileOpen] = useState(false);

  return (
    <>
      {/* Desktop sidebar */}
      <aside className="hidden md:flex md:w-60 md:flex-col md:border-r md:bg-card">
        <div className="flex h-14 items-center px-4 font-semibold">qdjr admin</div>
        <NavList pathname={pathname} />
      </aside>

      {/* Mobile sheet trigger */}
      <div className="flex h-14 items-center gap-2 border-b bg-card px-3 md:hidden">
        <Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
          <SheetTrigger
            render={
              <Button variant="ghost" size="icon-sm" aria-label="Open navigation" />
            }
          >
            <Menu className="size-4" />
          </SheetTrigger>
          <SheetContent side="left" className="p-0">
            <SheetHeader>
              <SheetTitle>qdjr admin</SheetTitle>
            </SheetHeader>
            <NavList pathname={pathname} onNavigate={() => setMobileOpen(false)} />
          </SheetContent>
        </Sheet>
        <span className="font-semibold">qdjr admin</span>
      </div>
    </>
  );
}
