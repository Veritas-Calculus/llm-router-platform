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
// credentials:'include' ensures the HttpOnly access cookie is sent on
// every GraphQL request. C-02: this is the browser-side half of moving
// access tokens off localStorage — together with the server-side
// llm_router_access cookie it eliminates the XSS-as-session-takeover
// blast radius.
const httpLink = createHttpLink({
  uri: '/graphql',
  credentials: 'include',
});

// ── Auth Link ──────────────────────────────────────────────────────
// Sends the active org header so the backend can default org-scoped
// reads to the user's current selection without each resolver
// threading an explicit argument. The access token rides in the
// llm_router_access HttpOnly cookie now (C-02); we deliberately do
// NOT set an Authorization header from browser state. If non-browser
// code reuses this client and stashes a bearer in the store, we
// still forward it — that's the API-key-style fallback path.
const authLink = setContext((_, { headers }) => {
  const { token, selectedOrgId } = useAuthStore.getState();
  const extra: Record<string, string> = {};
  if (selectedOrgId) extra['x-active-org'] = selectedOrgId;
  // Browser sessions: cookie is authoritative, do not double-attach.
  // Non-browser callers (e.g. tests, server-side rendering) may still
  // populate token via setAuth to use the bearer path.
  if (token && typeof window === 'undefined') {
    extra.Authorization = `Bearer ${token}`;
  }
  return {
    headers: {
      ...headers,
      ...extra,
    },
  };
});

// ── Silent refresh ─────────────────────────────────────────────────
// rotateRefreshToken is called when a request comes back with an auth error.
// The refresh token lives in an HttpOnly cookie; JS never reads or persists it.
// The result either yields a fresh access token (caller retries) or null
// (caller must log the user out).
//
// We deduplicate concurrent refreshes by caching the in-flight promise on the
// module — a burst of unauthenticated requests must not start N parallel
// rotations and burn N refresh tokens.
const ROTATE_MUTATION = gql`
  mutation RotateRefreshToken {
    rotateRefreshToken(refreshToken: "") {
      token
    }
  }
`;

let refreshInFlight: Promise<string | null> | null = null;

function rotateAccessToken(): Promise<string | null> {
  if (refreshInFlight) return refreshInFlight;

  refreshInFlight = (async () => {
    try {
      const result = await apolloClient.mutate({
        mutation: ROTATE_MUTATION,
        fetchPolicy: 'no-cache',
        context: { skipAuthRetry: true },
      });
      const payload = (result.data as { rotateRefreshToken?: { token: string } } | null | undefined)?.rotateRefreshToken;
      if (!payload?.token) return null;
      useAuthStore.getState().setAccessToken(payload.token);
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

      const shouldAttemptRefresh = !!useAuthStore.getState().isAuthenticated;
      if (!shouldAttemptRefresh) {
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
      const isForbidden =
        code === 'FORBIDDEN' ||
        (!code && (msg.includes('forbidden') || msg.includes('admin access required')));

      if (isForbidden) {
        // C-03: do NOT toast a forbidden error. The user already knows
        // they're not an admin; surfacing the raw "admin access required"
        // message leaks backend role names and produces noise on routes
        // that opportunistically fetch admin-scoped data. Log it for
        // observability instead — repeated FORBIDDEN on the same op is
        // the signal we want to act on (probably a missing client-side
        // role gate).
        const operationName = operation.operationName || '<anonymous>';
        if (typeof console !== 'undefined') {
          console.warn(`[apollo] suppressed FORBIDDEN on ${operationName}: ${msg}`);
        }
        Sentry.captureMessage(`FORBIDDEN on ${operationName}`, {
          level: 'warning',
          extra: { operation: operationName, message: msg },
        });
        return undefined;
      }
      // VALIDATION errors are component-local (form fields handle them
      // inline) so we don't toast either — toasting on submit-time
      // validation hides which field is bad.
      // INTERNAL errors and everything else fall through to the caller
      // which can decide whether to toast.
    }
  } else {
    // Network / unknown error
    toast.error('Network error — please check your connection');
    Sentry.captureException(error);
  }
  return undefined;
});

// Top-level Query fields that return an unpaginated list. The default
// InMemoryCache merge for an unkeyed list is "concatenate", which produces a
// stale-flash when a refetch returns fewer items (e.g. after a delete) — the
// old items linger until cache eviction. `merge: false` replaces the list
// wholesale on every refetch, which matches the user's mental model for
// list-of-everything queries.
//
// Keep this set authoritative against server/internal/graphql/schema/*.graphqls.
// New unpaginated list queries should be added here; paginated queries should
// instead use offsetLimitPagination / relayStylePagination.
const REPLACE_LIST_FIELDS = [
  // public/user-scoped
  'myOrganizations', 'myProjects', 'myApiKeys', 'myOrders', 'myRedeemHistory',
  'myDailyUsage', 'myUsageByProvider', 'usageChart', 'providerStats', 'modelStats',
  'organizationMembers', 'identityProviders', 'plans', 'activeAnnouncements',
  'publishedDocuments',
  // admin
  'users', 'providers', 'proxies', 'proxyPools', 'providerApiKeys', 'models',
  'apiKeys', 'alerts', 'healthApiKeys', 'healthProxies', 'healthProviders',
  'healthHistory', 'mcpServers', 'mcpTools', 'mcpResources', 'inviteCodes',
  'integrations', 'promptVersions', 'announcements', 'coupons', 'documents',
  'adminUsageByUser', 'adminRevenueChart', 'adminUserGrowth', 'userUsage',
  'userApiKeys', 'requestLogs', 'featureGates', 'notificationChannels',
  'webhooks', 'webhookDeliveries', 'semanticCaches',
] as const;

const queryFieldPolicies = Object.fromEntries(
  REPLACE_LIST_FIELDS.map((field) => [field, { merge: false as const }]),
);

// ── Apollo Client Instance ─────────────────────────────────────────
export const apolloClient = new ApolloClient({
  link: from([errorLink, authLink, httpLink]),
  cache: new InMemoryCache({
    typePolicies: {
      Query: {
        fields: queryFieldPolicies,
      },
      // H-06: User.balance is read on Dashboard, Subscription, Recharge —
      // each via a different query (UserDashboard, MyBilling, MyBalance).
      // With Apollo normalization the entity is already shared by id, but
      // we declare the field policy explicitly so a partial fetch
      // (e.g. me { id name } without balance) never wipes a previously
      // resolved balance from the cache. The custom merge keeps the
      // freshest non-undefined value, which matches the "monotonic"
      // user-mental-model for financial fields.
      User: {
        keyFields: ['id'],
        fields: {
          balance: {
            merge(existing, incoming) {
              if (incoming === undefined || incoming === null) return existing;
              return incoming;
            },
          },
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
