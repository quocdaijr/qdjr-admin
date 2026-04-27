'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { LogOut } from 'lucide-react';
import { toast } from 'sonner';

import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';

import { signOut, type AuthUser } from '@/lib/auth';
import { useMe } from '@/lib/me';

interface UserMenuProps {
  user: AuthUser;
}

export function UserMenu({ user }: UserMenuProps) {
  const router = useRouter();
  const meQuery = useMe();
  const [signingOut, setSigningOut] = useState(false);

  const handleSignOut = async () => {
    setSigningOut(true);
    try {
      await signOut();
      router.replace('/login');
      router.refresh();
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Sign out failed';
      toast.error(message);
      setSigningOut(false);
    }
  };

  const me = meQuery.data;

  return (
    <div className="flex items-center gap-2">
      {meQuery.isLoading ? (
        <Skeleton className="h-5 w-16" />
      ) : me?.role ? (
        <Badge variant="secondary">{me.role}</Badge>
      ) : null}
      <DropdownMenu>
        <DropdownMenuTrigger
          render={<Button variant="outline" size="sm" />}
        >
          <span className="max-w-[180px] truncate">{user.email}</span>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuLabel>
            <div className="flex flex-col gap-0.5">
              <span className="text-sm font-medium">{user.email}</span>
              {me?.role ? (
                <span className="text-xs text-muted-foreground">{me.role}</span>
              ) : null}
            </div>
          </DropdownMenuLabel>
          <DropdownMenuSeparator />
          <DropdownMenuItem onClick={handleSignOut} disabled={signingOut}>
            <LogOut className="size-4" aria-hidden="true" />
            <span>{signingOut ? 'Signing out…' : 'Sign out'}</span>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
