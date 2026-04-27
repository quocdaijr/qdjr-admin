import { ProfileForm } from './profile-form';

export default function ProfilePage() {
  return (
    <div className="flex flex-col gap-4">
      <h1 className="text-2xl font-semibold">Profile</h1>
      <ProfileForm />
    </div>
  );
}
