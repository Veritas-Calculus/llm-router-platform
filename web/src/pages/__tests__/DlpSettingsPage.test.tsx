import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, waitFor } from '@testing-library/react';
import DlpSettingsPage from '@/pages/DlpSettingsPage';

vi.mock('@apollo/client/react', () => ({
    useQuery: vi.fn((query) => {
        const qName = query?.definitions?.[0]?.name?.value;
        if (qName === 'MyOrganizations') {
            return { data: { myOrganizations: [{ id: 'org-1', name: 'Test Org', billingLimit: 100, createdAt: '2026-03-14T12:00:00Z' }] }, loading: false };
        }
        if (qName === 'MyProjects') {
            return { data: { myProjects: [{ id: 'proj-1', orgId: 'org-1', name: 'Test Project', description: '', quotaLimit: 100, createdAt: '2026-03-14T12:00:00Z' }] }, loading: false };
        }
        if (qName === 'GetDLPConfig') {
            return {
                data: {
                    dlpConfig: {
                        enabled: true,
                        mode: 'redact',
                        rules: [
                            { id: 'r-1', type: 'pii', name: 'Email Detection', pattern: 'email', action: 'mask', enabled: true },
                        ],
                    }
                },
                loading: false,
                refetch: vi.fn(),
            };
        }
        return { data: null, loading: false };
    }),
    useMutation: vi.fn(() => [vi.fn().mockResolvedValue({ data: {} }), { loading: false }]),
    useLazyQuery: vi.fn(() => [vi.fn(), { data: null, loading: false }]),
}));

vi.mock('@/stores/authStore', () => ({
    useAuthStore: vi.fn(() => ({ user: { id: 'u-1', role: 'admin' } })),
}));

describe('DlpSettingsPage', () => {
    beforeEach(() => { vi.clearAllMocks(); });

    it('should render without crash', async () => {
        const { container } = render(<DlpSettingsPage />);
        await waitFor(() => {
            expect(container.textContent).toBeTruthy();
        });
    });
});
