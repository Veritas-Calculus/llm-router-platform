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

vi.mock('@/lib/api', () => ({
    healthApi: {
        getApiKeysHealth: vi.fn().mockResolvedValue({
            data: [
                { id: 'key-1', provider_name: 'OpenAI', key_prefix: 'sk-abc', is_active: true, response_time: 120, last_check: '2026-03-14T12:00:00Z' },
            ],
        }),
        getProxiesHealth: vi.fn().mockResolvedValue({
            data: [
                { id: 'proxy-1', url: 'http://proxy1.example.com', type: 'http', region: 'us-east', is_active: true, is_healthy: true, response_time: 50, success_rate: 0.95 },
            ],
        }),
        getProvidersHealth: vi.fn().mockResolvedValue([
            { id: 'prov-1', name: 'OpenAI', base_url: 'https://api.openai.com', is_active: true, is_healthy: true, use_proxy: false, response_time: 200, success_rate: 0.99, last_check: '2026-03-14T12:00:00Z' },
        ]),
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

    it('should show loading spinner initially', () => {
        render(<HealthPage />);
        expect(document.querySelector('.animate-spin')).toBeTruthy();
    });

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

    it('should show provider health data on default tab', async () => {
        render(<HealthPage />);
        await waitFor(() => {
            expect(screen.getByText('Provider Health')).toBeInTheDocument();
            expect(screen.getByText('OpenAI')).toBeInTheDocument();
        });
    });

    it('should show Refresh button', async () => {
        render(<HealthPage />);
        await waitFor(() => {
            expect(screen.getByText('Refresh')).toBeInTheDocument();
        });
    });
});
