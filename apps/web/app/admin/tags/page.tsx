import { TagsTable } from './tags-table';

export default function TagsListPage() {
  return (
    <div className="flex flex-col gap-4">
      <h1 className="text-2xl font-semibold">Tags</h1>
      <TagsTable />
    </div>
  );
}
