'use client';

import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ImageIcon, X } from 'lucide-react';

import { apiGet, apiList } from '@/lib/api';
import type { Media } from '@/lib/types';
import { formatBytes } from '@/lib/format';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

const PER_PAGE = 24;

interface MediaPickerProps {
  value?: string | null;
  onChange: (id: string | null) => void;
}

export function MediaPicker({ value, onChange }: MediaPickerProps) {
  const [open, setOpen] = useState(false);

  const previewQuery = useQuery({
    queryKey: ['media', 'preview', value],
    queryFn: () => apiGet<Media>(`/v1/admin/media/${value}`),
    enabled: Boolean(value),
  });

  const preview = previewQuery.data;

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-2">
        <Button type="button" variant="outline" onClick={() => setOpen(true)}>
          <ImageIcon className="size-4" />
          {value ? 'Change image' : 'Pick image'}
        </Button>
        {value ? (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => onChange(null)}
          >
            <X className="size-4" />
            Clear
          </Button>
        ) : null}
      </div>
      {value ? (
        previewQuery.isLoading ? (
          <Skeleton className="aspect-video w-full max-w-xs" />
        ) : preview ? (
          <div className="overflow-hidden rounded-md border max-w-xs">
            {preview.mime_type.startsWith('image/') ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img
                src={preview.url}
                alt={preview.alt_text ?? preview.filename}
                className="aspect-video w-full object-cover"
              />
            ) : (
              <div className="flex aspect-video items-center justify-center bg-muted text-xs text-muted-foreground">
                {preview.mime_type}
              </div>
            )}
            <div className="p-2 text-xs">
              <p className="truncate font-medium">{preview.filename}</p>
              <p className="text-muted-foreground">
                {formatBytes(preview.size)}
              </p>
            </div>
          </div>
        ) : (
          <p className="text-xs text-muted-foreground">
            Selected media not found.
          </p>
        )
      ) : null}

      <MediaPickerDialog
        open={open}
        onOpenChange={setOpen}
        currentId={value ?? null}
        onSelect={(id) => {
          onChange(id);
          setOpen(false);
        }}
      />
    </div>
  );
}

interface MediaPickerDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentId: string | null;
  onSelect: (id: string) => void;
}

function MediaPickerDialog({
  open,
  onOpenChange,
  currentId,
  onSelect,
}: MediaPickerDialogProps) {
  const [page, setPage] = useState(1);
  const [selected, setSelected] = useState<string | null>(currentId);

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ['media', 'picker', page, PER_PAGE],
    queryFn: () =>
      apiList<Media>(`/v1/admin/media?page=${page}&perPage=${PER_PAGE}`),
    enabled: open,
  });

  const total = data?.meta.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PER_PAGE));
  const items = data?.data ?? [];

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle>Select media</DialogTitle>
        </DialogHeader>
        {isLoading ? (
          <div className="grid grid-cols-3 gap-3 sm:grid-cols-4 md:grid-cols-5">
            {Array.from({ length: 10 }).map((_, i) => (
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
          <div className="grid max-h-[60vh] grid-cols-3 gap-3 overflow-y-auto sm:grid-cols-4 md:grid-cols-5">
            {items.map((item) => {
              const isSelected = selected === item.id;
              const isImage = item.mime_type.startsWith('image/');
              return (
                <button
                  key={item.id}
                  type="button"
                  onClick={() => setSelected(item.id)}
                  onDoubleClick={() => onSelect(item.id)}
                  className={cn(
                    'group overflow-hidden rounded-md border bg-muted text-left transition',
                    isSelected
                      ? 'ring-2 ring-primary'
                      : 'hover:ring-1 hover:ring-foreground/20'
                  )}
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
                  <div className="border-t bg-background p-1.5">
                    <p
                      className="truncate text-xs font-medium"
                      title={item.filename}
                    >
                      {item.filename}
                    </p>
                  </div>
                </button>
              );
            })}
          </div>
        )}
        <div className="flex items-center justify-between text-xs text-muted-foreground">
          <span>{total} items</span>
          <div className="flex items-center gap-1">
            <Button
              variant="outline"
              size="sm"
              disabled={page <= 1 || isLoading}
              onClick={() => setPage((p) => Math.max(1, p - 1))}
            >
              Prev
            </Button>
            <span>
              {page} / {totalPages}
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
        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
          >
            Cancel
          </Button>
          <Button
            type="button"
            disabled={!selected}
            onClick={() => selected && onSelect(selected)}
          >
            Select
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
