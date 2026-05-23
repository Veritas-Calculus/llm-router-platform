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
 */
export function useAuthHydrated(): boolean {
  const [hydrated, setHydrated] = useState(() => useAuthStore.persist.hasHydrated());
  useEffect(() => {
    if (hydrated) return;
    const unsub = useAuthStore.persist.onFinishHydration(() => setHydrated(true));
    // Guard against the listener being registered after hydration finished.
    if (useAuthStore.persist.hasHydrated()) setHydrated(true);
    return () => unsub();
  }, [hydrated]);
  return hydrated;
}
