-- supabase/migrations/0005_contact.sql
create table public.contact_messages (
    id          uuid primary key default gen_random_uuid(),
    name        text not null,
    email       text not null,
    subject     text,
    body        text not null,
    ip          inet,
    user_agent  text,
    status      public.contact_status not null default 'new',
    created_at  timestamptz not null default now()
);
