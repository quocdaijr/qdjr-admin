import Link from 'next/link';

import { Button } from '@/components/ui/button';
import { PageHeader } from '@/components/page-header';

import { PostsTable } from './posts-table';

export default function PostsListPage() {
  return (
    <>
      <PageHeader title="Posts" actions={<Button nativeButton={false} render={<Link href="/admin/posts/new" />}>New post</Button>} />
      <PostsTable />
    </>
  );
}
