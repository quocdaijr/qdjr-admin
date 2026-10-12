// Admin session on the API's own JWTs (see apps/api/internal/session).
// Tokens live in localStorage like pate-shop; the admin is its own origin and
// loads no third-party scripts. Exports keep the shape the layout and user
// menu already use.

const BASE = process.env.NEXT_PUBLIC_API_URL!;
const ACCESS_KEY = 'qdjr.access_token';
const REFRESH_KEY = 'qdjr.refresh_token';
const USER_KEY = 'qdjr.user';
// Refresh a little before expiry so a request never leaves with a dead token.
const REFRESH_SKEW_MS = 30_000;

export interface AuthUser {
  id: string;
  email: string;
}

export interface Session {
  user: AuthUser;
}

export type AuthChangeEvent = 'SIGNED_IN' | 'SIGNED_OUT' | 'TOKEN_REFRESHED';
type Listener = (event: AuthChangeEvent, session: Session | null) => void;

interface TokenResponse {
  access_token: string;
  refresh_token: string;
  user: AuthUser;
}

const listeners = new Set<Listener>();
let refreshing: Promise<string | null> | null = null;

function read(key: string): string | null {
  try {
    return window.localStorage.getItem(key);
  } catch {
    return null;
  }
}

function write(key: string, value: string | null): void {
  try {
    if (value === null) window.localStorage.removeItem(key);
    else window.localStorage.setItem(key, value);
  } catch {
    // Storage blocked (private mode): the session just won't survive a reload.
  }
}

function storedUser(): AuthUser | null {
  const raw = read(USER_KEY);
  if (!raw) return null;
  try {
    const u = JSON.parse(raw) as AuthUser;
    return u && typeof u.id === 'string' && typeof u.email === 'string' ? u : null;
  } catch {
    return null;
  }
}

/** Seconds-since-epoch `exp` of a JWT, or 0 when unreadable. */
function expiryMs(token: string): number {
  try {
    const payload = JSON.parse(atob(token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')));
    return typeof payload.exp === 'number' ? payload.exp * 1000 : 0;
  } catch {
    return 0;
  }
}

function emit(event: AuthChangeEvent, session: Session | null): void {
  listeners.forEach((cb) => cb(event, session));
}

function store(t: TokenResponse): void {
  write(ACCESS_KEY, t.access_token);
  write(REFRESH_KEY, t.refresh_token);
  write(USER_KEY, JSON.stringify(t.user));
}

function clear(): void {
  write(ACCESS_KEY, null);
  write(REFRESH_KEY, null);
  write(USER_KEY, null);
}

async function postJSON(path: string, body: unknown): Promise<TokenResponse> {
  const res = await fetch(`${BASE}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
    cache: 'no-store',
  });
  const json = (await res.json().catch(() => null)) as
    | { data: TokenResponse | null; error: { message: string } | null }
    | null;
  if (!res.ok || !json?.data) {
    throw new Error(json?.error?.message ?? `Request failed (${res.status})`);
  }
  return json.data;
}

export async function signIn(email: string, password: string): Promise<void> {
  const t = await postJSON('/v1/auth/login', { email, password });
  store(t);
  emit('SIGNED_IN', { user: t.user });
}

export async function signOut(): Promise<void> {
  clear();
  emit('SIGNED_OUT', null);
}

/**
 * Exchanges the refresh token for a new pair. Concurrent callers share one
 * request. On failure the session is cleared and listeners hear SIGNED_OUT.
 */
export function refreshSession(): Promise<string | null> {
  if (refreshing) return refreshing;
  const refreshToken = read(REFRESH_KEY);
  if (!refreshToken) return Promise.resolve(null);
  refreshing = postJSON('/v1/auth/refresh', { refresh_token: refreshToken })
    .then((t) => {
      store(t);
      emit('TOKEN_REFRESHED', { user: t.user });
      return t.access_token;
    })
    .catch(() => {
      clear();
      emit('SIGNED_OUT', null);
      return null;
    })
    .finally(() => {
      refreshing = null;
    });
  return refreshing;
}

export async function getAccessToken(): Promise<string | null> {
  const token = read(ACCESS_KEY);
  if (token && expiryMs(token) - Date.now() > REFRESH_SKEW_MS) return token;
  return refreshSession();
}

export async function getCurrentUser(): Promise<AuthUser | null> {
  const user = storedUser();
  if (!user) return null;
  return (await getAccessToken()) ? storedUser() : null;
}

export function onAuthStateChange(cb: Listener): { unsubscribe: () => void } {
  listeners.add(cb);
  // Another tab signed out (or in): follow it.
  const onStorage = (e: StorageEvent) => {
    if (e.key !== REFRESH_KEY) return;
    const user = storedUser();
    cb(e.newValue && user ? 'SIGNED_IN' : 'SIGNED_OUT', e.newValue && user ? { user } : null);
  };
  window.addEventListener('storage', onStorage);
  return {
    unsubscribe: () => {
      listeners.delete(cb);
      window.removeEventListener('storage', onStorage);
    },
  };
}
