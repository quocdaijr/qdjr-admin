import { PageHeader } from '@/components/page-header';

import { SettingsForm } from './settings-form';

export default function SettingsPage() {
  return (
    <>
      <PageHeader title="Site settings" />
      <SettingsForm />
    </>
  );
}
