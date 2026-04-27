'use client';

import { useEffect } from 'react';
import { useForm, Controller } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { z } from 'zod';
import { toast } from 'sonner';

import { apiGet, apiPatch, ApiError } from '@/lib/api';
import type { SiteSettings } from '@/lib/types';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Skeleton } from '@/components/ui/skeleton';
import {
  SocialLinksEditor,
  socialLinksToRows,
  rowsToSocialLinks,
} from '@/components/social-links-editor';

const schema = z.object({
  site_title: z.string().min(1, 'Required').max(200),
  site_description: z.string().max(1000).optional().or(z.literal('')),
  footer_text: z.string().max(2000).optional().or(z.literal('')),
  contact_email: z
    .string()
    .email('Invalid email')
    .optional()
    .or(z.literal('')),
  social_links: z.array(z.object({ key: z.string(), url: z.string() })),
});

type FormValues = z.infer<typeof schema>;

function defaults(s?: SiteSettings | null): FormValues {
  return {
    site_title: s?.site_title ?? '',
    site_description: s?.site_description ?? '',
    footer_text: s?.footer_text ?? '',
    contact_email: s?.contact_email ?? '',
    social_links: socialLinksToRows(s?.social_links),
  };
}

export function SettingsForm() {
  const queryClient = useQueryClient();
  const settingsQuery = useQuery({
    queryKey: ['site-settings'],
    queryFn: () => apiGet<SiteSettings>('/v1/admin/site-settings'),
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
    if (settingsQuery.data) {
      reset(defaults(settingsQuery.data));
    }
  }, [settingsQuery.data, reset]);

  const updateMutation = useMutation({
    mutationFn: async (values: FormValues) =>
      apiPatch<SiteSettings>('/v1/admin/site-settings', toApiBody(values)),
    onSuccess: () => {
      toast.success('Settings saved');
      queryClient.invalidateQueries({ queryKey: ['site-settings'] });
    },
    onError: handleApiError,
  });

  const onSubmit = handleSubmit((values) => updateMutation.mutate(values));
  const submitting = isSubmitting || updateMutation.isPending;

  if (settingsQuery.isLoading) {
    return <Skeleton className="h-64 w-full max-w-2xl" />;
  }

  if (settingsQuery.isError) {
    return (
      <p className="text-sm text-red-600">
        {settingsQuery.error instanceof Error
          ? settingsQuery.error.message
          : 'Failed to load settings'}
      </p>
    );
  }

  return (
    <form onSubmit={onSubmit} className="flex max-w-2xl flex-col gap-4">
      <Field label="Site title" error={errors.site_title?.message}>
        <Input {...register('site_title')} placeholder="My site" />
      </Field>

      <Field label="Site description" error={errors.site_description?.message}>
        <Textarea
          {...register('site_description')}
          rows={3}
          placeholder="Short description for the site"
        />
      </Field>

      <Field label="Footer text" error={errors.footer_text?.message}>
        <Textarea
          {...register('footer_text')}
          rows={3}
          placeholder="Footer text"
        />
      </Field>

      <Field label="Contact email" error={errors.contact_email?.message}>
        <Input
          {...register('contact_email')}
          type="email"
          placeholder="hello@example.com"
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
    site_title: values.site_title,
    site_description:
      values.site_description && values.site_description !== ''
        ? values.site_description
        : null,
    footer_text:
      values.footer_text && values.footer_text !== ''
        ? values.footer_text
        : null,
    contact_email:
      values.contact_email && values.contact_email !== ''
        ? values.contact_email
        : null,
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
