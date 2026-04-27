-- supabase/migrations/0004_singletons.sql
create table public.profile (
    id            smallint primary key default 1 check (id = 1),
    full_name     text,
    bio           text,
    avatar_id     uuid references public.media(id) on delete set null,
    tagline       text,
    social_links  jsonb not null default '{}'::jsonb,
    location      text,
    email         text,
    updated_by    uuid references auth.users(id) on delete set null,
    updated_at    timestamptz not null default now()
);

create table public.site_settings (
    id                smallint primary key default 1 check (id = 1),
    site_title        text not null default 'qdjr.me',
    site_description  text,
    footer_text       text,
    contact_email     text,
    social_links      jsonb not null default '{}'::jsonb,
    updated_by        uuid references auth.users(id) on delete set null,
    updated_at        timestamptz not null default now()
);
