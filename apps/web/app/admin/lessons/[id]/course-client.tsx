'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { timestampDate } from '@bufbuild/protobuf/wkt';
import { MoreHorizontal } from 'lucide-react';
import { toast } from 'sonner';

import type { Lesson } from '@/gen/lessons/v1/lessons_pb';
import { lessonsClient, rpcErrorMessage } from '@/lib/rpc';
import { LESSON_STATUSES, statusFromKey, statusKey } from '@/lib/lesson-status';
import { PageHeader } from '@/components/page-header';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

import { CourseDialog } from '../course-dialog';
import { LessonChip } from '../lesson-chip';
import { LessonDialog } from '../lesson-dialog';

export function CourseClient({ id }: { id: string }) {
  const queryClient = useQueryClient();
  const [editCourse, setEditCourse] = useState(false);
  const [addLesson, setAddLesson] = useState(false);
  const [editLesson, setEditLesson] = useState<Lesson | null>(null);
  const [pendingDelete, setPendingDelete] = useState<Lesson | null>(null);

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ['course', id],
    queryFn: () => lessonsClient.getCourse({ id }),
  });

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['course', id] });
    queryClient.invalidateQueries({ queryKey: ['courses'] });
  };

  const statusMutation = useMutation({
    mutationFn: ({ lessonId, key }: { lessonId: string; key: string }) =>
      lessonsClient.updateLesson({ id: lessonId, status: statusFromKey(key) }),
    onSuccess: invalidate,
    onError: (err) => toast.error(rpcErrorMessage(err, 'Status change failed')),
  });

  const deleteMutation = useMutation({
    mutationFn: (lessonId: string) => lessonsClient.deleteLesson({ id: lessonId }),
    onSuccess: () => {
      setPendingDelete(null);
      invalidate();
    },
    onError: (err) => toast.error(rpcErrorMessage(err, 'Delete failed')),
  });

  if (isLoading) {
    return (
      <div className="flex flex-col gap-4">
        <Skeleton className="h-10 w-64" />
        <Skeleton className="h-8 w-full" />
        <Skeleton className="h-8 w-full" />
        <Skeleton className="h-8 w-full" />
      </div>
    );
  }
  if (isError) {
    return <p className="text-sm text-destructive">{rpcErrorMessage(error, 'Failed to load course')}</p>;
  }
  if (!data?.course) return null;

  const course = data.course;
  const lessons = data.lessons;

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title={course.title}
        meta={
          <>
            {course.provider || 'no provider'} · {course.doneCount}/{course.lessonCount} done
            {course.url ? (
              <>
                {' · '}
                <a href={course.url} target="_blank" rel="noreferrer" className="hover:underline">
                  open
                </a>
              </>
            ) : null}
          </>
        }
        actions={
          <>
            <Button variant="outline" onClick={() => setEditCourse(true)}>
              Edit course
            </Button>
            <Button onClick={() => setAddLesson(true)}>New lesson</Button>
          </>
        }
      />

      {course.notes ? (
        <pre className="whitespace-pre-wrap rounded-[10px] border bg-card p-4 font-mono text-[13px] leading-6">
          {course.notes}
        </pre>
      ) : null}

      <div className="overflow-x-auto rounded-[10px] border bg-card">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-[48px]">#</TableHead>
              <TableHead>Lesson</TableHead>
              <TableHead className="w-[180px]">Status</TableHead>
              <TableHead>Completed</TableHead>
              <TableHead className="w-[60px]" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {lessons.length === 0 ? (
              <TableRow>
                <TableCell colSpan={5} className="text-center text-muted-foreground">
                  No lessons yet.
                </TableCell>
              </TableRow>
            ) : (
              lessons.map((lesson) => (
                <TableRow key={lesson.id}>
                  <TableCell className="font-mono text-[11px] text-muted-foreground">{lesson.position}</TableCell>
                  <TableCell className="font-medium">
                    {lesson.url ? (
                      <a href={lesson.url} target="_blank" rel="noreferrer" className="hover:underline">
                        {lesson.title}
                      </a>
                    ) : (
                      lesson.title
                    )}
                    {lesson.notes ? (
                      <p className="mt-1 line-clamp-2 max-w-md text-xs text-muted-foreground">{lesson.notes}</p>
                    ) : null}
                  </TableCell>
                  <TableCell>
                    <Select
                      value={statusKey(lesson.status)}
                      onValueChange={(key) => key && statusMutation.mutate({ lessonId: lesson.id, key })}
                    >
                      <SelectTrigger size="sm" aria-label={`Status of ${lesson.title}`}>
                        <SelectValue>
                          <LessonChip status={lesson.status} />
                        </SelectValue>
                      </SelectTrigger>
                      <SelectContent>
                        {LESSON_STATUSES.map((s) => (
                          <SelectItem key={s.key} value={s.key}>
                            {s.label}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </TableCell>
                  <TableCell className="font-mono text-[11px] text-muted-foreground">
                    {lesson.completedAt ? timestampDate(lesson.completedAt).toLocaleDateString() : '—'}
                  </TableCell>
                  <TableCell>
                    <DropdownMenu>
                      <DropdownMenuTrigger render={<Button variant="ghost" size="icon-sm" aria-label="Actions" />}>
                        <MoreHorizontal className="size-4" />
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={() => setEditLesson(lesson)}>Edit</DropdownMenuItem>
                        <DropdownMenuSeparator />
                        <DropdownMenuItem variant="destructive" onClick={() => setPendingDelete(lesson)}>
                          Delete
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      <CourseDialog open={editCourse} onOpenChange={setEditCourse} course={course} />
      <LessonDialog open={addLesson} onOpenChange={setAddLesson} courseId={id} />
      <LessonDialog
        open={editLesson !== null}
        onOpenChange={(open) => !open && setEditLesson(null)}
        courseId={id}
        lesson={editLesson}
      />

      <Dialog open={pendingDelete !== null} onOpenChange={(open) => !open && setPendingDelete(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete lesson?</DialogTitle>
            <DialogDescription>
              This deletes &quot;{pendingDelete?.title}&quot;. This cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setPendingDelete(null)} disabled={deleteMutation.isPending}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={() => pendingDelete && deleteMutation.mutate(pendingDelete.id)}
              disabled={deleteMutation.isPending}
            >
              {deleteMutation.isPending ? 'Deleting…' : 'Delete'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
