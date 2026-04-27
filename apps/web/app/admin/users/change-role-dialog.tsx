'use client';

import { useEffect, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';

import { apiPatch, ApiError } from '@/lib/api';
import type { User } from '@/lib/types';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';

const ROLES = ['super_admin', 'editor', 'author'] as const;
type Role = (typeof ROLES)[number];

interface ChangeRoleDialogProps {
  user: User | null;
  onOpenChange: (open: boolean) => void;
}

export function ChangeRoleDialog({ user, onOpenChange }: ChangeRoleDialogProps) {
  const queryClient = useQueryClient();
  const [role, setRole] = useState<Role>('author');

  useEffect(() => {
    if (user) {
      setRole((user.role as Role | null) ?? 'author');
    }
  }, [user]);

  const updateMutation = useMutation({
    mutationFn: async () =>
      apiPatch<User>(`/v1/admin/users/${user!.id}/role`, { role }),
    onSuccess: () => {
      toast.success('Role updated');
      queryClient.invalidateQueries({ queryKey: ['users'] });
      onOpenChange(false);
    },
    onError: (err) => {
      const message =
        err instanceof ApiError
          ? err.code === 'LAST_SUPER_ADMIN'
            ? 'Cannot demote the last super admin'
            : `${err.code}: ${err.message}`
          : err instanceof Error
            ? err.message
            : 'Update failed';
      toast.error(message);
    },
  });

  const open = user !== null;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Change role</DialogTitle>
        </DialogHeader>
        <div className="flex flex-col gap-3">
          <p className="text-sm text-muted-foreground">{user?.email}</p>
          <div className="flex flex-col gap-1.5">
            <Label>Role</Label>
            <Select
              value={role}
              onValueChange={(v) => {
                if (typeof v === 'string') setRole(v as Role);
              }}
            >
              <SelectTrigger className="w-full">
                <SelectValue placeholder="Role" />
              </SelectTrigger>
              <SelectContent>
                {ROLES.map((r) => (
                  <SelectItem key={r} value={r}>
                    {r}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>
        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={updateMutation.isPending}
          >
            Cancel
          </Button>
          <Button
            type="button"
            onClick={() => updateMutation.mutate()}
            disabled={updateMutation.isPending}
          >
            {updateMutation.isPending ? 'Saving…' : 'Save'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
