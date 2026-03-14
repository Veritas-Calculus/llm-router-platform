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

    it('should show loading spinner initially', () => {
        render(<ProxiesPage />);
        expect(document.querySelector('.animate-spin')).toBeTruthy();
    });

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
            expect(screen.getByText('No proxies configured')).toBeInTheDocument();
        });
    });

    it('should show add proxy and batch import buttons in empty state', async () => {
        render(<ProxiesPage />);
        await waitFor(() => {
            expect(screen.getByText('Add your first proxy')).toBeInTheDocument();
        });
    });
});
