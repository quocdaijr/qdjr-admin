-- supabase/seed.sql
-- Idempotent seed: roles + default singleton rows.

insert into public.roles (name, description) values
    ('super_admin', 'Full system access including user management'),
    ('editor',      'Manage all content; cannot manage users'),
    ('author',      'Create and edit own posts; cannot publish or manage taxonomy')
on conflict (name) do nothing;

insert into public.profile (id) values (1) on conflict (id) do nothing;
insert into public.site_settings (id) values (1) on conflict (id) do nothing;
