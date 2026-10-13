'use client';

import { useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { z } from 'zod';
import { toast } from 'sonner';

import type { Course } from '@/gen/lessons/v1/lessons_pb';
import { lessonsClient, rpcErrorMessage, setFieldErrors } from '@/lib/rpc';
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
  title: z.string().trim().min(1, 'Required'),
  url: z.string().trim().url('Must be a URL').or(z.literal('')),
  provider: z.string().trim().max(100),
  notes: z.string().max(10_000),
});

type FormValues = z.infer<typeof schema>;

interface CourseDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  course?: Course | null;
}

function defaultValues(c?: Course | null): FormValues {
  return {
    title: c?.title ?? '',
    url: c?.url ?? '',
    provider: c?.provider ?? '',
    notes: c?.notes ?? '',
  };
}

export function CourseDialog({ open, onOpenChange, course }: CourseDialogProps) {
  const queryClient = useQueryClient();
  const isEdit = Boolean(course);

  const {
    register,
    handleSubmit,
    reset,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({ resolver: zodResolver(schema), defaultValues: defaultValues(course) });

  useEffect(() => {
    if (open) reset(defaultValues(course));
  }, [open, course, reset]);

  // Server-side field errors (google.rpc.BadRequest) go under the input; anything else is a toast.
  const onRpcError = (fallback: string) => (err: unknown) => {
    if (!setFieldErrors(err, ['title', 'url', 'provider', 'notes'] as const, setError)) toast.error(rpcErrorMessage(err, fallback));
  };

  const done = () => {
    queryClient.invalidateQueries({ queryKey: ['courses'] });
    if (course) queryClient.invalidateQueries({ queryKey: ['course', course.id] });
    onOpenChange(false);
  };

  const createMutation = useMutation({
    mutationFn: (values: FormValues) => lessonsClient.createCourse(values),
    onSuccess: done,
    onError: onRpcError('Create failed'),
  });

  const updateMutation = useMutation({
    mutationFn: (values: FormValues) => lessonsClient.updateCourse({ id: course!.id, ...values }),
    onSuccess: done,
    onError: onRpcError('Save failed'),
  });

  const onSubmit = handleSubmit((values) => {
    if (isEdit) updateMutation.mutate(values);
    else createMutation.mutate(values);
  });

  const submitting = isSubmitting || createMutation.isPending || updateMutation.isPending;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{isEdit ? 'Edit course' : 'New course'}</DialogTitle>
        </DialogHeader>
        <form onSubmit={onSubmit} className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="course-title">Title</Label>
            <Input id="course-title" {...register('title')} placeholder="Course title" />
            {errors.title ? <p className="text-xs text-destructive">{errors.title.message}</p> : null}
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="course-url">URL</Label>
            <Input id="course-url" {...register('url')} placeholder="https://…" />
            {errors.url ? <p className="text-xs text-destructive">{errors.url.message}</p> : null}
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="course-provider">Provider</Label>
            <Input id="course-provider" {...register('provider')} placeholder="go.dev, a book, a platform" />
            {errors.provider ? <p className="text-xs text-destructive">{errors.provider.message}</p> : null}
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="course-notes">Notes</Label>
            <Textarea
              id="course-notes"
              {...register('notes')}
              rows={4}
              placeholder="Markdown supported"
              className="font-mono text-[13px] leading-6"
            />
            {errors.notes ? <p className="text-xs text-destructive">{errors.notes.message}</p> : null}
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={submitting}>
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
