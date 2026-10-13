'use client';

import { useCallback, useRef, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { Upload, X } from 'lucide-react';
import { toast } from 'sonner';

import { ApiError } from '@/lib/api';
import { sendWithSession } from '@/lib/auth';
import { formatBytes } from '@/lib/format';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

type UploadStatus = 'pending' | 'uploading' | 'success' | 'error';

interface UploadItem {
  id: string;
  file: File;
  status: UploadStatus;
  progress: number;
  error?: string;
}

export function UploadDropzone() {
  const queryClient = useQueryClient();
  const [items, setItems] = useState<UploadItem[]>([]);
  const [dragOver, setDragOver] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  const updateItem = (id: string, patch: Partial<UploadItem>) => {
    setItems((prev) => prev.map((it) => (it.id === id ? { ...it, ...patch } : it)));
  };

  const uploadOne = useCallback(
    async (item: UploadItem) => {
      const { id, file } = item;
      try {
        updateItem(id, { status: 'uploading', progress: 0 });

        // One multipart request: the API sniffs the type, stores the file and
        // records it. XHR (not fetch) so the bar can show upload progress.
        const send = (token: string | null) =>
          new Promise<{ status: number; body: string }>((resolve, reject) => {
            const form = new FormData();
            form.append('file', file);
            const xhr = new XMLHttpRequest();
            xhr.open('POST', `${process.env.NEXT_PUBLIC_API_URL}/v1/admin/media`);
            if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`);
            xhr.upload.onprogress = (e) => {
              if (e.lengthComputable) {
                updateItem(id, { progress: Math.round((e.loaded / e.total) * 100) });
              }
            };
            xhr.onload = () => resolve({ status: xhr.status, body: xhr.responseText });
            xhr.onerror = () => reject(new Error('Network error during upload'));
            xhr.send(form);
          });
        // Same session rule as every other call: refresh once on 401, sign out
        // (and so redirect to /login) if the session cannot be recovered.
        const res = await sendWithSession(send);
        if (res.status === 401) throw new Error('Session expired, sign in again');
        if (res.status < 200 || res.status >= 300) {
          let message = `Upload failed (${res.status})`;
          try {
            const body = JSON.parse(res.body) as { error?: { message?: string } };
            message = body.error?.message ?? message;
          } catch {
            // non-JSON error body: keep the status message
          }
          throw new Error(message);
        }

        updateItem(id, { status: 'success', progress: 100 });
        toast.success(`${file.name} uploaded`);
        queryClient.invalidateQueries({ queryKey: ['media'] });
      } catch (err) {
        const message =
          err instanceof ApiError
            ? `${err.code}: ${err.message}`
            : err instanceof Error
              ? err.message
              : 'Upload failed';
        updateItem(id, { status: 'error', error: message });
        toast.error(`${item.file.name}: ${message}`);
      }
    },
    [queryClient]
  );

  const enqueue = useCallback(
    (files: FileList | File[]) => {
      const newItems: UploadItem[] = Array.from(files).map((file) => ({
        id: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
        file,
        status: 'pending',
        progress: 0,
      }));
      setItems((prev) => [...prev, ...newItems]);
      newItems.forEach((it) => {
        uploadOne(it);
      });
    },
    [uploadOne]
  );

  const onDrop = (e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setDragOver(false);
    if (e.dataTransfer.files.length > 0) enqueue(e.dataTransfer.files);
  };

  const onPick = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files.length > 0) enqueue(e.target.files);
    e.target.value = '';
  };

  const clearItem = (id: string) => {
    setItems((prev) => prev.filter((it) => it.id !== id));
  };

  return (
    <div className="flex flex-col gap-3">
      <div
        onDragOver={(e) => {
          e.preventDefault();
          setDragOver(true);
        }}
        onDragLeave={() => setDragOver(false)}
        onDrop={onDrop}
        className={cn(
          'flex flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed p-8 text-center transition-colors',
          dragOver ? 'border-primary bg-primary/5' : 'border-muted-foreground/30'
        )}
      >
        <Upload className="size-8 text-muted-foreground" />
        <p className="text-sm">
          Drag and drop files here, or
        </p>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => inputRef.current?.click()}
        >
          Choose files
        </Button>
        <input
          ref={inputRef}
          type="file"
          accept="image/png,image/jpeg,image/webp,image/gif"
          multiple
          className="hidden"
          onChange={onPick}
        />
      </div>

      {items.length > 0 ? (
        <ul className="flex flex-col gap-2 rounded-lg border p-2">
          {items.map((item) => (
            <li
              key={item.id}
              className="flex items-center gap-3 rounded-md p-2 text-sm"
            >
              <div className="flex-1 min-w-0">
                <p className="truncate font-medium" title={item.file.name}>
                  {item.file.name}
                </p>
                <p className="text-xs text-muted-foreground">
                  {formatBytes(item.file.size)}
                </p>
                {item.status === 'uploading' ? (
                  <div className="mt-1 h-1 w-full overflow-hidden rounded-full bg-muted">
                    <div
                      className="h-full bg-primary transition-all"
                      style={{ width: `${item.progress}%` }}
                    />
                  </div>
                ) : null}
                {item.status === 'error' ? (
                  <p className="text-xs text-destructive">{item.error}</p>
                ) : null}
              </div>
              <span className="text-xs text-muted-foreground">
                {item.status === 'pending' && 'Queued'}
                {item.status === 'uploading' && `${item.progress}%`}
                {item.status === 'success' && 'Done'}
                {item.status === 'error' && 'Failed'}
              </span>
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                aria-label="Remove"
                onClick={() => clearItem(item.id)}
              >
                <X className="size-4" />
              </Button>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}
