'use client';

import { useEffect } from 'react';
import { useForm, Controller } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { z } from 'zod';
import { toast } from 'sonner';

import { apiGet, apiPatch, ApiError } from '@/lib/api';
import type { Profile } from '@/lib/types';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Skeleton } from '@/components/ui/skeleton';
import { MediaPicker } from '@/components/media-picker';
import {
  SocialLinksEditor,
  socialLinksToRows,
  rowsToSocialLinks,
} from '@/components/social-links-editor';

const schema = z.object({
  full_name: z.string().max(200).optional().or(z.literal('')),
  bio: z.string().max(5000).optional().or(z.literal('')),
  avatar_id: z.string().uuid().nullable().optional(),
  tagline: z.string().max(300).optional().or(z.literal('')),
  location: z.string().max(200).optional().or(z.literal('')),
  email: z
    .string()
    .email('Invalid email')
    .optional()
    .or(z.literal('')),
  social_links: z.array(z.object({ key: z.string(), url: z.string() })),
});

type FormValues = z.infer<typeof schema>;

function defaults(p?: Profile | null): FormValues {
  return {
    full_name: p?.full_name ?? '',
    bio: p?.bio ?? '',
    avatar_id: p?.avatar_id ?? null,
    tagline: p?.tagline ?? '',
    location: p?.location ?? '',
    email: p?.email ?? '',
    social_links: socialLinksToRows(p?.social_links),
  };
}

export function ProfileForm() {
  const queryClient = useQueryClient();
  const profileQuery = useQuery({
    queryKey: ['profile'],
    queryFn: () => apiGet<Profile>('/v1/admin/profile'),
  });

  const {
    register,
    handleSubmit,
    control,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: defaults(null),
  });

  useEffect(() => {
    if (profileQuery.data) {
      reset(defaults(profileQuery.data));
    }
  }, [profileQuery.data, reset]);

  const updateMutation = useMutation({
    mutationFn: async (values: FormValues) =>
      apiPatch<Profile>('/v1/admin/profile', toApiBody(values)),
    onSuccess: () => {
      toast.success('Profile saved');
      queryClient.invalidateQueries({ queryKey: ['profile'] });
    },
    onError: handleApiError,
  });

  const onSubmit = handleSubmit((values) => updateMutation.mutate(values));
  const submitting = isSubmitting || updateMutation.isPending;

  if (profileQuery.isLoading) {
    return <Skeleton className="h-64 w-full max-w-2xl" />;
  }

  if (profileQuery.isError) {
    return (
      <p className="text-sm text-red-600">
        {profileQuery.error instanceof Error
          ? profileQuery.error.message
          : 'Failed to load profile'}
      </p>
    );
  }

  return (
    <form onSubmit={onSubmit} className="flex max-w-2xl flex-col gap-4">
      <Field label="Full name" error={errors.full_name?.message}>
        <Input {...register('full_name')} placeholder="Your name" />
      </Field>

      <Field label="Tagline" error={errors.tagline?.message}>
        <Input {...register('tagline')} placeholder="Short tagline" />
      </Field>

      <Field label="Bio" error={errors.bio?.message}>
        <Textarea {...register('bio')} rows={5} placeholder="About you" />
      </Field>

      <Field label="Avatar" error={errors.avatar_id?.message as string | undefined}>
        {profileQuery.data?.avatar_url && !profileQuery.data?.avatar_id ? (
          <p className="text-xs text-muted-foreground">
            Current avatar:{' '}
            <a
              href={profileQuery.data.avatar_url}
              target="_blank"
              rel="noreferrer"
              className="underline"
            >
              view
            </a>
            . Pick a new image to replace.
          </p>
        ) : null}
        <Controller
          control={control}
          name="avatar_id"
          render={({ field }) => (
            <MediaPicker
              value={field.value ?? null}
              onChange={(id) => field.onChange(id)}
            />
          )}
        />
      </Field>

      <Field label="Location" error={errors.location?.message}>
        <Input {...register('location')} placeholder="City, Country" />
      </Field>

      <Field label="Email" error={errors.email?.message}>
        <Input
          {...register('email')}
          type="email"
          placeholder="you@example.com"
        />
      </Field>

      <Field label="Social links">
        <Controller
          control={control}
          name="social_links"
          render={({ field }) => (
            <SocialLinksEditor value={field.value} onChange={field.onChange} />
          )}
        />
      </Field>

      <div>
        <Button type="submit" disabled={submitting}>
          {submitting ? 'Saving…' : 'Save'}
        </Button>
      </div>
    </form>
  );
}

function Field({
  label,
  error,
  children,
}: {
  label: string;
  error?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <Label>{label}</Label>
      {children}
      {error ? <p className="text-xs text-destructive">{error}</p> : null}
    </div>
  );
}

function toApiBody(values: FormValues) {
  return {
    full_name:
      values.full_name && values.full_name !== '' ? values.full_name : null,
    bio: values.bio && values.bio !== '' ? values.bio : null,
    avatar_id: values.avatar_id ?? null,
    tagline: values.tagline && values.tagline !== '' ? values.tagline : null,
    location:
      values.location && values.location !== '' ? values.location : null,
    email: values.email && values.email !== '' ? values.email : null,
    social_links: rowsToSocialLinks(values.social_links),
  };
}

function handleApiError(err: unknown) {
  const message =
    err instanceof ApiError
      ? `${err.code}: ${err.message}`
      : err instanceof Error
        ? err.message
        : 'Save failed';
  toast.error(message);
}
