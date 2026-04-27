import { CategoriesTable } from './categories-table';

export default function CategoriesListPage() {
  return (
    <div className="flex flex-col gap-4">
      <h1 className="text-2xl font-semibold">Categories</h1>
      <CategoriesTable />
    </div>
  );
}
