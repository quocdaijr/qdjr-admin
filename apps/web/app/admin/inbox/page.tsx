import { Inbox } from './inbox';

export default function InboxPage() {
  return (
    <div className="flex flex-col gap-4">
      <h1 className="text-2xl font-semibold">Contact inbox</h1>
      <Inbox />
    </div>
  );
}
