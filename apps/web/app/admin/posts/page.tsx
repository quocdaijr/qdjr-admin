import Link from 'next/link';

import { Button } from '@/components/ui/button';

import { PostsTable } from './posts-table';

export default function PostsListPage() {
  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Posts</h1>
        <Button render={<Link href="/admin/posts/new" />}>New post</Button>
      </div>
      <PostsTable />
    </div>
  );
}
