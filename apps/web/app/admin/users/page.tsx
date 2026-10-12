import { PageHeader } from '@/components/page-header';

import { UsersTable } from './users-table';

export default function UsersPage() {
  return (
    <>
      <PageHeader title="Users" />
      <UsersTable />
    </>
  );
}
