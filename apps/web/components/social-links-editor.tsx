'use client';

import { Plus, X } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

interface SocialLinksEditorProps {
  value: Array<{ key: string; url: string }>;
  onChange: (next: Array<{ key: string; url: string }>) => void;
}

export function SocialLinksEditor({ value, onChange }: SocialLinksEditorProps) {
  const updateRow = (idx: number, patch: Partial<{ key: string; url: string }>) => {
    const next = value.map((row, i) => (i === idx ? { ...row, ...patch } : row));
    onChange(next);
  };

  const removeRow = (idx: number) => {
    onChange(value.filter((_, i) => i !== idx));
  };

  const addRow = () => {
    onChange([...value, { key: '', url: '' }]);
  };

  return (
    <div className="flex flex-col gap-2">
      {value.length === 0 ? (
        <p className="text-xs text-muted-foreground">No links yet.</p>
      ) : (
        value.map((row, idx) => (
          <div key={idx} className="flex items-center gap-2">
            <Input
              placeholder="platform (e.g. twitter)"
              value={row.key}
              onChange={(e) => updateRow(idx, { key: e.target.value })}
              className="w-40"
            />
            <Input
              placeholder="https://…"
              value={row.url}
              onChange={(e) => updateRow(idx, { url: e.target.value })}
              className="flex-1"
            />
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              onClick={() => removeRow(idx)}
              aria-label="Remove link"
            >
              <X className="size-4" />
            </Button>
          </div>
        ))
      )}
      <div>
        <Button type="button" variant="outline" size="sm" onClick={addRow}>
          <Plus className="size-4" />
          Add link
        </Button>
      </div>
    </div>
  );
}

export function socialLinksToRows(
  obj: Record<string, string> | null | undefined
): Array<{ key: string; url: string }> {
  if (!obj) return [];
  return Object.entries(obj).map(([key, url]) => ({ key, url }));
}

export function rowsToSocialLinks(
  rows: Array<{ key: string; url: string }>
): Record<string, string> {
  const out: Record<string, string> = {};
  for (const { key, url } of rows) {
    const k = key.trim();
    const v = url.trim();
    if (k && v) out[k] = v;
  }
  return out;
}
