import { LessonStatus } from '@/gen/lessons/v1/lessons_pb';

export type LessonStatusKey = 'todo' | 'in_progress' | 'done';

/** Order is the display order in selects. */
export const LESSON_STATUSES: ReadonlyArray<{
  value: LessonStatus;
  key: LessonStatusKey;
  label: string;
}> = [
  { value: LessonStatus.TODO, key: 'todo', label: 'To do' },
  { value: LessonStatus.IN_PROGRESS, key: 'in_progress', label: 'In progress' },
  { value: LessonStatus.DONE, key: 'done', label: 'Done' },
];

export function statusKey(s: LessonStatus): LessonStatusKey {
  return LESSON_STATUSES.find((x) => x.value === s)?.key ?? 'todo';
}

export function statusFromKey(k: string): LessonStatus {
  return LESSON_STATUSES.find((x) => x.key === k)?.value ?? LessonStatus.TODO;
}

export function statusLabel(s: LessonStatus): string {
  return LESSON_STATUSES.find((x) => x.value === s)?.label ?? 'To do';
}
