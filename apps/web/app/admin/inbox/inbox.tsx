'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';

import { apiList, apiPatch, ApiError } from '@/lib/api';
import type { ContactMessage } from '@/lib/types';
import { timeAgo } from '@/lib/format';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';

type Status = ContactMessage['status'];

const TABS: Array<{ value: Status; label: string }> = [
  { value: 'new', label: 'New' },
  { value: 'read', label: 'Read' },
  { value: 'replied', label: 'Replied' },
  { value: 'spam', label: 'Spam' },
];

const PER_PAGE = 20;

export function Inbox() {
  const [tab, setTab] = useState<Status>('new');
  const [selected, setSelected] = useState<ContactMessage | null>(null);

  return (
    <Tabs
      value={tab}
      onValueChange={(v) => {
        if (typeof v === 'string') setTab(v as Status);
      }}
    >
      <TabsList>
        {TABS.map((t) => (
          <TabsTrigger key={t.value} value={t.value}>
            {t.label}
          </TabsTrigger>
        ))}
      </TabsList>

      {TABS.map((t) => (
        <TabsContent key={t.value} value={t.value}>
          <InboxTab
            status={t.value}
            active={tab === t.value}
            onSelect={setSelected}
          />
        </TabsContent>
      ))}

      <MessageSheet
        message={selected}
        onOpenChange={(open) => {
          if (!open) setSelected(null);
        }}
      />
    </Tabs>
  );
}

interface InboxTabProps {
  status: Status;
  active: boolean;
  onSelect: (msg: ContactMessage) => void;
}

function InboxTab({ status, active, onSelect }: InboxTabProps) {
  const [page, setPage] = useState(1);

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ['contact-messages', status, page, PER_PAGE],
    queryFn: () =>
      apiList<ContactMessage>(
        `/v1/admin/contact-messages?status=${status}&page=${page}&perPage=${PER_PAGE}`
      ),
    enabled: active,
  });

  const total = data?.meta.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PER_PAGE));
  const rows = data?.data ?? [];

  return (
    <div className="flex flex-col gap-3">
      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>From</TableHead>
              <TableHead>Subject</TableHead>
              <TableHead className="w-[160px]">Received</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              Array.from({ length: 5 }).map((_, i) => (
                <TableRow key={`skeleton-${i}`}>
                  <TableCell>
                    <Skeleton className="h-4 w-40" />
                  </TableCell>
                  <TableCell>
                    <Skeleton className="h-4 w-64" />
                  </TableCell>
                  <TableCell>
                    <Skeleton className="h-4 w-24" />
                  </TableCell>
                </TableRow>
              ))
            ) : isError ? (
              <TableRow>
                <TableCell colSpan={3} className="text-center text-red-600">
                  {error instanceof Error
                    ? error.message
                    : 'Failed to load messages'}
                </TableCell>
              </TableRow>
            ) : rows.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={3}
                  className="text-center text-muted-foreground"
                >
                  No messages.
                </TableCell>
              </TableRow>
            ) : (
              rows.map((msg) => (
                <TableRow
                  key={msg.id}
                  className="cursor-pointer"
                  onClick={() => onSelect(msg)}
                >
                  <TableCell className="font-medium">
                    <div className="flex flex-col">
                      <span>{msg.name}</span>
                      <span className="text-xs text-muted-foreground">
                        {msg.email}
                      </span>
                    </div>
                  </TableCell>
                  <TableCell className="max-w-md truncate">
                    {msg.subject ?? '(no subject)'}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {timeAgo(msg.created_at)}
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      <div className="flex items-center justify-between text-sm text-muted-foreground">
        <span>
          {total} message{total === 1 ? '' : 's'}
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
    </div>
  );
}

interface MessageSheetProps {
  message: ContactMessage | null;
  onOpenChange: (open: boolean) => void;
}

function MessageSheet({ message, onOpenChange }: MessageSheetProps) {
  const queryClient = useQueryClient();

  const setStatusMutation = useMutation({
    mutationFn: async ({ id, status }: { id: string; status: Status }) =>
      apiPatch<ContactMessage>(`/v1/admin/contact-messages/${id}`, { status }),
    onSuccess: (_data, vars) => {
      toast.success(`Marked ${vars.status}`);
      queryClient.invalidateQueries({ queryKey: ['contact-messages'] });
      onOpenChange(false);
    },
    onError: (err) => {
      const msg =
        err instanceof ApiError
          ? `${err.code}: ${err.message}`
          : err instanceof Error
            ? err.message
            : 'Update failed';
      toast.error(msg);
    },
  });

  const open = message !== null;
  const current = message?.status;

  const actions: Array<{ label: string; status: Status }> = [];
  if (current && current !== 'read')
    actions.push({ label: 'Mark read', status: 'read' });
  if (current && current !== 'replied')
    actions.push({ label: 'Mark replied', status: 'replied' });
  if (current && current !== 'spam')
    actions.push({ label: 'Mark spam', status: 'spam' });
  if (current === 'spam')
    actions.push({ label: 'Mark new', status: 'new' });

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        side="right"
        className="flex w-full flex-col gap-4 overflow-y-auto p-4 sm:max-w-lg"
      >
        <SheetHeader className="p-0">
          <SheetTitle>{message?.subject ?? '(no subject)'}</SheetTitle>
          <SheetDescription>
            {message ? `From ${message.name} <${message.email}>` : ''}
          </SheetDescription>
        </SheetHeader>

        {message ? (
          <div className="flex flex-col gap-4 text-sm">
            <div>
              <p className="text-xs uppercase text-muted-foreground">
                Received
              </p>
              <p>{new Date(message.created_at).toLocaleString()}</p>
            </div>
            <div>
              <p className="text-xs uppercase text-muted-foreground">Body</p>
              <p className="whitespace-pre-wrap rounded-md border bg-muted/40 p-3">
                {message.body}
              </p>
            </div>
            {message.ip ? (
              <div>
                <p className="text-xs uppercase text-muted-foreground">IP</p>
                <p className="font-mono text-xs">{message.ip}</p>
              </div>
            ) : null}
            {message.user_agent ? (
              <div>
                <p className="text-xs uppercase text-muted-foreground">
                  User agent
                </p>
                <p className="break-all font-mono text-xs">
                  {message.user_agent}
                </p>
              </div>
            ) : null}

            <div className="flex flex-wrap gap-2 pt-2">
              {actions.map((a) => (
                <Button
                  key={a.status}
                  variant="outline"
                  size="sm"
                  disabled={setStatusMutation.isPending}
                  onClick={() =>
                    setStatusMutation.mutate({ id: message.id, status: a.status })
                  }
                >
                  {a.label}
                </Button>
              ))}
            </div>
          </div>
        ) : null}
      </SheetContent>
    </Sheet>
  );
}
