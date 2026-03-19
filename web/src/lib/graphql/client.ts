import {
  ApolloClient,
  InMemoryCache,
  createHttpLink,
  from,
} from '@apollo/client';
import { setContext } from '@apollo/client/link/context';
import { onError } from '@apollo/client/link/error';
import { CombinedGraphQLErrors } from '@apollo/client/errors';
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

// ── Error Link (Apollo Client v4 API) ──────────────────────────────
const errorLink = onError(({ error }) => {
  if (CombinedGraphQLErrors.is(error)) {
    for (const gqlError of error.errors) {
      const msg = gqlError.message;
      if (msg.includes('unauthorized') || msg.includes('authentication required')) {
        useAuthStore.getState().logout();
        window.location.href = '/login';
        return;
      }
      if (msg.includes('forbidden') || msg.includes('admin access required')) {
        toast.error(msg);
        return;
      }
    }
  } else {
    // Network / unknown error
    toast.error('Network error — please check your connection');
    Sentry.captureException(error);
  }
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
