/* eslint-disable @typescript-eslint/no-explicit-any */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, waitFor } from '@testing-library/react';
import WebhooksPage from '@/pages/WebhooksPage';

vi.mock('framer-motion', () => ({
    motion: {
        div: ({ children, ...props }: any) => <div {...props}>{children}</div>,
    },
    AnimatePresence: ({ children }: any) => <>{children}</>,
}));

vi.mock('@apollo/client/react', () => ({
    useQuery: vi.fn((query) => {
        const qName = query?.definitions?.[0]?.name?.value;
        if (qName === 'MyOrganizations') {
            return { data: { myOrganizations: [{ id: 'org-1', name: 'Test Org', billingLimit: 100, createdAt: '2026-03-14T12:00:00Z' }] }, loading: false };
        }
        if (qName === 'MyProjects') {
            return { data: { myProjects: [{ id: 'proj-1', orgId: 'org-1', name: 'Test Project', description: '', quotaLimit: 100, createdAt: '2026-03-14T12:00:00Z' }] }, loading: false };
        }
        return { data: null, loading: false, refetch: vi.fn() };
    }),
    useMutation: vi.fn(() => [vi.fn().mockResolvedValue({ data: {} }), { loading: false }]),
}));

describe('WebhooksPage', () => {
    beforeEach(() => { vi.clearAllMocks(); });

    it('should render without crash', async () => {
        const { container } = render(<WebhooksPage />);
        await waitFor(() => {
            expect(container.textContent).toBeTruthy();
        });
    });
});
