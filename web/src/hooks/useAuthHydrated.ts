import { useEffect, useState } from 'react';
import { useAuthStore } from '@/stores/authStore';

/**
 * useAuthHydrated returns true once Zustand's persist middleware has finished
 * rehydrating the auth store from localStorage. Components that read persisted
 * state (e.g. selectedOrgId) and would otherwise default it on mount must gate
 * the default-set effect on this hook — otherwise the effect runs against
 * pre-hydration `null` state, writes a default, and gets overwritten when
 * rehydration completes, producing a visible flicker plus a brief period where
 * other components see the wrong value.
 *
 * Tests sometimes stub useAuthStore without the persist wrapper; in that case
 * the helpers are missing and we return true (hydrated) by default — the
 * defaulting effects then behave like they did before persist was added.
 */
export function useAuthHydrated(): boolean {
  const persist = useAuthStore.persist as
    | { hasHydrated(): boolean; onFinishHydration(cb: () => void): () => void }
    | undefined;
  const [hydrated, setHydrated] = useState(() => (persist ? persist.hasHydrated() : true));
  useEffect(() => {
    if (hydrated || !persist) return;
    const unsub = persist.onFinishHydration(() => setHydrated(true));
    // Guard against the listener being registered after hydration finished.
    if (persist.hasHydrated()) setHydrated(true);
    return () => unsub();
  }, [hydrated, persist]);
  return hydrated;
}
