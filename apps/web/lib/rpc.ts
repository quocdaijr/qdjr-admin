// Connect client for the lessons service — the one admin API served over
// Connect/gRPC instead of REST (docs/adr/0001). Fetch-based, so it runs in the
// browser and in the OpenNext worker alike. Auth mirrors lib/api.ts: bearer on
// every call, one refresh-and-retry on Unauthenticated, sign out when the
// refresh fails.
import { Code, ConnectError, createClient, type Interceptor } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';

import { LessonsService } from '@/gen/lessons/v1/lessons_pb';

import { getAccessToken, refreshSession, signOut } from './auth';

const BASE = process.env.NEXT_PUBLIC_API_URL!;

const auth: Interceptor = (next) => async (req) => {
  const token = await getAccessToken();
  if (token) req.header.set('Authorization', `Bearer ${token}`);
  try {
    return await next(req);
  } catch (err) {
    if (!(err instanceof ConnectError) || err.code !== Code.Unauthenticated) throw err;
    const fresh = await refreshSession();
    if (!fresh) {
      await signOut();
      throw err;
    }
    // Unary only: the request message is a plain object, safe to resend.
    req.header.set('Authorization', `Bearer ${fresh}`);
    return next(req);
  }
};

const transport = createConnectTransport({ baseUrl: BASE, interceptors: [auth] });

export const lessonsClient = createClient(LessonsService, transport);

/** Toast text in the same "CODE: message" shape ApiError produces for REST. */
export function rpcErrorMessage(err: unknown, fallback = 'Request failed'): string {
  if (err instanceof ConnectError) return `${Code[err.code]}: ${err.rawMessage}`;
  if (err instanceof Error) return err.message;
  return fallback;
}
