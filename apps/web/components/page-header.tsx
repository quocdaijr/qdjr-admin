import type { ReactNode } from 'react';

interface PageHeaderProps {
  title: string;
  /** One mono line under the title: a count, a slug, a status. Real values only. */
  meta?: ReactNode;
  actions?: ReactNode;
}

export function PageHeader({ title, meta, actions }: PageHeaderProps) {
  return (
    <header className="flex flex-wrap items-end justify-between gap-4 border-b pb-4">
      <div className="min-w-0">
        <h1 className="text-2xl font-semibold [overflow-wrap:anywhere]">{title}</h1>
        {meta ? (
          <p className="mt-1 font-mono text-[11px] uppercase tracking-[0.06em] text-muted-foreground">
            {meta}
          </p>
        ) : null}
      </div>
      {actions ? <div className="flex shrink-0 items-center gap-2">{actions}</div> : null}
    </header>
  );
}
