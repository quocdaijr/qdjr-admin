-- supabase/migrations/0001_init_enums.sql
create type public.post_status as enum ('draft', 'published', 'archived');
create type public.contact_status as enum ('new', 'read', 'replied', 'spam');
