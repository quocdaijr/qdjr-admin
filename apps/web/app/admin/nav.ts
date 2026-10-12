import {
  Folder,
  GraduationCap,
  Image as ImageIcon,
  Inbox,
  LayoutDashboard,
  Newspaper,
  Settings,
  Tags,
  User as UserIcon,
  Users,
} from 'lucide-react';
import type { ComponentType, SVGProps } from 'react';

export interface NavItem {
  href: string;
  label: string;
  icon: ComponentType<SVGProps<SVGSVGElement>>;
}

export interface NavGroup {
  label: string;
  items: NavItem[];
}

// One source for the rail and the ⌘K palette.
export const NAV_GROUPS: NavGroup[] = [
  {
    label: 'Write',
    items: [
      { href: '/admin', label: 'Desk', icon: LayoutDashboard },
      { href: '/admin/posts', label: 'Posts', icon: Newspaper },
      { href: '/admin/media', label: 'Media', icon: ImageIcon },
    ],
  },
  {
    label: 'Organise',
    items: [
      { href: '/admin/categories', label: 'Categories', icon: Folder },
      { href: '/admin/tags', label: 'Tags', icon: Tags },
    ],
  },
  {
    label: 'Learn',
    items: [{ href: '/admin/lessons', label: 'Lessons', icon: GraduationCap }],
  },
  {
    label: 'Site',
    items: [
      { href: '/admin/profile', label: 'Profile', icon: UserIcon },
      { href: '/admin/settings', label: 'Settings', icon: Settings },
    ],
  },
  {
    label: 'People',
    items: [
      { href: '/admin/inbox', label: 'Inbox', icon: Inbox },
      { href: '/admin/users', label: 'Users', icon: Users },
    ],
  },
];

export const NAV_ITEMS: NavItem[] = NAV_GROUPS.flatMap((group) => group.items);

export function isActive(pathname: string, href: string): boolean {
  if (href === '/admin') return pathname === '/admin';
  return pathname === href || pathname.startsWith(`${href}/`);
}
