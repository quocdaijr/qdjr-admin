'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';

import { apiPost, apiDelete, ApiError } from '@/lib/api';
import type { Post } from '@/lib/types';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

interface PostActionsProps {
  post: Post;
}

function describeError(err: unknown): string {
  if (err instanceof ApiError) return `${err.code}: ${err.message}`;
  if (err instanceof Error) return err.message;
  return 'Operation failed';
}

export function PostActions({ post }: PostActionsProps) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [confirmDelete, setConfirmDelete] = useState(false);

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['posts'] });
    queryClient.invalidateQueries({ queryKey: ['post', post.id] });
  };

  const publishMutation = useMutation({
    mutationFn: () => apiPost(`/v1/admin/posts/${post.id}/publish`, {}),
    onSuccess: () => {
      toast.success('Post published');
      invalidate();
    },
    onError: (err) => toast.error(describeError(err)),
  });

  const unpublishMutation = useMutation({
    mutationFn: () => apiPost(`/v1/admin/posts/${post.id}/unpublish`, {}),
    onSuccess: () => {
      toast.success('Post unpublished');
      invalidate();
    },
    onError: (err) => toast.error(describeError(err)),
  });

  const deleteMutation = useMutation({
    mutationFn: () => apiDelete(`/v1/admin/posts/${post.id}`),
    onSuccess: () => {
      toast.success('Post deleted');
      queryClient.invalidateQueries({ queryKey: ['posts'] });
      setConfirmDelete(false);
      router.push('/admin/posts');
    },
    onError: (err) => toast.error(describeError(err)),
  });

  return (
    <>
      {post.status === 'draft' ? (
        <Button
          type="button"
          variant="secondary"
          onClick={() => publishMutation.mutate()}
          disabled={publishMutation.isPending}
        >
          {publishMutation.isPending ? 'Publishing…' : 'Publish'}
        </Button>
      ) : null}
      {post.status === 'published' ? (
        <Button
          type="button"
          variant="secondary"
          onClick={() => unpublishMutation.mutate()}
          disabled={unpublishMutation.isPending}
        >
          {unpublishMutation.isPending ? 'Unpublishing…' : 'Unpublish'}
        </Button>
      ) : null}
      <Button
        type="button"
        variant="destructive"
        onClick={() => setConfirmDelete(true)}
      >
        Delete
      </Button>

      <Dialog
        open={confirmDelete}
        onOpenChange={(open) => !open && setConfirmDelete(false)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete post?</DialogTitle>
            <DialogDescription>
              This will permanently delete &quot;{post.title}&quot;. This action
              cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setConfirmDelete(false)}
              disabled={deleteMutation.isPending}
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={() => deleteMutation.mutate()}
              disabled={deleteMutation.isPending}
            >
              {deleteMutation.isPending ? 'Deleting…' : 'Delete'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
