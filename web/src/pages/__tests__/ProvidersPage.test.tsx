import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import ProvidersPage from '@/pages/ProvidersPage';

vi.mock('framer-motion', () => ({
    motion: {
        div: ({ children, ...props }: any) => <div {...props}>{children}</div>,
    },
    AnimatePresence: ({ children }: any) => <>{children}</>,
}));

vi.mock('@/lib/api', () => ({
    providersApi: {
        list: vi.fn().mockResolvedValue({
            data: [
                {
                    id: 'prov-1', name: 'openai', base_url: 'https://api.openai.com',
                    is_active: true, use_proxy: false, requires_api_key: true, default_proxy_id: null,
                },
                {
                    id: 'prov-2', name: 'anthropic', base_url: 'https://api.anthropic.com',
                    is_active: true, use_proxy: false, requires_api_key: true, default_proxy_id: null,
                },
            ],
        }),
        getApiKeys: vi.fn().mockResolvedValue({
            data: [
                { id: 'key-1', key_prefix: 'sk-abc', is_active: true, priority: 1, weight: 1, rate_limit: 60, usage_count: 100, total_cost: 1.23 },
            ],
        }),
        update: vi.fn(),
        toggle: vi.fn(),
        toggleProxy: vi.fn(),
        checkHealth: vi.fn(),
        createApiKey: vi.fn(),
        updateApiKey: vi.fn(),
        toggleApiKey: vi.fn(),
        deleteApiKey: vi.fn(),
    },
    proxiesApi: {
        list: vi.fn().mockResolvedValue({ data: [] }),
    },
}));

describe('ProvidersPage', () => {
    beforeEach(() => { vi.clearAllMocks(); });

    it('should show loading spinner initially', () => {
        render(<ProvidersPage />);
        expect(document.querySelector('.animate-spin')).toBeTruthy();
    });

    it('should render providers page after loading', async () => {
        render(<ProvidersPage />);
        await waitFor(() => {
            expect(screen.getByText('Manage LLM providers and their API keys')).toBeInTheDocument();
        });
    });

    it('should render provider list area', async () => {
        render(<ProvidersPage />);
        await waitFor(() => {
            expect(screen.getByText('Manage LLM providers and their API keys')).toBeInTheDocument();
        });
    });
});
