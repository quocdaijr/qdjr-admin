'use client';

import { useEffect } from 'react';
import { useForm, Controller } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { z } from 'zod';
import { toast } from 'sonner';

import { apiPost, ApiError } from '@/lib/api';
import type { User } from '@/lib/types';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
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

const schema = z.object({
  email: z.string().email('Invalid email'),
  role: z.enum(ROLES),
  password: z.string().min(8, 'At least 8 characters').optional().or(z.literal('')),
});

type FormValues = z.infer<typeof schema>;

interface InviteDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function InviteDialog({ open, onOpenChange }: InviteDialogProps) {
  const queryClient = useQueryClient();

  const {
    register,
    handleSubmit,
    control,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { email: '', role: 'author', password: '' },
  });

  useEffect(() => {
    if (open) {
      reset({ email: '', role: 'author', password: '' });
    }
  }, [open, reset]);

  const createMutation = useMutation({
    mutationFn: async (values: FormValues) =>
      apiPost<User>('/v1/admin/users', {
        email: values.email,
        role: values.role,
        password:
          values.password && values.password !== '' ? values.password : undefined,
      }),
    onSuccess: () => {
      toast.success('User invited');
      queryClient.invalidateQueries({ queryKey: ['users'] });
      onOpenChange(false);
    },
    onError: (err) => {
      const message =
        err instanceof ApiError
          ? `${err.code}: ${err.message}`
          : err instanceof Error
            ? err.message
            : 'Invite failed';
      toast.error(message);
    },
  });

  const onSubmit = handleSubmit((values) => createMutation.mutate(values));
  const submitting = isSubmitting || createMutation.isPending;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Invite user</DialogTitle>
        </DialogHeader>
        <form onSubmit={onSubmit} className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <Label>Email</Label>
            <Input
              {...register('email')}
              type="email"
              placeholder="user@example.com"
            />
            {errors.email ? (
              <p className="text-xs text-destructive">{errors.email.message}</p>
            ) : null}
          </div>
          <div className="flex flex-col gap-1.5">
            <Label>Role</Label>
            <Controller
              control={control}
              name="role"
              render={({ field }) => (
                <Select
                  value={field.value}
                  onValueChange={(v) => {
                    if (typeof v === 'string') field.onChange(v);
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
              )}
            />
            {errors.role ? (
              <p className="text-xs text-destructive">{errors.role.message}</p>
            ) : null}
          </div>
          <div className="flex flex-col gap-1.5">
            <Label>Password (optional)</Label>
            <Input
              {...register('password')}
              type="password"
              placeholder="Leave blank to send invite email"
            />
            {errors.password ? (
              <p className="text-xs text-destructive">
                {errors.password.message}
              </p>
            ) : (
              <p className="text-xs text-muted-foreground">
                Required only when creating a new user without an invite flow.
              </p>
            )}
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={submitting}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={submitting}>
              {submitting ? 'Inviting…' : 'Invite'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
