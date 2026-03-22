import { create } from 'zustand';
import { persist } from 'zustand/middleware';

interface OnboardingState {
  hasCompletedTour: boolean;
  currentStep: number;
  isOpen: boolean;
  completeTour: () => void;
  startTour: () => void;
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
      startTour: () => set({ isOpen: true, currentStep: 0 }),
      nextStep: () => set((state) => ({ currentStep: state.currentStep + 1 })),
      prevStep: () => set((state) => ({ currentStep: Math.max(0, state.currentStep - 1) })),
      setStep: (step: number) => set({ currentStep: step }),
      closeTour: () => set({ isOpen: false }),
    }),
    {
      name: 'onboarding-storage',
    }
  )
);
