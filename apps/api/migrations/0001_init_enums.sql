-- apps/api/migrations/0001_init_enums.sql
-- +goose Up
create type public.post_status as enum ('draft', 'published', 'archived');
create type public.contact_status as enum ('new', 'read', 'replied', 'spam');
