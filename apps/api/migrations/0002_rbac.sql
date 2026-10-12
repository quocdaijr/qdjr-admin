-- apps/api/migrations/0002_rbac.sql
-- +goose Up
-- Admin accounts. Owned by the API (bcrypt hashes, own JWTs); emails are
-- stored lower-cased so the unique index is case-insensitive in practice.
create table public.users (
    id              uuid primary key default gen_random_uuid(),
    email           text not null unique check (email = lower(email) and length(email) between 3 and 320),
    password_hash   text not null,
    last_sign_in_at timestamptz,
    created_at      timestamptz not null default now(),
    updated_at      timestamptz not null default now()
);

create table public.roles (
    id          smallint primary key generated always as identity,
    name        text not null unique,
    description text
);

create table public.user_roles (
    user_id     uuid primary key references public.users(id) on delete cascade,
    role_id     smallint not null references public.roles(id),
    assigned_at timestamptz not null default now()
);

create index user_roles_role_id_idx on public.user_roles (role_id);
