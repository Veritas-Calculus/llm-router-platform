/* eslint-disable @typescript-eslint/no-explicit-any */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import SubscriptionPage from '@/pages/SubscriptionPage';
import toast from 'react-hot-toast';

const mocks = vi.hoisted(() => ({
    changePlan: vi.fn(),
    createCheckoutSession: vi.fn(),
    redeemCode: vi.fn(),
    toastError: vi.fn(),
    toastSuccess: vi.fn(),
    refetchBalance: vi.fn(),
    refetchBilling: vi.fn(),
    refetchRedeemHistory: vi.fn(),
    updateUser: vi.fn(),
    redeemHistory: [] as any[],
    balance: 50,
}));

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
                    myOrders: [],
                },
                loading: false,
                refetch: mocks.refetchBilling,
            };
        }
        if (qName === 'MyRedeemHistory') {
            return {
                data: {
                    myRedeemHistory: mocks.redeemHistory,
                },
                loading: false,
                refetch: mocks.refetchRedeemHistory,
            };
        }
        return { data: null, loading: false };
    }),
    useLazyQuery: vi.fn(() => [mocks.refetchBalance, { loading: false }]),
    useMutation: vi.fn((mutation) => {
        const mName = mutation?.definitions?.[0]?.name?.value;
        if (mName === 'ChangePlan') return [mocks.changePlan, { loading: false }];
        if (mName === 'CreateCheckoutSession') return [mocks.createCheckoutSession, { loading: false }];
        if (mName === 'RedeemCode') return [mocks.redeemCode, { loading: false }];
        return [vi.fn().mockResolvedValue({ data: {} }), { loading: false }];
    }),
}));

vi.mock('react-hot-toast', () => ({
    default: {
        error: mocks.toastError,
        success: mocks.toastSuccess,
    },
}));

vi.mock('@/stores/authStore', () => ({
    useAuthStore: vi.fn(() => ({
        user: { id: 'u-1', email: 'test@test.com', role: 'user', balance: mocks.balance },
        updateUser: mocks.updateUser,
    })),
}));

vi.mock('@/components/RechargeModal', () => ({
    default: () => null,
}));

describe('SubscriptionPage', () => {
    beforeEach(() => {
        vi.clearAllMocks();
        mocks.changePlan.mockResolvedValue({ data: { changePlan: { id: 'sub-1' } } });
        mocks.createCheckoutSession.mockResolvedValue({ data: { createCheckoutSession: { url: 'https://checkout.test/session' } } });
        mocks.redeemCode.mockResolvedValue({ data: { redeemCode: { success: true } } });
        mocks.refetchBalance.mockImplementation(() => Promise.resolve({ data: { me: { id: 'u-1', balance: mocks.balance } } }));
        mocks.redeemHistory = [];
        mocks.refetchBilling.mockResolvedValue({ data: {} });
        mocks.refetchRedeemHistory.mockResolvedValue({ data: { myRedeemHistory: mocks.redeemHistory } });
        mocks.balance = 50;
    });

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

    it('shows payment-disabled error returned by checkout mutation', async () => {
        mocks.balance = 0;
        mocks.createCheckoutSession.mockResolvedValue({
            data: null,
            error: new Error('payments are currently disabled'),
        });

        render(<SubscriptionPage />);

        fireEvent.click(screen.getByRole('button', { name: /Upgrade Now|立即升级/ }));

        await waitFor(() => {
            expect(toast.error).toHaveBeenCalledWith(expect.stringMatching(/Payments are not enabled|支付通道未启用/));
        });
    });

    it('uses balance for paid plan when balance is sufficient', async () => {
        render(<SubscriptionPage />);

        fireEvent.click(screen.getByRole('button', { name: /Upgrade Now|立即升级/ }));

        await waitFor(() => {
            expect(mocks.changePlan).toHaveBeenCalledWith(expect.objectContaining({ variables: { planId: 'plan-pro' } }));
            expect(mocks.createCheckoutSession).not.toHaveBeenCalled();
            expect(toast.success).toHaveBeenCalled();
        });
    });

    it('refreshes displayed balance after redeeming a credit code', async () => {
        mocks.refetchBalance.mockResolvedValue({ data: { me: { id: 'u-1', balance: 75 } } });
        render(<SubscriptionPage />);

        fireEvent.change(screen.getByPlaceholderText(/Enter your redemption code|输入您的兑换码/), {
            target: { value: 'ABCD-1234-EFGH' },
        });
        fireEvent.click(screen.getByRole('button', { name: /^(Redeem|兑换)$/ }));

        await waitFor(() => {
            expect(mocks.redeemCode).toHaveBeenCalledWith({ variables: { code: 'ABCD-1234-EFGH' } });
            expect(mocks.updateUser).toHaveBeenCalledWith(expect.objectContaining({ balance: 75 }));
            expect(mocks.refetchRedeemHistory).toHaveBeenCalled();
        });
    });

    it('shows gift card recharge records in redeem history', async () => {
        mocks.redeemHistory = [
            {
                id: 'redeem-1',
                code: 'GIFT-2026-CASH',
                creditAmount: 10,
                planName: null,
                redeemedAt: '2026-05-09T04:00:00Z',
            },
        ];

        const { container } = render(<SubscriptionPage />);

        fireEvent.click(screen.getByRole('button', { name: /Redeem History|兑换记录/ }));

        await waitFor(() => {
            expect(screen.getByText('GIFT-2026-CASH')).toBeInTheDocument();
            expect(container.textContent).toContain('+$10.00');
        });
    });
});
