import { UsersTable } from './users-table';

export default function UsersPage() {
  return (
    <div className="flex flex-col gap-4">
      <h1 className="text-2xl font-semibold">Users</h1>
      <UsersTable />
    </div>
  );
}
