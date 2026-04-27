import { SettingsForm } from './settings-form';

export default function SettingsPage() {
  return (
    <div className="flex flex-col gap-4">
      <h1 className="text-2xl font-semibold">Site settings</h1>
      <SettingsForm />
    </div>
  );
}
