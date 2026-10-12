import { PageHeader } from '@/components/page-header';

import { TagsTable } from './tags-table';

export default function TagsListPage() {
  return (
    <>
      <PageHeader title="Tags" />
      <TagsTable />
    </>
  );
}
