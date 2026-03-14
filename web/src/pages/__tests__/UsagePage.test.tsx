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

vi.mock('@/lib/api', () => ({
    usageApi: {
        getDailyStats: vi.fn().mockResolvedValue({ data: [] }),
        getRecords: vi.fn().mockResolvedValue({ data: [], total: 0 }),
        getMonthlyUsage: vi.fn().mockResolvedValue({
            total_requests: 500, total_tokens: 25000, total_cost: 6.78,
        }),
    },
}));

describe('UsagePage', () => {
    beforeEach(() => { vi.clearAllMocks(); });

    it('should show loading spinner initially', () => {
        render(<UsagePage />);
        expect(document.querySelector('.animate-spin')).toBeTruthy();
    });

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
