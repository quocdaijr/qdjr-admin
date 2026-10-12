import { LessonStatus } from '@/gen/lessons/v1/lessons_pb';
import { statusKey, statusLabel } from '@/lib/lesson-status';
import { cn } from '@/lib/utils';

const DOT: Record<ReturnType<typeof statusKey>, string> = {
  done: 'bg-primary',
  in_progress: 'border border-primary bg-transparent',
  todo: 'border border-muted-foreground bg-transparent',
};

export function LessonChip({ status }: { status: LessonStatus }) {
  const key = statusKey(status);
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 font-mono text-[11px] uppercase tracking-[0.06em]',
        key === 'todo' ? 'text-muted-foreground' : 'text-foreground'
      )}
    >
      <span aria-hidden="true" className={cn('size-1.5 rounded-full', DOT[key])} />
      {statusLabel(status)}
    </span>
  );
}
