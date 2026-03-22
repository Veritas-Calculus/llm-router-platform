/* eslint-disable @typescript-eslint/no-explicit-any */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import UsagePage from '@/pages/UsagePage';

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
    useQuery: vi.fn(() => ({
        data: {
            myUsageSummary: { totalRequests: 500, totalTokens: 25000, totalCost: 6.78, successRate: 99.2 },
            myDailyUsage: [],
            myRecentUsage: { data: [], total: 0 },
        },
        loading: false,
        refetch: vi.fn(),
    })),
    useMutation: vi.fn(() => [vi.fn(), { loading: false }]),
}));

describe('UsagePage', () => {
    beforeEach(() => { vi.clearAllMocks(); });

    it('should render usage page title after loading', async () => {
        render(<UsagePage />);
        await waitFor(() => {
            expect(screen.getByRole('heading', { name: 'Usage' })).toBeInTheDocument();
        });
    });

    it('should render monthly stats after loading', async () => {
        render(<UsagePage />);
        await waitFor(() => {
            expect(screen.getByText('Monthly Requests')).toBeInTheDocument();
        });
    });
});
