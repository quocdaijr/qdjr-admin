-- supabase/migrations/0003_content.sql
create table public.media (
    id            uuid primary key default gen_random_uuid(),
    filename      text not null,
    storage_path  text not null unique,
    mime_type     text not null,
    size          bigint not null check (size >= 0),
    width         int,
    height        int,
    alt_text      text,
    uploaded_by   uuid references auth.users(id) on delete set null,
    created_at    timestamptz not null default now()
);

create table public.posts (
    id                uuid primary key default gen_random_uuid(),
    slug              text not null unique check (length(slug) <= 200 and slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    title             text not null,
    excerpt           text,
    content           text not null default '',
    status            public.post_status not null default 'draft',
    thumbnail_id      uuid references public.media(id) on delete set null,
    og_image_id       uuid references public.media(id) on delete set null,
    meta_title        text,
    meta_description  text,
    published_at      timestamptz,
    created_by        uuid references auth.users(id) on delete set null,
    updated_by        uuid references auth.users(id) on delete set null,
    created_at        timestamptz not null default now(),
    updated_at        timestamptz not null default now()
);

create table public.categories (
    id          uuid primary key default gen_random_uuid(),
    slug        text not null unique check (length(slug) <= 200 and slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    name        text not null,
    description text,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now()
);

create table public.tags (
    id          uuid primary key default gen_random_uuid(),
    slug        text not null unique check (length(slug) <= 200 and slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    name        text not null,
    description text,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now()
);

create table public.post_categories (
    post_id     uuid not null references public.posts(id) on delete cascade,
    category_id uuid not null references public.categories(id) on delete cascade,
    primary key (post_id, category_id)
);

create table public.post_tags (
    post_id uuid not null references public.posts(id) on delete cascade,
    tag_id  uuid not null references public.tags(id) on delete cascade,
    primary key (post_id, tag_id)
);
