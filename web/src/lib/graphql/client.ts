import {
  ApolloClient,
  InMemoryCache,
  createHttpLink,
  from,
  gql,
} from '@apollo/client';
import { setContext } from '@apollo/client/link/context';
import { onError } from '@apollo/client/link/error';
import { CombinedGraphQLErrors } from '@apollo/client/errors';
import { Observable } from 'rxjs';
import toast from 'react-hot-toast';
import * as Sentry from '@sentry/react';
import { useAuthStore } from '@/stores/authStore';

// ── HTTP Link ──────────────────────────────────────────────────────
const httpLink = createHttpLink({
  uri: '/graphql',
});

// ── Auth Link ──────────────────────────────────────────────────────
const authLink = setContext((_, { headers }) => {
  const token = useAuthStore.getState().token;
  return {
    headers: {
      ...headers,
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
  };
});

// ── Silent refresh ─────────────────────────────────────────────────
// rotateRefreshToken is called when a request comes back with an auth error
// and the store has a refresh token. The result either yields a fresh access
// token (caller retries) or null (caller must log the user out).
//
// We deduplicate concurrent refreshes by caching the in-flight promise on the
// module — a burst of unauthenticated requests must not start N parallel
// rotations and burn N refresh tokens.
const ROTATE_MUTATION = gql`
  mutation RotateRefreshToken($refreshToken: String!) {
    rotateRefreshToken(refreshToken: $refreshToken) {
      token
      refreshToken
    }
  }
`;

let refreshInFlight: Promise<string | null> | null = null;

function rotateAccessToken(): Promise<string | null> {
  if (refreshInFlight) return refreshInFlight;

  const stored = useAuthStore.getState().refreshToken;
  if (!stored) return Promise.resolve(null);

  refreshInFlight = (async () => {
    try {
      const result = await apolloClient.mutate({
        mutation: ROTATE_MUTATION,
        variables: { refreshToken: stored },
        // The auth link will skip the Authorization header because we'll have
        // cleared the token below... actually no, the access token is still
        // there. That's fine: rotateRefreshToken doesn't require a valid
        // access token; it validates the refresh token signature + iat
        // server-side.
        fetchPolicy: 'no-cache',
        context: { skipAuthRetry: true },
      });
      const payload = (result.data as { rotateRefreshToken?: { token: string; refreshToken: string | null } } | null | undefined)?.rotateRefreshToken;
      if (!payload?.token) return null;
      useAuthStore.getState().setAccessToken(payload.token, payload.refreshToken ?? null);
      return payload.token;
    } catch {
      return null;
    } finally {
      refreshInFlight = null;
    }
  })();
  return refreshInFlight;
}

// Stable error codes mirror the server side (handler.go classifyClientError).
// Branching on these is robust to message-text changes; the legacy substring
// fallback exists for older code paths that haven't been retrofitted yet.
type GqlErrorCode =
  | 'UNAUTHENTICATED'
  | 'FORBIDDEN'
  | 'RATE_LIMITED'
  | 'INVALID_CREDENTIALS'
  | 'NOT_FOUND'
  | 'INSUFFICIENT_BALANCE'
  | 'ACCOUNT_DISABLED'
  | 'VALIDATION'
  | 'INTERNAL';

function codeOf(e: { extensions?: Record<string, unknown> }): GqlErrorCode | undefined {
  const c = e.extensions?.code;
  return typeof c === 'string' ? (c as GqlErrorCode) : undefined;
}

function isAuthError(e: { extensions?: Record<string, unknown>; message: string }): boolean {
  if (codeOf(e) === 'UNAUTHENTICATED') return true;
  // Fallback for errors emitted before the server was retrofitted with
  // extensions.code (custom user-service errors, third-party middleware).
  return /unauthor|authentication required|token has been revoked|invalid token/i.test(e.message);
}

// ── Error Link (Apollo Client v4 API) ──────────────────────────────
const errorLink = onError(({ error, operation, forward }) => {
  if (CombinedGraphQLErrors.is(error)) {
    const hasAuthError = error.errors.some(isAuthError);
    if (hasAuthError) {
      // Avoid recursing if this *is* the refresh call.
      if (operation.getContext().skipAuthRetry) {
        return undefined;
      }

      const hasRefresh = !!useAuthStore.getState().refreshToken;
      if (!hasRefresh) {
        void apolloClient.clearStore().finally(() => {
          useAuthStore.getState().logout();
          if (typeof window !== 'undefined' && window.location.pathname !== '/login') {
            window.location.href = '/login';
          }
        });
        return undefined;
      }

      // Try to refresh once and retry the failing operation.
      return new Observable((observer) => {
        rotateAccessToken().then((newToken) => {
          if (!newToken) {
            void apolloClient.clearStore().finally(() => {
              useAuthStore.getState().logout();
              if (typeof window !== 'undefined' && window.location.pathname !== '/login') {
                window.location.href = '/login';
              }
              observer.error(error);
            });
            return;
          }
          // The auth link reads the latest token from the store on each
          // execution, so simply forwarding works.
          const sub = forward(operation).subscribe({
            next: observer.next.bind(observer),
            error: observer.error.bind(observer),
            complete: observer.complete.bind(observer),
          });
          return () => sub.unsubscribe();
        });
      });
    }

    for (const gqlError of error.errors) {
      const code = codeOf(gqlError);
      const msg = gqlError.message;
      if (code === 'FORBIDDEN' || (!code && (msg.includes('forbidden') || msg.includes('admin access required')))) {
        toast.error(msg);
        return undefined;
      }
    }
  } else {
    // Network / unknown error
    toast.error('Network error — please check your connection');
    Sentry.captureException(error);
  }
  return undefined;
});

// ── Apollo Client Instance ─────────────────────────────────────────
export const apolloClient = new ApolloClient({
  link: from([errorLink, authLink, httpLink]),
  cache: new InMemoryCache({
    typePolicies: {
      Query: {
        fields: {
          users: { merge: false },
          providers: { merge: false },
          proxies: { merge: false },
          alerts: { merge: false },
          apiKeys: { merge: false },
        },
      },
    },
  }),
  defaultOptions: {
    watchQuery: {
      fetchPolicy: 'cache-and-network',
      errorPolicy: 'all',
    },
    query: {
      errorPolicy: 'all',
    },
    mutate: {
      errorPolicy: 'all',
    },
  },
});
