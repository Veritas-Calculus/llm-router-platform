/* eslint-disable @typescript-eslint/no-explicit-any */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, waitFor } from '@testing-library/react';
import SubscriptionPage from '@/pages/SubscriptionPage';

vi.mock('framer-motion', () => ({
    motion: {
        div: ({ children, ...props }: any) => <div {...props}>{children}</div>,
        button: ({ children, ...props }: any) => <button {...props}>{children}</button>,
    },
    AnimatePresence: ({ children }: any) => <>{children}</>,
}));

vi.mock('@apollo/client/react', () => ({
    useQuery: vi.fn((query) => {
        const qName = query?.definitions?.[0]?.name?.value;
        if (qName === 'Plans') {
            return {
                data: {
                    plans: [
                        { id: 'plan-free', name: 'Free', description: 'Basic', priceMonth: 0, tokenLimit: 100000, rateLimit: 60, supportLevel: 'community', features: 'Basic routing', isActive: true },
                        { id: 'plan-pro', name: 'Pro', description: 'Pro', priceMonth: 29.99, tokenLimit: 1000000, rateLimit: 600, supportLevel: 'priority', features: 'Priority routing,Analytics', isActive: true },
                    ]
                },
                loading: false,
                refetch: vi.fn(),
            };
        }
        if (qName === 'MyBilling') {
            return {
                data: {
                    mySubscription: {
                        id: 'sub-1', planId: 'plan-free', planName: 'Free', status: 'active',
                        currentPeriodStart: '2026-03-01T00:00:00Z', currentPeriodEnd: '2026-04-01T00:00:00Z',
                        usedTokens: 50000, tokenLimit: 100000, quotaPercentage: 50, isQuotaExceeded: false,
                    },
                    myBudget: null,
                    invoices: [],
                },
                loading: false,
                refetch: vi.fn(),
            };
        }
        return { data: null, loading: false };
    }),
    useMutation: vi.fn(() => [vi.fn().mockResolvedValue({ data: {} }), { loading: false }]),
}));

vi.mock('@/stores/authStore', () => ({
    useAuthStore: vi.fn(() => ({ user: { id: 'u-1', email: 'test@test.com', role: 'user', balance: 50.0 } })),
}));

vi.mock('@/components/RechargeModal', () => ({
    default: () => null,
}));

describe('SubscriptionPage', () => {
    beforeEach(() => { vi.clearAllMocks(); });

    it('should render without crash and display balance', async () => {
        const { container } = render(<SubscriptionPage />);
        await waitFor(() => {
            expect(container.textContent).toBeTruthy();
            // Balance rendered from authStore user.balance
            expect(container.textContent).toContain('50.00');
        });
    });

    it('should display plan names', async () => {
        const { container } = render(<SubscriptionPage />);
        await waitFor(() => {
            expect(container.textContent).toContain('Free');
            expect(container.textContent).toContain('Pro');
        });
    });
});
