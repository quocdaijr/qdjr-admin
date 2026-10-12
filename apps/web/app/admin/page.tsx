'use client';

import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { ArrowRight } from 'lucide-react';

import { apiList } from '@/lib/api';
import { timeAgo } from '@/lib/format';
import type { ContactMessage, Post } from '@/lib/types';
import { Skeleton } from '@/components/ui/skeleton';
import { StatusChip } from '@/components/status-chip';

const DESK_SIZE = 5;

function usePosts(status: 'draft' | 'published') {
  return useQuery({
    queryKey: ['posts', 'desk', status],
    queryFn: () => apiList<Post>(`/v1/admin/posts?status=${status}&page=1&perPage=${DESK_SIZE}`),
  });
}

function Panel({ title, count, href, children }: { title: string; count?: number; href: string; children: React.ReactNode }) {
  return (
    <section className="flex min-w-0 flex-col rounded-[10px] border bg-card">
      <header className="flex items-center justify-between border-b px-4 py-3">
        <h2 className="text-sm font-semibold">
          {title}
          {count !== undefined ? <span className="ml-2 font-mono text-xs text-muted-foreground">{count}</span> : null}
        </h2>
        <Link href={href} className="inline-flex items-center gap-1 whitespace-nowrap text-xs text-muted-foreground hover:text-primary">
          View all <ArrowRight className="size-3" aria-hidden="true" />
        </Link>
      </header>
      <div className="flex flex-col">{children}</div>
    </section>
  );
}

function Rows({ loading, empty, children }: { loading: boolean; empty: string; children: React.ReactNode[] }) {
  if (loading) {
    return (
      <div className="flex flex-col gap-2 p-4">
        <Skeleton className="h-4 w-2/3" />
        <Skeleton className="h-4 w-1/2" />
      </div>
    );
  }
  if (children.length === 0) return <p className="px-4 py-6 text-sm text-muted-foreground">{empty}</p>;
  return <ul className="divide-y">{children}</ul>;
}

export default function DeskPage() {
  const drafts = usePosts('draft');
  const published = usePosts('published');
  const inbox = useQuery({
    queryKey: ['contact-messages', 'desk'],
    queryFn: () => apiList<ContactMessage>(`/v1/admin/contact-messages?status=new&page=1&perPage=3`),
  });

  const draftRows = (drafts.data?.data ?? []).map((post) => (
    <li key={post.id}>
      <Link href={`/admin/posts/${post.id}`} className="flex items-center gap-3 px-4 py-3 hover:bg-muted">
        <span className="min-w-0 flex-1 truncate text-sm font-medium">{post.title || 'Untitled'}</span>
        <span className="hidden font-mono text-[11px] text-muted-foreground sm:inline">{timeAgo(post.updated_at)}</span>
      </Link>
    </li>
  ));

  const publishedRows = (published.data?.data ?? []).map((post) => (
    <li key={post.id}>
      <Link href={`/admin/posts/${post.id}`} className="flex items-center gap-3 px-4 py-3 hover:bg-muted">
        <StatusChip status="published" />
        <span className="min-w-0 flex-1 truncate text-sm">{post.title}</span>
        <span className="hidden font-mono text-[11px] text-muted-foreground sm:inline">/{post.slug}</span>
      </Link>
    </li>
  ));

  const inboxRows = (inbox.data?.data ?? []).map((message) => (
    <li key={message.id}>
      <Link href="/admin/inbox" className="flex flex-col gap-0.5 px-4 py-3 hover:bg-muted">
        <span className="flex items-baseline justify-between gap-3">
          <span className="truncate text-sm font-medium">{message.name}</span>
          <span className="shrink-0 font-mono text-[11px] text-muted-foreground">{timeAgo(message.created_at)}</span>
        </span>
        <span className="truncate text-xs text-muted-foreground">{message.subject || message.body}</span>
      </Link>
    </li>
  ));

  return (
    <>
      <header className="flex flex-wrap items-end justify-between gap-4 border-b pb-6">
        <h1 className="text-3xl font-semibold">Desk</h1>
        <Link
          href="/admin/posts/new"
          className="group inline-flex items-center gap-2 whitespace-nowrap text-sm font-medium text-foreground underline decoration-primary/40 underline-offset-4 hover:decoration-primary"
        >
          Write a new post <ArrowRight className="size-4 transition-transform group-hover:translate-x-0.5" aria-hidden="true" />
        </Link>
      </header>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
        <Panel title="Drafts in progress" count={drafts.data?.meta.total} href="/admin/posts">
          <Rows loading={drafts.isLoading} empty="No drafts. Everything is published or archived.">{draftRows}</Rows>
        </Panel>
        <Panel title="New in inbox" count={inbox.data?.meta.total} href="/admin/inbox">
          <Rows loading={inbox.isLoading} empty="Inbox zero.">{inboxRows}</Rows>
        </Panel>
      </div>

      <Panel title="Recently published" count={published.data?.meta.total} href="/admin/posts">
        <Rows loading={published.isLoading} empty="Nothing published yet.">{publishedRows}</Rows>
      </Panel>
    </>
  );
}
