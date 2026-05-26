import { useEffect } from 'react';
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

export default function OnboardingTour() {
  const { user } = useAuthStore();
  const { hasCompletedTour, isOpen, currentStep, nextStep, prevStep, completeTour, startTour, closeTour } = useOnboardingStore();
  const navigate = useNavigate();

  // Auto-start for new users (e.g. within first 24 hours of account creation, or just if not completed)
  useEffect(() => {
    if (user && !hasCompletedTour && !isOpen) {
      // Small delay to allow main UI to settle
      const timer = setTimeout(() => {
        startTour();
      }, 1500);
      return () => clearTimeout(timer);
    }
  }, [user, hasCompletedTour, isOpen, startTour]);

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
