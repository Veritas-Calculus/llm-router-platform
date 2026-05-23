import { useEffect } from 'react';

/**
 * useVisibilityAwarePolling toggles a useQuery's polling based on
 * document.visibilityState. When the tab is hidden, polling is stopped;
 * when it becomes visible again, polling resumes at the original interval.
 *
 * Usage:
 *   const q = useQuery(MY_QUERY, { pollInterval: 30_000 });
 *   useVisibilityAwarePolling(q, 30_000);
 *
 * Passing pollInterval as the second argument matches the value supplied to
 * useQuery; we use it to know what interval to resume at.
 *
 * Apollo's useQuery already keeps polling when the tab is hidden — at the
 * browser's throttled setInterval rate. For a control plane with N polling
 * components (notification center, system status, dashboard, finance) that
 * was producing N×60×k requests/hour against a background tab. This hook
 * drops it to zero until the user comes back.
 *
 * The result type is duck-typed so the hook accepts both useQuery and
 * useLazyQuery results without coupling to a particular Apollo release's
 * exported types.
 */
type Pollable = {
  startPolling?: (ms: number) => void;
  stopPolling?: () => void;
};

export function useVisibilityAwarePolling(result: Pollable, pollInterval: number | undefined): void {
  useEffect(() => {
    if (!pollInterval || pollInterval <= 0) return;
    if (typeof document === 'undefined') return;
    if (typeof result.startPolling !== 'function' || typeof result.stopPolling !== 'function') {
      return;
    }
    const start = result.startPolling.bind(result);
    const stop = result.stopPolling.bind(result);

    const apply = () => {
      if (document.visibilityState === 'visible') {
        start(pollInterval);
      } else {
        stop();
      }
    };

    apply();
    document.addEventListener('visibilitychange', apply);
    return () => {
      document.removeEventListener('visibilitychange', apply);
    };
  }, [result, pollInterval]);
}
