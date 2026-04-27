// Post matches both public (GET /v1/posts) and admin (GET /v1/admin/posts)
// shapes. Public uses `category` (single, derived) and `thumbnail` (resolved URL);
// admin uses `categories[]` (full list) and `thumbnail_id` (raw FK). All
// admin-only fields are optional so the type covers both endpoints.
export interface Post {
  id: string;
  slug: string;
  title: string;
  excerpt: string | null;
  content: string;
  published_at: string | null;
  // Public-only:
  thumbnail?: { url: string; alt: string | null } | null;
  category?: { id: string; slug: string; name: string } | null;
  // Admin-only:
  categories?: { id: string; slug: string; name: string; description: string | null; created_at: string; updated_at: string }[];
  thumbnail_id?: string | null;
  og_image_id?: string | null;
  meta_title?: string | null;
  meta_description?: string | null;
  // Shared:
  tags: { id: string; slug: string; name: string }[];
  status?: 'draft' | 'published' | 'archived';
  created_by?: string | null;
  updated_by?: string | null;
  created_at: string;
  updated_at: string;
}

export interface Category {
  id: string;
  slug: string;
  name: string;
  description: string | null;
  created_at: string;
  updated_at: string;
}

export interface Tag {
  id: string;
  slug: string;
  name: string;
  description: string | null;
  created_at: string;
  updated_at: string;
}

export interface Media {
  id: string;
  filename: string;
  storage_path: string;
  url: string;
  mime_type: string;
  size: number;
  width: number | null;
  height: number | null;
  alt_text: string | null;
  uploaded_by: string | null;
  created_at: string;
}

// Profile shape returned by both public GET /v1/profile and admin
// GET /v1/admin/profile. The BE returns avatar_url (resolved public URL).
// Admin PATCH accepts avatar_id (media UUID) which is therefore optional and
// only used in update payloads, not responses.
export interface Profile {
  id: number;
  full_name: string | null;
  bio: string | null;
  avatar_url: string | null;
  avatar_id?: string | null;
  tagline: string | null;
  social_links: Record<string, string>;
  location: string | null;
  email: string | null;
  updated_at: string;
}

export interface SiteSettings {
  id: number;
  site_title: string;
  site_description: string | null;
  footer_text: string | null;
  contact_email: string | null;
  social_links: Record<string, string>;
  updated_at: string;
}

export interface ContactMessage {
  id: string;
  name: string;
  email: string;
  subject: string | null;
  body: string;
  ip: string | null;
  user_agent: string | null;
  status: 'new' | 'read' | 'replied' | 'spam';
  created_at: string;
}

export interface User {
  id: string;
  email: string;
  role: 'super_admin' | 'editor' | 'author' | null;
  last_sign_in_at: string | null;
  created_at: string;
  assigned_at: string | null;
}

export interface Me {
  user_id: string;
  role: string;
  permissions: string[];
}
