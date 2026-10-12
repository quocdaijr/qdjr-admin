'use client';

import { useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { z } from 'zod';
import { toast } from 'sonner';

import type { Lesson } from '@/gen/lessons/v1/lessons_pb';
import { lessonsClient, rpcErrorMessage } from '@/lib/rpc';
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
  title: z.string().trim().min(1, 'Required').max(200),
  url: z.string().trim().url('Must be a URL').or(z.literal('')),
  notes: z.string().max(10_000),
});

type FormValues = z.infer<typeof schema>;

interface LessonDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  courseId: string;
  lesson?: Lesson | null;
}

function defaultValues(l?: Lesson | null): FormValues {
  return { title: l?.title ?? '', url: l?.url ?? '', notes: l?.notes ?? '' };
}

export function LessonDialog({ open, onOpenChange, courseId, lesson }: LessonDialogProps) {
  const queryClient = useQueryClient();
  const isEdit = Boolean(lesson);

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({ resolver: zodResolver(schema), defaultValues: defaultValues(lesson) });

  useEffect(() => {
    if (open) reset(defaultValues(lesson));
  }, [open, lesson, reset]);

  const done = () => {
    queryClient.invalidateQueries({ queryKey: ['course', courseId] });
    queryClient.invalidateQueries({ queryKey: ['courses'] });
    onOpenChange(false);
  };

  const createMutation = useMutation({
    mutationFn: (values: FormValues) => lessonsClient.createLesson({ courseId, ...values }),
    onSuccess: done,
    onError: (err) => toast.error(rpcErrorMessage(err, 'Create failed')),
  });

  const updateMutation = useMutation({
    mutationFn: (values: FormValues) => lessonsClient.updateLesson({ id: lesson!.id, ...values }),
    onSuccess: done,
    onError: (err) => toast.error(rpcErrorMessage(err, 'Save failed')),
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
          <DialogTitle>{isEdit ? 'Edit lesson' : 'New lesson'}</DialogTitle>
        </DialogHeader>
        <form onSubmit={onSubmit} className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="lesson-title">Title</Label>
            <Input id="lesson-title" {...register('title')} placeholder="Lesson title" />
            {errors.title ? <p className="text-xs text-destructive">{errors.title.message}</p> : null}
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="lesson-url">URL</Label>
            <Input id="lesson-url" {...register('url')} placeholder="https://…" />
            {errors.url ? <p className="text-xs text-destructive">{errors.url.message}</p> : null}
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="lesson-notes">Notes</Label>
            <Textarea
              id="lesson-notes"
              {...register('notes')}
              rows={6}
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
              {submitting ? 'Saving…' : isEdit ? 'Save' : 'Add'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
