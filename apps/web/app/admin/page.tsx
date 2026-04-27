'use client';

import { useEffect, useState } from 'react';

import { getCurrentUser, type AuthUser } from '@/lib/auth';
import { apiGet, ApiError } from '@/lib/api';
import type { Me } from '@/lib/types';
import { Badge } from '@/components/ui/badge';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';

export default function AdminHomePage() {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [me, setMe] = useState<Me | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const [u, m] = await Promise.all([
          getCurrentUser(),
          apiGet<Me>('/v1/admin/me').catch((err) => {
            if (err instanceof ApiError) {
              throw new Error(`${err.code}: ${err.message}`);
            }
            throw err;
          }),
        ]);
        if (cancelled) return;
        setUser(u);
        setMe(m);
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Failed to load admin info');
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader>
          <CardTitle>Welcome{user?.email ? `, ${user.email}` : ''}</CardTitle>
          <CardDescription>
            This is your admin dashboard. Use the sidebar to manage content.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          {loading ? (
            <>
              <Skeleton className="h-4 w-40" />
              <Skeleton className="h-4 w-64" />
            </>
          ) : error ? (
            <p className="text-sm text-red-600">{error}</p>
          ) : me ? (
            <div className="flex flex-col gap-2">
              <div className="flex items-center gap-2">
                <span className="text-sm text-muted-foreground">Role:</span>
                <Badge variant="secondary">{me.role}</Badge>
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <span className="text-sm text-muted-foreground">Permissions:</span>
                {me.permissions.length === 0 ? (
                  <span className="text-sm text-muted-foreground">none</span>
                ) : (
                  me.permissions.map((p) => (
                    <Badge key={p} variant="outline">
                      {p}
                    </Badge>
                  ))
                )}
              </div>
            </div>
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}
