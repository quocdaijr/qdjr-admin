'use client';

import { useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import { useForm, Controller } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { z } from 'zod';
import { toast } from 'sonner';
import { ChevronDown } from 'lucide-react';

import { apiList, apiPost, apiPatch, ApiError } from '@/lib/api';
import type { Post, Category, Tag } from '@/lib/types';
import { slugify } from '@/lib/slugify';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover';
import { Checkbox } from '@/components/ui/checkbox';
import { MediaPicker } from '@/components/media-picker';

import { PostActions } from './post-actions';

const UNCATEGORIZED = '__uncategorized__';

const schema = z.object({
  title: z.string().min(1, 'Required').max(300),
  slug: z
    .string()
    .regex(/^[a-z0-9]+(-[a-z0-9]+)*$/, 'lowercase, hyphenated')
    .max(200)
    .optional()
    .or(z.literal('')),
  excerpt: z.string().max(1000).optional().or(z.literal('')),
  content: z.string().min(1, 'Required'),
  status: z.enum(['draft', 'published', 'archived']),
  thumbnail_id: z.string().uuid().nullable().optional(),
  meta_title: z.string().max(200).optional().or(z.literal('')),
  meta_description: z.string().max(500).optional().or(z.literal('')),
  category_ids: z.array(z.string().uuid()),
  tag_ids: z.array(z.string().uuid()),
});

export type PostFormValues = z.infer<typeof schema>;

interface PostFormProps {
  initial?: Post;
}

function defaultValues(post?: Post): PostFormValues {
  return {
    title: post?.title ?? '',
    slug: post?.slug ?? '',
    excerpt: post?.excerpt ?? '',
    content: post?.content ?? '',
    status: post?.status ?? 'draft',
    thumbnail_id:
      (post as (Post & { thumbnail_id?: string | null }) | undefined)
        ?.thumbnail_id ?? null,
    meta_title: '',
    meta_description: '',
    category_ids: post?.category ? [post.category.id] : [],
    tag_ids: post?.tags?.map((t) => t.id) ?? [],
  };
}

export function PostForm({ initial }: PostFormProps) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const isEdit = Boolean(initial);

  const {
    register,
    handleSubmit,
    control,
    watch,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm<PostFormValues>({
    resolver: zodResolver(schema),
    defaultValues: defaultValues(initial),
  });

  const titleValue = watch('title');
  const slugValue = watch('slug');
  const [slugTouched, setSlugTouched] = useState(Boolean(initial?.slug));

  const handleTitleBlur = () => {
    if (!slugTouched && (!slugValue || slugValue === '')) {
      const generated = slugify(titleValue ?? '');
      if (generated) setValue('slug', generated, { shouldValidate: true });
    }
  };

  const categoriesQuery = useQuery({
    queryKey: ['categories', 'all'],
    queryFn: () => apiList<Category>('/v1/admin/categories?perPage=100'),
  });
  const tagsQuery = useQuery({
    queryKey: ['tags', 'all'],
    queryFn: () => apiList<Tag>('/v1/admin/tags?perPage=200'),
  });

  const categories = categoriesQuery.data?.data ?? [];
  const tags = tagsQuery.data?.data ?? [];

  const createMutation = useMutation({
    mutationFn: async (values: PostFormValues) =>
      apiPost<Post>('/v1/admin/posts', toApiBody(values)),
    onSuccess: (post) => {
      toast.success('Post created');
      queryClient.invalidateQueries({ queryKey: ['posts'] });
      router.push(`/admin/posts/${post.id}`);
    },
    onError: handleApiError,
  });

  const updateMutation = useMutation({
    mutationFn: async (values: PostFormValues) =>
      apiPatch<Post>(`/v1/admin/posts/${initial!.id}`, toApiBody(values)),
    onSuccess: (post) => {
      toast.success('Post saved');
      queryClient.invalidateQueries({ queryKey: ['posts'] });
      queryClient.invalidateQueries({ queryKey: ['post', post.id] });
    },
    onError: handleApiError,
  });

  const onSubmit = handleSubmit((values) => {
    if (isEdit) updateMutation.mutate(values);
    else createMutation.mutate(values);
  });

  const submitting =
    isSubmitting || createMutation.isPending || updateMutation.isPending;

  return (
    <form onSubmit={onSubmit} className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">
          {isEdit ? 'Edit post' : 'New post'}
        </h1>
        <div className="flex items-center gap-2">
          {isEdit && initial ? (
            <PostActions post={initial} />
          ) : null}
          <Button type="submit" disabled={submitting}>
            {submitting ? 'Saving…' : isEdit ? 'Save' : 'Create'}
          </Button>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-6 md:grid-cols-3">
        <div className="flex flex-col gap-4 md:col-span-2">
          <FormField label="Title" error={errors.title?.message}>
            <Input
              {...register('title', {
                onBlur: handleTitleBlur,
              })}
              placeholder="Post title"
            />
          </FormField>

          <FormField
            label="Slug"
            hint="lowercase, hyphenated"
            error={errors.slug?.message}
          >
            <Input
              {...register('slug', {
                onChange: () => setSlugTouched(true),
              })}
              placeholder="my-post-slug"
            />
          </FormField>

          <FormField label="Excerpt" error={errors.excerpt?.message}>
            <Textarea
              {...register('excerpt')}
              rows={3}
              placeholder="Short summary"
            />
          </FormField>

          <FormField
            label="Content"
            hint="Markdown supported"
            error={errors.content?.message}
          >
            <Textarea
              {...register('content')}
              rows={15}
              placeholder="Write your post…"
              className="font-mono"
            />
          </FormField>
        </div>

        <div className="flex flex-col gap-4">
          <FormField label="Status" error={errors.status?.message}>
            <Controller
              control={control}
              name="status"
              render={({ field }) => (
                <Select value={field.value} onValueChange={field.onChange}>
                  <SelectTrigger className="w-full">
                    <SelectValue placeholder="Status" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="draft">Draft</SelectItem>
                    <SelectItem value="published">Published</SelectItem>
                    <SelectItem value="archived">Archived</SelectItem>
                  </SelectContent>
                </Select>
              )}
            />
          </FormField>

          <FormField
            label="Thumbnail"
            error={errors.thumbnail_id?.message as string | undefined}
          >
            <Controller
              control={control}
              name="thumbnail_id"
              render={({ field }) => (
                <MediaPicker
                  value={field.value ?? null}
                  onChange={(id) => field.onChange(id)}
                />
              )}
            />
          </FormField>

          <FormField label="Category">
            {categoriesQuery.isLoading ? (
              <Skeleton className="h-8 w-full" />
            ) : categories.length === 0 ? (
              <p className="text-sm text-muted-foreground">
                Create some categories first.
              </p>
            ) : (
              <Controller
                control={control}
                name="category_ids"
                render={({ field }) => {
                  const current = field.value[0] ?? UNCATEGORIZED;
                  return (
                    <Select
                      value={current}
                      onValueChange={(v) =>
                        field.onChange(v === UNCATEGORIZED ? [] : [v])
                      }
                    >
                      <SelectTrigger className="w-full">
                        <SelectValue placeholder="Select category" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value={UNCATEGORIZED}>
                          Uncategorized
                        </SelectItem>
                        {categories.map((c) => (
                          <SelectItem key={c.id} value={c.id}>
                            {c.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  );
                }}
              />
            )}
          </FormField>

          <FormField label="Tags">
            <Controller
              control={control}
              name="tag_ids"
              render={({ field }) => (
                <TagsMultiSelect
                  tags={tags}
                  loading={tagsQuery.isLoading}
                  value={field.value}
                  onChange={field.onChange}
                />
              )}
            />
          </FormField>

          <FormField
            label="Meta title"
            error={errors.meta_title?.message}
          >
            <Input {...register('meta_title')} placeholder="SEO title" />
          </FormField>

          <FormField
            label="Meta description"
            error={errors.meta_description?.message}
          >
            <Textarea
              {...register('meta_description')}
              rows={3}
              placeholder="SEO description"
            />
          </FormField>
        </div>
      </div>
    </form>
  );
}

function FormField({
  label,
  hint,
  error,
  children,
}: {
  label: string;
  hint?: string;
  error?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <Label>{label}</Label>
      {children}
      {hint && !error ? (
        <p className="text-xs text-muted-foreground">{hint}</p>
      ) : null}
      {error ? <p className="text-xs text-destructive">{error}</p> : null}
    </div>
  );
}

function TagsMultiSelect({
  tags,
  value,
  onChange,
  loading,
}: {
  tags: Tag[];
  value: string[];
  onChange: (next: string[]) => void;
  loading: boolean;
}) {
  const selectedSet = useMemo(() => new Set(value), [value]);
  const selectedTags = useMemo(
    () => tags.filter((t) => selectedSet.has(t.id)),
    [tags, selectedSet]
  );

  const toggle = (id: string) => {
    if (selectedSet.has(id)) onChange(value.filter((v) => v !== id));
    else onChange([...value, id]);
  };

  return (
    <div className="flex flex-col gap-2">
      <Popover>
        <PopoverTrigger
          render={
            <Button
              type="button"
              variant="outline"
              className="w-full justify-between"
            />
          }
        >
          <span>
            {value.length === 0
              ? 'Select tags'
              : `${value.length} selected`}
          </span>
          <ChevronDown className="size-4 text-muted-foreground" />
        </PopoverTrigger>
        <PopoverContent align="start" className="w-72 max-h-72 overflow-auto">
          {loading ? (
            <Skeleton className="h-24 w-full" />
          ) : tags.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              No tags yet. Create some first.
            </p>
          ) : (
            <ul className="flex flex-col gap-1">
              {tags.map((tag) => {
                const checked = selectedSet.has(tag.id);
                return (
                  <li key={tag.id}>
                    <label className="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1 text-sm hover:bg-muted">
                      <Checkbox
                        checked={checked}
                        onCheckedChange={() => toggle(tag.id)}
                      />
                      <span>{tag.name}</span>
                    </label>
                  </li>
                );
              })}
            </ul>
          )}
        </PopoverContent>
      </Popover>
      {selectedTags.length > 0 ? (
        <div className="flex flex-wrap gap-1">
          {selectedTags.map((t) => (
            <Badge key={t.id} variant="secondary">
              {t.name}
            </Badge>
          ))}
        </div>
      ) : null}
    </div>
  );
}

function toApiBody(values: PostFormValues) {
  return {
    title: values.title,
    slug: values.slug && values.slug !== '' ? values.slug : undefined,
    excerpt: values.excerpt && values.excerpt !== '' ? values.excerpt : null,
    content: values.content,
    status: values.status,
    thumbnail_id: values.thumbnail_id ?? null,
    meta_title:
      values.meta_title && values.meta_title !== ''
        ? values.meta_title
        : undefined,
    meta_description:
      values.meta_description && values.meta_description !== ''
        ? values.meta_description
        : undefined,
    category_ids: values.category_ids,
    tag_ids: values.tag_ids,
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

