/* eslint-disable @typescript-eslint/no-explicit-any */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, waitFor } from '@testing-library/react';
import PromptRegistryPage from '@/pages/PromptRegistryPage';

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

describe('PromptRegistryPage', () => {
    beforeEach(() => { vi.clearAllMocks(); });

    it('should render without crash', async () => {
        const { container } = render(<PromptRegistryPage />);
        await waitFor(() => {
            expect(container.textContent).toBeTruthy();
        });
    });
});
