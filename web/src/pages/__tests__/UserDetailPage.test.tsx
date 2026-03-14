import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import UserDetailPage from '@/pages/UserDetailPage';

vi.mock('framer-motion', () => ({
    motion: {
        div: ({ children, ...props }: any) => <div {...props}>{children}</div>,
    },
    AnimatePresence: ({ children }: any) => <>{children}</>,
}));

vi.mock('recharts', () => ({
    ResponsiveContainer: ({ children }: any) => <div>{children}</div>,
    BarChart: ({ children }: any) => <div>{children}</div>,
    Bar: () => null,
    LineChart: ({ children }: any) => <div>{children}</div>,
    Line: () => null,
    XAxis: () => null,
    YAxis: () => null,
    CartesianGrid: () => null,
    Tooltip: () => null,
}));

vi.mock('@heroicons/react/24/outline', () => ({
    ArrowLeftIcon: (props: any) => <svg data-testid="arrow-left" {...props} />,
    ShieldCheckIcon: (props: any) => <svg data-testid="shield-check" {...props} />,
    KeyIcon: (props: any) => <svg data-testid="key" {...props} />,
    ChartBarIcon: (props: any) => <svg data-testid="chart-bar" {...props} />,
    CurrencyDollarIcon: (props: any) => <svg data-testid="currency" {...props} />,
}));

vi.mock('@/lib/api', () => ({
    usersApi: {
        getById: vi.fn().mockResolvedValue({
            id: 'u1', email: 'admin@example.com', name: 'Admin User',
            role: 'admin', is_active: true, created_at: '2026-03-14T12:00:00Z',
            monthly_token_limit: 100000, monthly_budget_usd: 50.00,
            require_password_change: false, api_keys: 0,
            usage_month: { total_requests: 500, total_tokens: 25000, total_cost: 6.78 },
        }),
        getUsage: vi.fn().mockResolvedValue({ data: [] }),
        getApiKeys: vi.fn().mockResolvedValue({ data: [] }),
        toggle: vi.fn(),
        updateRole: vi.fn(),
        updateQuota: vi.fn(),
    },
}));

function renderWithRoute() {
    return render(
        <MemoryRouter initialEntries={['/users/u1']}>
            <Routes>
                <Route path="/users/:id" element={<UserDetailPage />} />
            </Routes>
        </MemoryRouter>
    );
}

describe('UserDetailPage', () => {
    beforeEach(() => { vi.clearAllMocks(); });

    it('should show loading spinner initially', () => {
        renderWithRoute();
        expect(document.querySelector('.animate-spin')).toBeTruthy();
    });

    it('should render user name after loading', async () => {
        renderWithRoute();
        await waitFor(() => {
            expect(screen.getByText('Admin User')).toBeInTheDocument();
        });
    });
});
