/* eslint-disable @typescript-eslint/no-explicit-any */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import ProxiesPage from '@/pages/ProxiesPage';

vi.mock('framer-motion', () => ({
    motion: {
        div: ({ children, ...props }: any) => <div {...props}>{children}</div>,
    },
    AnimatePresence: ({ children }: any) => <>{children}</>,
}));

vi.mock('@apollo/client/react', () => ({
    useQuery: vi.fn(() => ({ data: null, loading: false })),
    useMutation: vi.fn(() => [vi.fn(), { loading: false }]),
}));

vi.mock('@/lib/api', () => ({
    proxiesApi: {
        list: vi.fn().mockResolvedValue({ data: [] }),
        create: vi.fn(),
        update: vi.fn(),
        delete: vi.fn(),
        toggle: vi.fn(),
        test: vi.fn(),
        testAll: vi.fn(),
        batchCreate: vi.fn(),
    },
}));

describe('ProxiesPage', () => {
    beforeEach(() => { vi.clearAllMocks(); });

    it('should render proxies title after loading', async () => {
        render(<ProxiesPage />);
        await waitFor(() => {
            expect(screen.getByText('Proxies')).toBeInTheDocument();
            expect(screen.getByText('Manage proxy nodes for API requests')).toBeInTheDocument();
        });
    });

    it('should show empty state when no proxies', async () => {
        render(<ProxiesPage />);
        await waitFor(() => {
            expect(screen.getByText('No Proxies Configured')).toBeInTheDocument();
        });
    });

    it('should show add proxy button in empty state', async () => {
        render(<ProxiesPage />);
        await waitFor(() => {
            expect(screen.getByText('Add your first proxy')).toBeInTheDocument();
        });
    });
});
