/* eslint-disable @typescript-eslint/no-explicit-any */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import ApiKeysPage from '@/pages/ApiKeysPage';

vi.mock('framer-motion', () => ({
    motion: {
        div: ({ children, ...props }: any) => <div {...props}>{children}</div>,
    },
    AnimatePresence: ({ children }: any) => <>{children}</>,
}));

vi.mock('@/lib/api', () => ({
    apiKeysApi: {
        list: vi.fn().mockResolvedValue({
            data: [
                { id: 'key-1', name: 'Test Key', key_prefix: 'sk-abc', is_active: true, rate_limit: 60, daily_limit: 1000, created_at: '2026-03-14T12:00:00Z' },
            ],
        }),
        create: vi.fn(),
        revoke: vi.fn(),
        delete: vi.fn(),
    },
}));

describe('ApiKeysPage', () => {
    beforeEach(() => { vi.clearAllMocks(); });

    it('should show loading spinner initially', () => {
        render(<ApiKeysPage />);
        expect(document.querySelector('.animate-spin')).toBeTruthy();
    });

    it('should render API Keys heading after loading', async () => {
        render(<ApiKeysPage />);
        await waitFor(() => {
            expect(screen.getByRole('heading', { name: 'API Keys' })).toBeInTheDocument();
        });
    });

    it('should render create key button', async () => {
        render(<ApiKeysPage />);
        await waitFor(() => {
            expect(screen.getByText('Create API Key')).toBeInTheDocument();
        });
    });
});
