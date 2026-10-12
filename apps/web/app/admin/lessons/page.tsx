import { PageHeader } from '@/components/page-header';

import { CoursesTable } from './courses-table';

export default function LessonsListPage() {
  return (
    <>
      <PageHeader title="Lessons" />
      <CoursesTable />
    </>
  );
}
