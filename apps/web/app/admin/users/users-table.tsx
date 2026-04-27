'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { MoreHorizontal } from 'lucide-react';
import { toast } from 'sonner';

import { apiList, apiDelete, ApiError } from '@/lib/api';
import { useMe } from '@/lib/me';
import type { User } from '@/lib/types';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

import { InviteDialog } from './invite-dialog';
import { ChangeRoleDialog } from './change-role-dialog';

const PER_PAGE = 20;

export function UsersTable() {
  const meQuery = useMe();
  const queryClient = useQueryClient();
  const [page, setPage] = useState(1);
  const [inviteOpen, setInviteOpen] = useState(false);
  const [roleTarget, setRoleTarget] = useState<User | null>(null);
  const [pendingDelete, setPendingDelete] = useState<User | null>(null);

  const isSuperAdmin = meQuery.data?.role === 'super_admin';
  const enabled = isSuperAdmin;

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ['users', page, PER_PAGE],
    queryFn: () =>
      apiList<User>(`/v1/admin/users?page=${page}&perPage=${PER_PAGE}`),
    enabled,
  });

  const deleteMutation = useMutation({
    mutationFn: async (id: string) => apiDelete(`/v1/admin/users/${id}`),
    onSuccess: () => {
      toast.success('User deleted');
      setPendingDelete(null);
      queryClient.invalidateQueries({ queryKey: ['users'] });
    },
    onError: (err) => {
      const message =
        err instanceof ApiError
          ? err.code === 'LAST_SUPER_ADMIN'
            ? 'Cannot delete the last super admin'
            : `${err.code}: ${err.message}`
          : err instanceof Error
            ? err.message
            : 'Delete failed';
      toast.error(message);
    },
  });

  if (meQuery.isLoading) {
    return <Skeleton className="h-64 w-full" />;
  }

  if (!isSuperAdmin) {
    return (
      <div className="rounded-lg border bg-muted/40 p-6 text-sm text-muted-foreground">
        Access denied. User management requires the super_admin role.
      </div>
    );
  }

  const total = data?.meta.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PER_PAGE));
  const rows = data?.data ?? [];

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-end">
        <Button onClick={() => setInviteOpen(true)}>Invite user</Button>
      </div>

      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Email</TableHead>
              <TableHead>Role</TableHead>
              <TableHead>Created</TableHead>
              <TableHead>Last sign-in</TableHead>
              <TableHead className="w-[60px]" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              Array.from({ length: 5 }).map((_, i) => (
                <TableRow key={`skeleton-${i}`}>
                  <TableCell>
                    <Skeleton className="h-4 w-48" />
                  </TableCell>
                  <TableCell>
                    <Skeleton className="h-4 w-20" />
                  </TableCell>
                  <TableCell>
                    <Skeleton className="h-4 w-24" />
                  </TableCell>
                  <TableCell>
                    <Skeleton className="h-4 w-24" />
                  </TableCell>
                  <TableCell />
                </TableRow>
              ))
            ) : isError ? (
              <TableRow>
                <TableCell colSpan={5} className="text-center text-red-600">
                  {error instanceof Error
                    ? error.message
                    : 'Failed to load users'}
                </TableCell>
              </TableRow>
            ) : rows.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={5}
                  className="text-center text-muted-foreground"
                >
                  No users.
                </TableCell>
              </TableRow>
            ) : (
              rows.map((u) => (
                <TableRow key={u.id}>
                  <TableCell className="font-medium">{u.email}</TableCell>
                  <TableCell>
                    {u.role ? (
                      <Badge variant="secondary">{u.role}</Badge>
                    ) : (
                      <span className="text-muted-foreground">—</span>
                    )}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {new Date(u.created_at).toLocaleDateString()}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {u.last_sign_in_at
                      ? new Date(u.last_sign_in_at).toLocaleDateString()
                      : '—'}
                  </TableCell>
                  <TableCell>
                    <DropdownMenu>
                      <DropdownMenuTrigger
                        render={
                          <Button
                            variant="ghost"
                            size="icon-sm"
                            aria-label="Actions"
                          />
                        }
                      >
                        <MoreHorizontal className="size-4" />
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={() => setRoleTarget(u)}>
                          Change role
                        </DropdownMenuItem>
                        <DropdownMenuSeparator />
                        <DropdownMenuItem
                          variant="destructive"
                          onClick={() => setPendingDelete(u)}
                        >
                          Delete
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      <div className="flex items-center justify-between text-sm text-muted-foreground">
        <span>
          {total} user{total === 1 ? '' : 's'}
        </span>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={page <= 1 || isLoading}
            onClick={() => setPage((p) => Math.max(1, p - 1))}
          >
            Previous
          </Button>
          <span>
            Page {page} of {totalPages}
          </span>
          <Button
            variant="outline"
            size="sm"
            disabled={page >= totalPages || isLoading}
            onClick={() => setPage((p) => p + 1)}
          >
            Next
          </Button>
        </div>
      </div>

      <InviteDialog open={inviteOpen} onOpenChange={setInviteOpen} />
      <ChangeRoleDialog
        user={roleTarget}
        onOpenChange={(open) => !open && setRoleTarget(null)}
      />

      <Dialog
        open={pendingDelete !== null}
        onOpenChange={(open) => !open && setPendingDelete(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete user?</DialogTitle>
            <DialogDescription>
              This will permanently delete &quot;{pendingDelete?.email}&quot;.
              This action cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setPendingDelete(null)}
              disabled={deleteMutation.isPending}
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={() =>
                pendingDelete && deleteMutation.mutate(pendingDelete.id)
              }
              disabled={deleteMutation.isPending}
            >
              {deleteMutation.isPending ? 'Deleting…' : 'Delete'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
