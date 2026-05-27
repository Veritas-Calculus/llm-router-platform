import { useEffect, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { motion, AnimatePresence } from 'framer-motion';
import { useOnboardingStore } from '@/stores/useOnboardingStore';
import { useAuthStore } from '@/stores/authStore';
import {
  AdjustmentsHorizontalIcon,
  CpuChipIcon,
  KeyIcon,
  CreditCardIcon,
  PlayCircleIcon,
  ShieldCheckIcon,
  XMarkIcon,
} from '@heroicons/react/24/outline';

// M-01: Belt-and-braces auto-open guard.
//
// `hasCompletedTour` in the persisted store is the primary signal — once
// the user clicks X or Skip, it flips to true and the auto-open useEffect
// never fires again. But the audit caught a second failure mode: if the
// auto-open useEffect re-runs while the persist hydration is mid-flight,
// or if the store was wiped, the modal would re-trigger on every route
// change.
//
// This module-scoped ref records whether the auto-open useEffect has
// already fired *in this browser tab*. It is intentionally NOT persisted
// — a fresh reload should restart the gate from scratch, deferring to
// `hasCompletedTour` as before. The combination gives us:
//   1. Across reloads: hasCompletedTour blocks reopening.
//   2. Within a session: this ref blocks reopening even if some other
//      code path flips hasCompletedTour back to false transiently.
let autoOpenAttemptedInSession = false;

export default function OnboardingTour() {
  const { user } = useAuthStore();
  const { hasCompletedTour, isOpen, currentStep, nextStep, prevStep, completeTour, startTour, closeTour } = useOnboardingStore();
  const navigate = useNavigate();
  // Local mirror of the module flag so we don't have to read it inside
  // the effect's dependency closure. The ref preserves the value across
  // renders without retriggering the effect.
  const attemptedRef = useRef(autoOpenAttemptedInSession);

  // Auto-start for new users — only ever fires once per session, and
  // only when the long-lived `hasCompletedTour` says we haven't been
  // dismissed yet. After this useEffect fires (whether or not we opened
  // the modal), the session flag latches and any subsequent navigations
  // do nothing here.
  useEffect(() => {
    if (!user) return;
    if (attemptedRef.current) return;
    if (hasCompletedTour) {
      // Already dismissed; lock it in for this session too so the next
      // route change can't sneak in a re-trigger.
      attemptedRef.current = true;
      autoOpenAttemptedInSession = true;
      return;
    }
    if (isOpen) return;

    // Small delay to allow the main shell to mount and settle before
    // we paint over it.
    const timer = setTimeout(() => {
      // Double-check the persistent flag right before opening — the
      // user may have dismissed the tour in another tab between
      // useEffect arming the timer and the timer firing.
      if (!useOnboardingStore.getState().hasCompletedTour) {
        startTour();
      }
      attemptedRef.current = true;
      autoOpenAttemptedInSession = true;
    }, 1500);
    return () => clearTimeout(timer);
  }, [user, hasCompletedTour, isOpen, startTour]);

  // Unmount completely when closed — this is the load-bearing piece of
  // the M-01 fix on the DOM side. The previous implementation wrapped
  // the entire return in <AnimatePresence> and only relied on the
  // backdrop+motion.div animation states. The audit caught that the
  // fixed-position z-100 backdrop intercepted pointer events even when
  // `isOpen` was false in some race conditions. Returning null up here
  // guarantees there is no overlay element on the page when the modal
  // is closed.
  if (!isOpen) return null;

  const isAdmin = user?.role === 'admin';
  const steps = isAdmin
    ? [
        {
          title: 'Connect Providers',
          description: 'Start by adding upstream providers and encrypted provider API keys so the gateway has real models to route to.',
          icon: <CpuChipIcon className="w-12 h-12 text-apple-blue" />,
          actionLabel: 'Open Providers',
          action: () => {
            navigate('/admin/providers');
            nextStep();
          }
        },
        {
          title: 'Define Routing Policy',
          description: 'Review routing rules after providers are ready so fallback, cost, and latency behavior match your operating model.',
          icon: <AdjustmentsHorizontalIcon className="w-12 h-12 text-apple-blue" />,
          actionLabel: 'Open Routing Rules',
          action: () => {
            navigate('/admin/routing-rules');
            nextStep();
          }
        },
        {
          title: 'Harden System Settings',
          description: 'Finish by checking security, registration, SSO, payment, and feature gates before opening the platform to users.',
          icon: <ShieldCheckIcon className="w-12 h-12 text-apple-blue" />,
          actionLabel: 'Open System Settings',
          action: () => {
            navigate('/admin/settings');
            completeTour();
          }
        }
      ]
    : [
        {
          title: 'Welcome to LLM Router Platform',
          description: 'Create an API key to authenticate requests from your application.',
          icon: <KeyIcon className="w-12 h-12 text-apple-blue" />,
          actionLabel: 'Go to API Keys',
          action: () => {
            navigate('/api-keys');
            nextStep();
          }
        },
        {
          title: 'Check Your Balance',
          description: 'Review your plan, remaining balance, and billing options before sending production traffic.',
          icon: <CreditCardIcon className="w-12 h-12 text-apple-blue" />,
          actionLabel: 'View Subscription & Billing',
          action: () => {
            navigate('/subscription');
            nextStep();
          }
        },
        {
          title: 'Try the Playground',
          description: 'Open the Playground to test routed model calls before wiring the API into your app.',
          icon: <PlayCircleIcon className="w-12 h-12 text-apple-blue" />,
          actionLabel: 'Open Playground',
          action: () => {
            navigate('/playground');
            completeTour();
          }
        }
      ];

  const step = steps[currentStep];

  return (
    <AnimatePresence>
      <motion.div
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        exit={{ opacity: 0 }}
        // Backdrop layer. Clicking the backdrop dismisses the tour the
        // same way the X does — persistently — which matches user
        // expectation for a non-blocking welcome modal. e.target ===
        // e.currentTarget is the standard "only fire when click landed
        // on the backdrop, not on a child" guard; without it, clicking
        // anywhere inside the modal box would also dismiss.
        onClick={(e) => {
          if (e.target === e.currentTarget) closeTour();
        }}
        className="fixed inset-0 z-[100] flex items-center justify-center bg-black/40 backdrop-blur-sm p-4"
      >
        <motion.div
          role="dialog"
          aria-modal="true"
          aria-labelledby="onboarding-title"
          initial={{ scale: 0.95, opacity: 0, y: 10 }}
          animate={{ scale: 1, opacity: 1, y: 0 }}
          exit={{ scale: 0.95, opacity: 0, y: 10 }}
          className="bg-white rounded-2xl shadow-apple-2xl max-w-md w-full overflow-hidden"
        >
          <div className="relative p-6 sm:p-8">
            <button
              onClick={closeTour}
              aria-label="Close onboarding tour"
              className="absolute top-4 right-4 p-2 text-apple-gray-400 hover:text-apple-gray-600 bg-apple-gray-50 hover:bg-apple-gray-100 rounded-full transition-colors"
            >
              <XMarkIcon className="w-5 h-5" />
            </button>

            <div className="flex flex-col items-center text-center mt-2">
              <div className="w-20 h-20 bg-blue-50 rounded-full flex items-center justify-center mb-6">
                {step.icon}
              </div>

              <h2 id="onboarding-title" className="text-2xl font-semibold text-apple-gray-900 mb-2">
                {step.title}
              </h2>

              <p className="text-apple-gray-500 mb-8 leading-relaxed">
                {step.description}
              </p>

              <div className="flex flex-col w-full gap-3">
                <button
                  onClick={step.action}
                  className="btn btn-primary w-full justify-center py-2.5 text-base"
                >
                  {step.actionLabel}
                </button>
                {currentStep > 0 && (
                  <button
                    onClick={prevStep}
                    className="text-apple-gray-500 hover:text-apple-gray-700 text-sm font-medium py-2"
                  >
                    Back to previous step
                  </button>
                )}
                <div className="flex gap-2 justify-center mt-4">
                  {steps.map((_, idx) => (
                    <div
                      key={idx}
                      className={`h-1.5 rounded-full transition-all duration-300 ${
                        idx === currentStep ? 'w-6 bg-apple-blue' : 'w-2 bg-apple-gray-200'
                      }`}
                    />
                  ))}
                </div>
              </div>
            </div>
            <div className="mt-8 text-center border-t border-apple-gray-100 pt-4">
              <button
                onClick={completeTour}
                className="text-apple-gray-400 hover:text-apple-gray-600 text-sm hover:underline"
              >
                Skip tour
              </button>
            </div>
          </div>
        </motion.div>
      </motion.div>
    </AnimatePresence>
  );
}
