import { getAccessToken } from './auth';

const BASE = process.env.NEXT_PUBLIC_API_URL!;

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

export interface Envelope<T> {
  data: T | null;
  meta?: { page: number; perPage: number; total: number };
  error: { code: string; message: string } | null;
}

async function request<T>(path: string, init?: RequestInit): Promise<Envelope<T>> {
  const token = await getAccessToken();
  const headers = new Headers(init?.headers);
  if (!headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }
  if (token) headers.set('Authorization', `Bearer ${token}`);

  const res = await fetch(`${BASE}${path}`, { ...init, headers, cache: 'no-store' });
  if (res.status === 204) {
    return { data: null, error: null } as Envelope<T>;
  }
  const body = (await res.json()) as Envelope<T>;
  if (!res.ok || body.error) {
    const err = body.error ?? { code: 'INTERNAL', message: res.statusText };
    throw new ApiError(res.status, err.code, err.message);
  }
  return body;
}

export async function apiGet<T>(path: string): Promise<T> {
  const env = await request<T>(path);
  return env.data as T;
}

export async function apiList<T>(
  path: string
): Promise<{ data: T[]; meta: NonNullable<Envelope<T[]>['meta']> }> {
  const env = await request<T[]>(path);
  return {
    data: (env.data ?? []) as T[],
    meta: env.meta ?? { page: 1, perPage: 20, total: 0 },
  };
}

export async function apiPost<T>(path: string, body: unknown): Promise<T> {
  const env = await request<T>(path, {
    method: 'POST',
    body: JSON.stringify(body),
  });
  return env.data as T;
}

export async function apiPatch<T>(path: string, body: unknown): Promise<T> {
  const env = await request<T>(path, {
    method: 'PATCH',
    body: JSON.stringify(body),
  });
  return env.data as T;
}

export async function apiDelete(path: string): Promise<void> {
  await request<unknown>(path, { method: 'DELETE' });
}
