import { create } from 'zustand';
import { persist } from 'zustand/middleware';

// M-01: the onboarding tour modal must NOT re-open on every route change.
//
// Three orthogonal pieces of state cooperate to make that work:
//   - `hasCompletedTour` (persisted): the long-lived "user has dismissed
//     this once and never wants to see it auto-open again" bit. Lives in
//     localStorage so it survives reload / re-login.
//   - `isOpen` (in-memory): whether the modal is currently mounted. Set
//     by startTour / closeTour / completeTour. Resets to false on every
//     page reload.
//   - `restartTour` (action): the only way to forcibly open the modal
//     after the user has dismissed it. Used by the "Re-run onboarding
//     tour" link in Settings.
//
// The audit (M-01) found that the X button was calling closeTour, which
// only flipped `isOpen=false`. The OnboardingTour useEffect then re-fired
// on the next route change because `hasCompletedTour` was still false →
// modal reopens, overlay re-intercepts pointer events on the underlying
// CTA. The fix is: the X button (and "Skip tour") must persist the
// dismissal, not just toggle visibility.
interface OnboardingState {
  hasCompletedTour: boolean;
  currentStep: number;
  isOpen: boolean;
  completeTour: () => void;
  startTour: () => void;
  restartTour: () => void;
  nextStep: () => void;
  prevStep: () => void;
  setStep: (step: number) => void;
  closeTour: () => void;
}

export const useOnboardingStore = create<OnboardingState>()(
  persist(
    (set) => ({
      hasCompletedTour: false,
      currentStep: 0,
      isOpen: false,
      completeTour: () => set({ hasCompletedTour: true, isOpen: false }),
      // startTour is the auto-open path used by the OnboardingTour
      // useEffect on first login. It only takes effect when the modal
      // is not yet open — `hasCompletedTour` is guarded by the caller.
      startTour: () => set({ isOpen: true, currentStep: 0 }),
      // restartTour is the explicit "I want to re-take the tour" path
      // exposed in user-facing settings. It bypasses hasCompletedTour
      // because the user is explicitly asking for it; the modal will
      // re-set hasCompletedTour=true once they dismiss or finish.
      restartTour: () => set({ hasCompletedTour: false, isOpen: true, currentStep: 0 }),
      nextStep: () => set((state) => ({ currentStep: state.currentStep + 1 })),
      prevStep: () => set((state) => ({ currentStep: Math.max(0, state.currentStep - 1) })),
      setStep: (step: number) => set({ currentStep: step }),
      // closeTour is the user dismissing the modal (X button or backdrop
      // click). It persists hasCompletedTour=true so the auto-open guard
      // in OnboardingTour does not re-fire on the next route change.
      // This is the load-bearing fix for M-01: the previous version set
      // only isOpen=false, which left the auto-open useEffect armed.
      closeTour: () => set({ hasCompletedTour: true, isOpen: false }),
    }),
    {
      name: 'onboarding-storage',
    }
  )
);
