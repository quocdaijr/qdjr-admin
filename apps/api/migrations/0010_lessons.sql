-- apps/api/migrations/0010_lessons.sql
-- +goose Up
-- Self-study tracker: a course is one external thing being studied, a lesson
-- one ordered unit of it. Progress is derived from lessons.status, never stored.

create type public.lesson_status as enum ('todo', 'in_progress', 'done');

create table public.courses (
    id          uuid primary key default gen_random_uuid(),
    title       text not null check (char_length(title) between 1 and 200),
    url         text not null default '',
    provider    text not null default '',
    notes       text not null default '',
    created_by  uuid references public.users(id) on delete set null,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now()
);

-- Lessons have no life outside their course: cascade is intentional here.
create table public.lessons (
    id           uuid primary key default gen_random_uuid(),
    course_id    uuid not null references public.courses(id) on delete cascade,
    title        text not null check (char_length(title) between 1 and 200),
    url          text not null default '',
    position     integer not null default 0,
    status       public.lesson_status not null default 'todo',
    notes        text not null default '',
    completed_at timestamptz,
    created_at   timestamptz not null default now(),
    updated_at   timestamptz not null default now()
);

create index lessons_course_position_idx on public.lessons (course_id, position);
