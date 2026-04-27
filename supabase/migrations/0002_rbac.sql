-- supabase/migrations/0002_rbac.sql
create table public.roles (
    id          smallint primary key generated always as identity,
    name        text not null unique,
    description text
);

create table public.user_roles (
    user_id     uuid primary key references auth.users(id) on delete cascade,
    role_id     smallint not null references public.roles(id),
    assigned_at timestamptz not null default now()
);

create index user_roles_role_id_idx on public.user_roles (role_id);
