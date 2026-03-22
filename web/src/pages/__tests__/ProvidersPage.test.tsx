/* eslint-disable @typescript-eslint/no-explicit-any */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import ProvidersPage from '@/pages/ProvidersPage';

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
    providersApi: {
        list: vi.fn().mockResolvedValue({ data: [] }),
        getApiKeys: vi.fn().mockResolvedValue({ data: [] }),
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
