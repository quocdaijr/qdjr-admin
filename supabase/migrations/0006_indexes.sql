-- supabase/migrations/0006_indexes.sql
create index posts_status_published_at_idx on public.posts (status, published_at desc);
create index posts_created_by_idx on public.posts (created_by);
create index post_tags_tag_id_idx on public.post_tags (tag_id);
create index post_categories_category_id_idx on public.post_categories (category_id);
create index contact_messages_status_created_at_idx on public.contact_messages (status, created_at desc);
create index media_uploaded_by_idx on public.media (uploaded_by);
