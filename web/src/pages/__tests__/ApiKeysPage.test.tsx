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

vi.mock('@apollo/client/react', () => ({
    useQuery: vi.fn((query) => {
        const qName = query?.definitions?.[0]?.name?.value;
        if (qName === 'MyOrganizations') {
            return { data: { myOrganizations: [{ id: 'org-1', name: 'Test Org', billingLimit: 100, createdAt: '2026-03-14T12:00:00Z' }] }, loading: false };
        }
        if (qName === 'MyProjects') {
            return { data: { myProjects: [{ id: 'proj-1', orgId: 'org-1', name: 'Test Project', description: '', quotaLimit: 100, createdAt: '2026-03-14T12:00:00Z' }] }, loading: false };
        }
        if (qName === 'MyApiKeys') {
            return {
                data: {
                    myApiKeys: [
                        { id: 'key-1', projectId: 'proj-1', channel: 'default', name: 'Test Key', keyPrefix: 'sk-abc', isActive: true, rateLimit: 60, dailyLimit: 1000, lastUsedAt: null, createdAt: '2026-03-14T12:00:00Z', expiresAt: null },
                    ]
                },
                loading: false,
                refetch: vi.fn()
            };
        }
        return { data: null, loading: false };
    }),
    useMutation: vi.fn(() => [vi.fn().mockResolvedValue({ data: {} }), { loading: false }]),
}));

describe('ApiKeysPage', () => {
    beforeEach(() => { vi.clearAllMocks(); });

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

    it('should render organization and project dropdowns', async () => {
        render(<ApiKeysPage />);
        await waitFor(() => {
            expect(screen.getByDisplayValue('Test Org')).toBeInTheDocument();
            expect(screen.getByDisplayValue('Test Project')).toBeInTheDocument();
        });
    });
});
