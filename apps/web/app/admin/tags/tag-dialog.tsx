'use client';

import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { z } from 'zod';
import { toast } from 'sonner';

import { apiPost, apiPatch, ApiError } from '@/lib/api';
import type { Tag } from '@/lib/types';
import { slugify } from '@/lib/slugify';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

const schema = z.object({
  name: z.string().min(1, 'Required').max(200),
  slug: z
    .string()
    .regex(/^[a-z0-9]+(-[a-z0-9]+)*$/, 'lowercase, hyphenated')
    .max(200)
    .optional()
    .or(z.literal('')),
  description: z.string().max(1000).optional().or(z.literal('')),
});

type FormValues = z.infer<typeof schema>;

interface TagDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  tag?: Tag | null;
}

function defaultValues(t?: Tag | null): FormValues {
  return {
    name: t?.name ?? '',
    slug: t?.slug ?? '',
    description: t?.description ?? '',
  };
}

export function TagDialog({ open, onOpenChange, tag }: TagDialogProps) {
  const queryClient = useQueryClient();
  const isEdit = Boolean(tag);

  const {
    register,
    handleSubmit,
    reset,
    watch,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: defaultValues(tag),
  });

  useEffect(() => {
    if (open) {
      reset(defaultValues(tag));
      setSlugTouched(Boolean(tag?.slug));
    }
  }, [open, tag, reset]);

  const [slugTouched, setSlugTouched] = useState(Boolean(tag?.slug));

  const nameValue = watch('name');
  const slugValue = watch('slug');

  const handleNameBlur = () => {
    if (!slugTouched && (!slugValue || slugValue === '')) {
      const generated = slugify(nameValue ?? '');
      if (generated) setValue('slug', generated, { shouldValidate: true });
    }
  };

  const createMutation = useMutation({
    mutationFn: async (values: FormValues) =>
      apiPost<Tag>('/v1/admin/tags', toApiBody(values)),
    onSuccess: () => {
      toast.success('Tag created');
      queryClient.invalidateQueries({ queryKey: ['tags'] });
      onOpenChange(false);
    },
    onError: handleApiError,
  });

  const updateMutation = useMutation({
    mutationFn: async (values: FormValues) =>
      apiPatch<Tag>(`/v1/admin/tags/${tag!.id}`, toApiBody(values)),
    onSuccess: () => {
      toast.success('Tag saved');
      queryClient.invalidateQueries({ queryKey: ['tags'] });
      onOpenChange(false);
    },
    onError: handleApiError,
  });

  const onSubmit = handleSubmit((values) => {
    if (isEdit) updateMutation.mutate(values);
    else createMutation.mutate(values);
  });

  const submitting =
    isSubmitting || createMutation.isPending || updateMutation.isPending;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{isEdit ? 'Edit tag' : 'New tag'}</DialogTitle>
        </DialogHeader>
        <form onSubmit={onSubmit} className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <Label>Name</Label>
            <Input
              {...register('name', { onBlur: handleNameBlur })}
              placeholder="Tag name"
            />
            {errors.name ? (
              <p className="text-xs text-destructive">{errors.name.message}</p>
            ) : null}
          </div>
          <div className="flex flex-col gap-1.5">
            <Label>Slug</Label>
            <Input
              {...register('slug', {
                onChange: () => setSlugTouched(true),
              })}
              placeholder="my-tag"
            />
            {errors.slug ? (
              <p className="text-xs text-destructive">{errors.slug.message}</p>
            ) : (
              <p className="text-xs text-muted-foreground">
                lowercase, hyphenated
              </p>
            )}
          </div>
          <div className="flex flex-col gap-1.5">
            <Label>Description</Label>
            <Textarea
              {...register('description')}
              rows={3}
              placeholder="Optional description"
            />
            {errors.description ? (
              <p className="text-xs text-destructive">
                {errors.description.message}
              </p>
            ) : null}
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
              {submitting ? 'Saving…' : isEdit ? 'Save' : 'Create'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function toApiBody(values: FormValues) {
  return {
    name: values.name,
    slug: values.slug && values.slug !== '' ? values.slug : undefined,
    description:
      values.description && values.description !== ''
        ? values.description
        : null,
  };
}

function handleApiError(err: unknown) {
  const message =
    err instanceof ApiError
      ? `${err.code}: ${err.message}`
      : err instanceof Error
        ? err.message
        : 'Save failed';
  toast.error(message);
}
