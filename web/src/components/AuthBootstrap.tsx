import { useEffect } from 'react';
import { useApolloClient } from '@apollo/client/react';
import { ME } from '@/lib/graphql/operations';
import { useAuthStore } from '@/stores/authStore';
import { useAuthHydrated } from '@/hooks/useAuthHydrated';

/**
 * AuthBootstrap rehydrates the auth session on app load.
 *
 * C-02: with the access token in an HttpOnly cookie we no longer have a
 * usable bearer in localStorage. On a page reload the persisted Zustand
 * state still says `isAuthenticated: true` (carried by `partialize`),
 * but only the cookie can prove the session is still valid. We call
 * the `me` query once on mount — if it succeeds, we refresh the user
 * payload in the store; if it fails with an auth error, the Apollo
 * errorLink already handles the redirect to /login.
 *
 * Renders nothing.
 */
export default function AuthBootstrap() {
  const hydrated = useAuthHydrated();
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  const setUserFromCookie = useAuthStore((s) => s.setUserFromCookie);
  const client = useApolloClient();

  useEffect(() => {
    if (!hydrated || !isAuthenticated) return;
    let cancelled = false;
    void client
      .query({ query: ME, fetchPolicy: 'network-only' })
      .then((res) => {
        if (cancelled) return;
        if (res.data?.me) {
          setUserFromCookie(res.data.me);
        }
      })
      .catch(() => {
        // The errorLink in client.ts handles UNAUTHENTICATED -> /login.
        // Nothing else to do here.
      });
    return () => {
      cancelled = true;
    };
    // Run once on mount; subsequent updates handled by individual queries.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [hydrated]);

  return null;
}
