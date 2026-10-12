import { PageHeader } from '@/components/page-header';

import { ProfileForm } from './profile-form';

export default function ProfilePage() {
  return (
    <>
      <PageHeader title="Profile" />
      <ProfileForm />
    </>
  );
}
