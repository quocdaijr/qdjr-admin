'use client';

import { useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation } from '@tanstack/react-query';
import { z } from 'zod';
import { toast } from 'sonner';

import { apiPatch, ApiError } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

const schema = z
  .object({
    current_password: z.string().min(1, 'Enter your current password'),
    new_password: z.string().min(10, 'At least 10 characters').max(72, 'At most 72 characters'),
    confirm: z.string(),
  })
  .refine((v) => v.new_password === v.confirm, { path: ['confirm'], message: 'Passwords do not match' })
  .refine((v) => v.new_password !== v.current_password, {
    path: ['new_password'],
    message: 'Choose a different password',
  });

type FormValues = z.infer<typeof schema>;

const EMPTY: FormValues = { current_password: '', new_password: '', confirm: '' };

export function ChangePasswordDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const {
    register,
    handleSubmit,
    reset,
    setError,
    formState: { errors },
  } = useForm<FormValues>({ resolver: zodResolver(schema), defaultValues: EMPTY });

  useEffect(() => {
    if (open) reset(EMPTY);
  }, [open, reset]);

  const mutation = useMutation({
    mutationFn: (v: FormValues) =>
      apiPatch<null>('/v1/admin/me/password', {
        current_password: v.current_password,
        new_password: v.new_password,
      }),
    onSuccess: () => {
      toast.success('Password changed');
      onOpenChange(false);
    },
    onError: (err) => {
      if (err instanceof ApiError && err.code === 'WRONG_PASSWORD') {
        setError('current_password', { message: 'Current password is incorrect' });
        return;
      }
      toast.error(err instanceof Error ? err.message : 'Could not change password');
    },
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Change password</DialogTitle>
          <DialogDescription>Use at least 10 characters.</DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit((v) => mutation.mutate(v))} className="flex flex-col gap-4" noValidate>
          <Field id="current-password" label="Current password" error={errors.current_password?.message}>
            <Input id="current-password" type="password" autoComplete="current-password" {...register('current_password')} />
          </Field>
          <Field id="new-password" label="New password" error={errors.new_password?.message}>
            <Input id="new-password" type="password" autoComplete="new-password" {...register('new_password')} />
          </Field>
          <Field id="confirm-password" label="Confirm new password" error={errors.confirm?.message}>
            <Input id="confirm-password" type="password" autoComplete="new-password" {...register('confirm')} />
          </Field>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={mutation.isPending}>
              Cancel
            </Button>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending ? 'Saving…' : 'Change password'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function Field({ id, label, error, children }: { id: string; label: string; error?: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={id}>{label}</Label>
      {children}
      <p className="min-h-4 text-xs text-destructive">{error ?? ''}</p>
    </div>
  );
}
