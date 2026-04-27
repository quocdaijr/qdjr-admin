'use client';

import { useQuery } from '@tanstack/react-query';

import { apiGet, ApiError } from '@/lib/api';
import type { Post } from '@/lib/types';
import { Skeleton } from '@/components/ui/skeleton';

import { PostForm } from '../post-form';

export function EditPostClient({ id }: { id: string }) {
  const { data, isLoading, isError, error } = useQuery({
    queryKey: ['post', id],
    queryFn: () => apiGet<Post>(`/v1/admin/posts/${id}`),
  });

  if (isLoading) {
    return (
      <div className="flex flex-col gap-4">
        <Skeleton className="h-10 w-64" />
        <div className="grid grid-cols-1 gap-6 md:grid-cols-3">
          <div className="flex flex-col gap-4 md:col-span-2">
            <Skeleton className="h-8 w-full" />
            <Skeleton className="h-8 w-full" />
            <Skeleton className="h-20 w-full" />
            <Skeleton className="h-72 w-full" />
          </div>
          <div className="flex flex-col gap-4">
            <Skeleton className="h-8 w-full" />
            <Skeleton className="h-8 w-full" />
            <Skeleton className="h-8 w-full" />
            <Skeleton className="h-8 w-full" />
          </div>
        </div>
      </div>
    );
  }

  if (isError) {
    const message =
      error instanceof ApiError
        ? `${error.code}: ${error.message}`
        : error instanceof Error
          ? error.message
          : 'Failed to load post';
    return <p className="text-sm text-red-600">{message}</p>;
  }

  if (!data) return null;

  return <PostForm initial={data} />;
}
