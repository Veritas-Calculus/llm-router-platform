/* eslint-disable @typescript-eslint/no-explicit-any */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import HealthPage from '@/pages/HealthPage';

vi.mock('framer-motion', () => ({
    motion: {
        div: ({ children, ...props }: any) => <div {...props}>{children}</div>,
    },
    AnimatePresence: ({ children }: any) => <>{children}</>,
}));

vi.mock('@apollo/client/react', () => ({
    useQuery: vi.fn(() => ({ data: null, loading: false, refetch: vi.fn() })),
    useMutation: vi.fn(() => [vi.fn(), { loading: false }]),
}));

vi.mock('@/lib/api', () => ({
    healthApi: {
        getApiKeysHealth: vi.fn().mockResolvedValue({ data: [] }),
        getProxiesHealth: vi.fn().mockResolvedValue({ data: [] }),
        getProvidersHealth: vi.fn().mockResolvedValue([]),
        checkApiKey: vi.fn(),
        checkProxy: vi.fn(),
        checkProvider: vi.fn(),
        checkAllProviders: vi.fn(),
    },
    alertsApi: {
        list: vi.fn().mockResolvedValue({ data: [] }),
        acknowledge: vi.fn(),
        resolve: vi.fn(),
    },
}));

describe('HealthPage', () => {
    beforeEach(() => { vi.clearAllMocks(); });

    it('should render health monitor title after loading', async () => {
        render(<HealthPage />);
        await waitFor(() => {
            expect(screen.getByText('Health Monitor')).toBeInTheDocument();
        });
    });

    it('should render tab navigation with providers, api-keys, proxies, alerts', async () => {
        render(<HealthPage />);
        await waitFor(() => {
            expect(screen.getByText('Providers')).toBeInTheDocument();
            expect(screen.getByText('API Keys')).toBeInTheDocument();
            expect(screen.getByText('Proxies')).toBeInTheDocument();
            expect(screen.getByText('Alerts')).toBeInTheDocument();
        });
    });

    it('should show Refresh button', async () => {
        render(<HealthPage />);
        await waitFor(() => {
            expect(screen.getByText('Refresh')).toBeInTheDocument();
        });
    });
});
