/* eslint-disable @typescript-eslint/no-explicit-any */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, waitFor } from '@testing-library/react';
import RoutingRulesPage from '@/pages/RoutingRulesPage';

vi.mock('framer-motion', () => ({
    motion: {
        div: ({ children, ...props }: any) => <div {...props}>{children}</div>,
    },
    AnimatePresence: ({ children }: any) => <>{children}</>,
}));

vi.mock('@apollo/client/react', () => ({
    useQuery: vi.fn(() => ({
        data: null,
        loading: false,
        refetch: vi.fn(),
    })),
    useMutation: vi.fn(() => [vi.fn().mockResolvedValue({ data: {} }), { loading: false }]),
}));

describe('RoutingRulesPage', () => {
    beforeEach(() => { vi.clearAllMocks(); });

    it('should render without crash', async () => {
        const { container } = render(<RoutingRulesPage />);
        await waitFor(() => {
            expect(container.textContent).toBeTruthy();
        });
    });
});
