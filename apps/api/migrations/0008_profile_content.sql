-- apps/api/migrations/0008_profile_content.sql
-- +goose Up
-- Rich bilingual site copy rendered by apps/site on / and /about:
-- {"vi": ProfileContent, "en": ProfileContent}. The shape is owned by
-- apps/site/app/data/profile.ts; the API stores and serves it opaquely.
alter table public.profile
    add column content jsonb not null default '{}'::jsonb,
    add constraint profile_content_is_object check (jsonb_typeof(content) = 'object');
