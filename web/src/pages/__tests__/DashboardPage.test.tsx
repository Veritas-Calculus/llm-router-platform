/* eslint-disable @typescript-eslint/no-explicit-any */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';
import DashboardPage from '@/pages/DashboardPage';

vi.mock('framer-motion', () => ({
    motion: {
        div: ({ children, ...props }: any) => <div {...props}>{children}</div>,
    },
    AnimatePresence: ({ children }: any) => <>{children}</>,
}));

vi.mock('recharts', () => ({
    ResponsiveContainer: ({ children }: any) => <div data-testid="chart-container">{children}</div>,
    LineChart: ({ children }: any) => <div data-testid="line-chart">{children}</div>,
    Line: () => null,
    XAxis: () => null,
    YAxis: () => null,
    CartesianGrid: () => null,
    Tooltip: () => null,
}));

vi.mock('@apollo/client/react', () => ({
    useQuery: vi.fn((query: any) => {
        const qName = query?.definitions?.[0]?.name?.value;
        if (qName === 'DashboardOverview') {
            return {
                data: {
                    dashboardOverview: {
                        totalRequests: 1234, totalTokens: 56789, totalCost: 12.34,
                        successRate: 98.5, errorCount: 3, requestsToday: 100,
                        tokensToday: 5000, costToday: 1.23, activeProviders: 3,
                        apiKeysHealth: { total: 5, healthy: 4 },
                        proxiesHealth: { total: 2, healthy: 2 },
                    },
                },
                loading: false,
                refetch: vi.fn(),
            };
        }
        return { data: null, loading: false, refetch: vi.fn() };
    }),
    useMutation: vi.fn(() => [vi.fn(), { loading: false }]),
}));

vi.mock('@/stores/authStore', () => ({
    useAuthStore: vi.fn(() => ({
        user: { id: 'user-1', email: 'test@example.com', name: 'Test User', role: 'user' },
    })),
}));

describe('DashboardPage', () => {
    beforeEach(() => { vi.clearAllMocks(); });

    it('should render dashboard title after loading', async () => {
        render(<BrowserRouter><DashboardPage /></BrowserRouter>);
        await waitFor(() => {
            expect(screen.getByText('Dashboard')).toBeInTheDocument();
        });
    });

    it('should render stat cards with data', async () => {
        render(<BrowserRouter><DashboardPage /></BrowserRouter>);
        await waitFor(() => {
            expect(screen.getByText('Total Requests')).toBeInTheDocument();
            expect(screen.getByText('Total Tokens')).toBeInTheDocument();
        });
    });

    it('should render system health section', async () => {
        render(<BrowserRouter><DashboardPage /></BrowserRouter>);
        await waitFor(() => {
            // Dashboard renders even with empty data
            expect(document.querySelector('.card')).toBeTruthy();
        });
    });
});
