'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { timestampDate } from '@bufbuild/protobuf/wkt';
import { MoreHorizontal } from 'lucide-react';
import { toast } from 'sonner';

import type { Course } from '@/gen/lessons/v1/lessons_pb';
import { lessonsClient, rpcErrorMessage } from '@/lib/rpc';
import { timeAgo } from '@/lib/format';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
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

import { CourseDialog } from './course-dialog';

const PER_PAGE = 20;

export function CoursesTable() {
  const queryClient = useQueryClient();
  const [page, setPage] = useState(1);
  const [q, setQ] = useState('');
  const [qInput, setQInput] = useState('');
  const [pendingDelete, setPendingDelete] = useState<Course | null>(null);
  const [editTarget, setEditTarget] = useState<Course | null>(null);
  const [createOpen, setCreateOpen] = useState(false);

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ['courses', page, PER_PAGE, q] as const,
    queryFn: () => lessonsClient.listCourses({ page, perPage: PER_PAGE, q }),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => lessonsClient.deleteCourse({ id }),
    onSuccess: () => {
      setPendingDelete(null);
      queryClient.invalidateQueries({ queryKey: ['courses'] });
    },
    onError: (err) => toast.error(rpcErrorMessage(err, 'Delete failed')),
  });

  const total = data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PER_PAGE));
  const rows = data?.courses ?? [];

  const onSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setPage(1);
    setQ(qInput.trim());
  };

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <form onSubmit={onSearchSubmit} className="flex w-full min-w-0 items-center gap-2 sm:w-auto">
          <Input
            placeholder="Search title…"
            value={qInput}
            onChange={(e) => setQInput(e.target.value)}
            className="min-w-0 flex-1 sm:w-64 sm:flex-none"
          />
          <Button type="submit" variant="outline" size="sm">
            Search
          </Button>
        </form>
        <Button onClick={() => setCreateOpen(true)}>New course</Button>
      </div>

      <div className="overflow-x-auto rounded-[10px] border bg-card">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Title</TableHead>
              <TableHead>Provider</TableHead>
              <TableHead>Progress</TableHead>
              <TableHead>Updated</TableHead>
              <TableHead className="w-[60px]" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              Array.from({ length: 5 }).map((_, i) => (
                <TableRow key={`skeleton-${i}`}>
                  <TableCell>
                    <Skeleton className="h-4 w-48" />
                  </TableCell>
                  <TableCell>
                    <Skeleton className="h-4 w-24" />
                  </TableCell>
                  <TableCell>
                    <Skeleton className="h-4 w-12" />
                  </TableCell>
                  <TableCell>
                    <Skeleton className="h-4 w-20" />
                  </TableCell>
                  <TableCell />
                </TableRow>
              ))
            ) : isError ? (
              <TableRow>
                <TableCell colSpan={5} className="text-center text-destructive">
                  {rpcErrorMessage(error, 'Failed to load courses')}
                </TableCell>
              </TableRow>
            ) : rows.length === 0 ? (
              <TableRow>
                <TableCell colSpan={5} className="text-center text-muted-foreground">
                  No courses yet.
                </TableCell>
              </TableRow>
            ) : (
              rows.map((course) => (
                <TableRow key={course.id}>
                  <TableCell className="font-medium">
                    <Link href={`/admin/lessons/${course.id}`} className="hover:underline">
                      {course.title}
                    </Link>
                  </TableCell>
                  <TableCell className="text-muted-foreground">{course.provider || '—'}</TableCell>
                  <TableCell className="font-mono text-[11px] uppercase tracking-[0.06em]">
                    {course.doneCount}/{course.lessonCount}
                  </TableCell>
                  <TableCell className="font-mono text-[11px] text-muted-foreground">
                    {course.updatedAt ? timeAgo(timestampDate(course.updatedAt).toISOString()) : '—'}
                  </TableCell>
                  <TableCell>
                    <DropdownMenu>
                      <DropdownMenuTrigger render={<Button variant="ghost" size="icon-sm" aria-label="Actions" />}>
                        <MoreHorizontal className="size-4" />
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={() => setEditTarget(course)}>Edit</DropdownMenuItem>
                        <DropdownMenuSeparator />
                        <DropdownMenuItem variant="destructive" onClick={() => setPendingDelete(course)}>
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

      <div className="flex items-center justify-between text-sm text-muted-foreground">
        <span>
          {total} course{total === 1 ? '' : 's'}
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

      <CourseDialog open={createOpen} onOpenChange={setCreateOpen} />
      <CourseDialog
        open={editTarget !== null}
        onOpenChange={(open) => !open && setEditTarget(null)}
        course={editTarget}
      />

      <Dialog open={pendingDelete !== null} onOpenChange={(open) => !open && setPendingDelete(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete course?</DialogTitle>
            <DialogDescription>
              This deletes &quot;{pendingDelete?.title}&quot; and all its lessons. This cannot be undone.
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
