import { useQuery } from '@tanstack/react-query';

import { apiGet } from './api';
import type { Me } from './types';

export function useMe() {
  return useQuery({
    queryKey: ['me'],
    queryFn: () => apiGet<Me>('/v1/admin/me'),
    staleTime: 60_000,
  });
}
