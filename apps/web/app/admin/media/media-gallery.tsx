'use client';

import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Trash2, Copy, Check } from 'lucide-react';
import { toast } from 'sonner';

import { apiList, apiDelete, ApiError } from '@/lib/api';
import type { Media } from '@/lib/types';
import { formatBytes } from '@/lib/format';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';

const PER_PAGE = 24;

export function MediaGallery() {
  const queryClient = useQueryClient();
  const [page, setPage] = useState(1);
  const [pendingDelete, setPendingDelete] = useState<Media | null>(null);
  const [detailItem, setDetailItem] = useState<Media | null>(null);

  const queryKey = ['media', page, PER_PAGE] as const;
  const { data, isLoading, isError, error } = useQuery({
    queryKey,
    queryFn: async () => {
      const params = new URLSearchParams();
      params.set('page', String(page));
      params.set('perPage', String(PER_PAGE));
      return apiList<Media>(`/v1/admin/media?${params.toString()}`);
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async (id: string) => apiDelete(`/v1/admin/media/${id}`),
    onSuccess: () => {
      toast.success('Media deleted');
      setPendingDelete(null);
      queryClient.invalidateQueries({ queryKey: ['media'] });
    },
    onError: (err) => {
      const message =
        err instanceof ApiError
          ? err.code === 'IN_USE'
            ? 'Media is referenced by a post and cannot be deleted'
            : `${err.code}: ${err.message}`
          : err instanceof Error
            ? err.message
            : 'Delete failed';
      toast.error(message);
    },
  });

  const total = data?.meta.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PER_PAGE));
  const items = data?.data ?? [];

  return (
    <div className="flex flex-col gap-3">
      {isLoading ? (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
          {Array.from({ length: 12 }).map((_, i) => (
            <Skeleton key={i} className="aspect-square w-full" />
          ))}
        </div>
      ) : isError ? (
        <p className="text-center text-red-600">
          {error instanceof Error ? error.message : 'Failed to load media'}
        </p>
      ) : items.length === 0 ? (
        <p className="text-center text-muted-foreground">No media yet.</p>
      ) : (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
          {items.map((item) => (
            <MediaCard
              key={item.id}
              item={item}
              onClick={() => setDetailItem(item)}
              onDelete={() => setPendingDelete(item)}
            />
          ))}
        </div>
      )}

      <div className="flex items-center justify-between text-sm text-muted-foreground">
        <span>
          {total} item{total === 1 ? '' : 's'}
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

      <Dialog
        open={pendingDelete !== null}
        onOpenChange={(open) => !open && setPendingDelete(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete media?</DialogTitle>
            <DialogDescription>
              This will permanently delete &quot;{pendingDelete?.filename}
              &quot;. This action cannot be undone.
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

      <Sheet
        open={detailItem !== null}
        onOpenChange={(open) => !open && setDetailItem(null)}
      >
        <SheetContent side="right" className="overflow-y-auto">
          <SheetHeader>
            <SheetTitle>{detailItem?.filename}</SheetTitle>
            <SheetDescription>Media details</SheetDescription>
          </SheetHeader>
          {detailItem ? <MediaDetails item={detailItem} /> : null}
        </SheetContent>
      </Sheet>
    </div>
  );
}

function MediaCard({
  item,
  onClick,
  onDelete,
}: {
  item: Media;
  onClick: () => void;
  onDelete: () => void;
}) {
  const isImage = item.mime_type.startsWith('image/');
  return (
    <div className="group relative cursor-pointer overflow-hidden rounded-md border bg-muted">
      <button
        type="button"
        onClick={onClick}
        className="block w-full text-left"
      >
        <div className="aspect-square w-full">
          {isImage ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img
              src={item.url}
              alt={item.alt_text ?? item.filename}
              className="h-full w-full object-cover"
            />
          ) : (
            <div className="flex h-full w-full items-center justify-center text-xs text-muted-foreground">
              {item.mime_type}
            </div>
          )}
        </div>
        <div className="border-t bg-background p-2">
          <p className="truncate text-xs font-medium" title={item.filename}>
            {item.filename}
          </p>
          <p className="text-xs text-muted-foreground">
            {formatBytes(item.size)}
          </p>
        </div>
      </button>
      <div className="pointer-events-none absolute inset-0 flex items-start justify-end bg-black/0 p-2 opacity-0 transition-all group-hover:bg-black/20 group-hover:opacity-100">
        <Button
          type="button"
          variant="destructive"
          size="icon-sm"
          className="pointer-events-auto"
          aria-label="Delete"
          onClick={(e) => {
            e.stopPropagation();
            onDelete();
          }}
        >
          <Trash2 className="size-4" />
        </Button>
      </div>
    </div>
  );
}

function MediaDetails({ item }: { item: Media }) {
  const [copied, setCopied] = useState(false);
  const isImage = item.mime_type.startsWith('image/');

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(item.url);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      toast.error('Copy failed');
    }
  };

  return (
    <div className="flex flex-col gap-3 px-4 pb-4 text-sm">
      {isImage ? (
        // eslint-disable-next-line @next/next/no-img-element
        <img
          src={item.url}
          alt={item.alt_text ?? item.filename}
          className="w-full rounded-md border"
        />
      ) : null}
      <DetailRow label="Filename" value={item.filename} />
      <DetailRow label="MIME" value={item.mime_type} />
      <DetailRow label="Size" value={formatBytes(item.size)} />
      {item.width && item.height ? (
        <DetailRow label="Dimensions" value={`${item.width} × ${item.height}`} />
      ) : null}
      {item.alt_text ? (
        <DetailRow label="Alt text" value={item.alt_text} />
      ) : null}
      <div className="flex flex-col gap-1">
        <span className="text-xs font-medium text-muted-foreground">URL</span>
        <div className="flex items-start gap-2">
          <code className="flex-1 break-all rounded bg-muted px-2 py-1 text-xs">
            {item.url}
          </code>
          <Button
            type="button"
            variant="outline"
            size="icon-sm"
            onClick={copy}
            aria-label="Copy URL"
          >
            {copied ? (
              <Check className="size-4" />
            ) : (
              <Copy className="size-4" />
            )}
          </Button>
        </div>
      </div>
    </div>
  );
}

function DetailRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-xs font-medium text-muted-foreground">{label}</span>
      <span className="break-words">{value}</span>
    </div>
  );
}
