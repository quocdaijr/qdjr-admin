'use client';

import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { z } from 'zod';
import { toast } from 'sonner';

import { apiPost, apiPatch, ApiError } from '@/lib/api';
import type { Category } from '@/lib/types';
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

interface CategoryDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  category?: Category | null;
}

function defaultValues(c?: Category | null): FormValues {
  return {
    name: c?.name ?? '',
    slug: c?.slug ?? '',
    description: c?.description ?? '',
  };
}

export function CategoryDialog({
  open,
  onOpenChange,
  category,
}: CategoryDialogProps) {
  const queryClient = useQueryClient();
  const isEdit = Boolean(category);

  const {
    register,
    handleSubmit,
    reset,
    watch,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: defaultValues(category),
  });

  useEffect(() => {
    if (open) {
      reset(defaultValues(category));
      setSlugTouched(Boolean(category?.slug));
    }
  }, [open, category, reset]);

  const [slugTouched, setSlugTouched] = useState(Boolean(category?.slug));

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
      apiPost<Category>('/v1/admin/categories', toApiBody(values)),
    onSuccess: () => {
      toast.success('Category created');
      queryClient.invalidateQueries({ queryKey: ['categories'] });
      onOpenChange(false);
    },
    onError: handleApiError,
  });

  const updateMutation = useMutation({
    mutationFn: async (values: FormValues) =>
      apiPatch<Category>(
        `/v1/admin/categories/${category!.id}`,
        toApiBody(values)
      ),
    onSuccess: () => {
      toast.success('Category saved');
      queryClient.invalidateQueries({ queryKey: ['categories'] });
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
          <DialogTitle>{isEdit ? 'Edit category' : 'New category'}</DialogTitle>
        </DialogHeader>
        <form onSubmit={onSubmit} className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <Label>Name</Label>
            <Input
              {...register('name', { onBlur: handleNameBlur })}
              placeholder="Category name"
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
              placeholder="my-category"
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
