import { PageHeader } from '@/components/page-header';

import { CategoriesTable } from './categories-table';

export default function CategoriesListPage() {
  return (
    <>
      <PageHeader title="Categories" />
      <CategoriesTable />
    </>
  );
}
