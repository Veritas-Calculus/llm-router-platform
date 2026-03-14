import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import DashboardPage from '@/pages/DashboardPage';

// Mock framer-motion to avoid animation issues in tests
vi.mock('framer-motion', () => ({
    motion: {
        div: ({ children, ...props }: any) => <div {...props}>{children}</div>,
    },
    AnimatePresence: ({ children }: any) => <>{children}</>,
}));

// Mock recharts to avoid canvas issues in JSDOM
vi.mock('recharts', () => ({
    ResponsiveContainer: ({ children }: any) => <div data-testid="chart-container">{children}</div>,
    LineChart: ({ children }: any) => <div data-testid="line-chart">{children}</div>,
    Line: () => null,
    XAxis: () => null,
    YAxis: () => null,
    CartesianGrid: () => null,
    Tooltip: () => null,
}));

// Mock the dashboard API — inline data to avoid hoisting issues
vi.mock('@/lib/api', () => ({
    dashboardApi: {
        getOverview: vi.fn().mockResolvedValue({
            total_requests: 1234,
            total_tokens: 56789,
            total_cost: 12.34,
            success_rate: 98.5,
            error_count: 3,
            requests_today: 100,
            tokens_today: 5000,
            cost_today: 1.23,
            active_providers: 3,
            api_keys: { total: 5, healthy: 4 },
            proxies: { total: 2, healthy: 2 },
        }),
        getUsageChart: vi.fn().mockResolvedValue({ data: [] }),
        getProviderStats: vi.fn().mockResolvedValue({ data: [] }),
        getModelStats: vi.fn().mockResolvedValue({ data: [] }),
    },
}));

vi.mock('@/stores/authStore', () => ({
    useAuthStore: vi.fn(() => ({
        user: {
            id: 'user-1',
            email: 'test@example.com',
            name: 'Test User',
            role: 'user',
            monthly_budget_usd: 0,
            monthly_token_limit: 0,
        },
    })),
}));

describe('DashboardPage', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('should show loading spinner initially', () => {
        render(<DashboardPage />);
        const spinner = document.querySelector('.animate-spin');
        expect(spinner).toBeTruthy();
    });

    it('should render dashboard title after loading', async () => {
        render(<DashboardPage />);
        await waitFor(() => {
            expect(screen.getByText('Dashboard')).toBeInTheDocument();
        });
    });

    it('should render stat cards with data', async () => {
        render(<DashboardPage />);
        await waitFor(() => {
            expect(screen.getByText('Total Requests')).toBeInTheDocument();
            expect(screen.getByText('Total Tokens')).toBeInTheDocument();
            expect(screen.getByText('Total Cost')).toBeInTheDocument();
            expect(screen.getByText('Success Rate')).toBeInTheDocument();
        });
    });

    it('should render system health section', async () => {
        render(<DashboardPage />);
        await waitFor(() => {
            expect(screen.getByText('Active Providers')).toBeInTheDocument();
            expect(screen.getByText('API Keys Health')).toBeInTheDocument();
            expect(screen.getByText('Proxies Health')).toBeInTheDocument();
        });
    });

    it('should render chart sections', async () => {
        render(<DashboardPage />);
        await waitFor(() => {
            expect(screen.getByText('Request Trend (7 Days)')).toBeInTheDocument();
            expect(screen.getByText('Cost Trend')).toBeInTheDocument();
            expect(screen.getByText('Token Usage Trend')).toBeInTheDocument();
        });
    });

    it('should show empty state for provider usage', async () => {
        render(<DashboardPage />);
        await waitFor(() => {
            expect(screen.getByText('No provider usage data yet')).toBeInTheDocument();
        });
    });
});
