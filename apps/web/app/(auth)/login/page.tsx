import { LoginForm } from './login-form';

export const metadata = {
  title: 'Sign in — qdjr admin',
};

export default function LoginPage() {
  return (
    <main className="flex min-h-screen items-center justify-center bg-background px-4 py-12">
      <LoginForm />
    </main>
  );
}
