import { cn } from '@/lib/utils';

type Status = 'draft' | 'published' | 'archived';

const DOT: Record<Status, string> = {
  published: 'bg-primary',
  draft: 'border border-muted-foreground bg-transparent',
  archived: 'bg-muted-foreground/40',
};

export function StatusChip({ status }: { status: Status | undefined }) {
  const value = status ?? 'draft';
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 font-mono text-[11px] uppercase tracking-[0.06em]',
        value === 'archived' ? 'text-muted-foreground' : 'text-foreground'
      )}
    >
      <span aria-hidden="true" className={cn('size-1.5 rounded-full', DOT[value])} />
      {value}
    </span>
  );
}
