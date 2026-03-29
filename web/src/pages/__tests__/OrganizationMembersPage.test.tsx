/* eslint-disable @typescript-eslint/no-explicit-any */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import OrganizationMembersPage from '@/pages/OrganizationMembersPage';

vi.mock('framer-motion', async () => {
    const React = await import('react');
    const mc = (tag: string) => React.forwardRef(({ children, ...p }: any, ref: any) =>
        React.createElement(tag, { ...p, ref }, children));
    return {
        motion: {
            div: mc('div'), span: mc('span'), button: mc('button'),
            tr: mc('tr'), td: mc('td'), li: mc('li'), ul: mc('ul'),
            section: mc('section'), p: mc('p'), h1: mc('h1'), h2: mc('h2'),
            h3: mc('h3'), form: mc('form'), a: mc('a'),
            nav: mc('nav'), header: mc('header'), footer: mc('footer'),
        },
        AnimatePresence: ({ children }: any) => children,
    };
});

vi.mock('@apollo/client/react', () => ({
    useQuery: vi.fn((query: any) => {
        const qName = query?.definitions?.[0]?.name?.value;
        if (qName === 'MyOrganizations') {
            return {
                data: { myOrganizations: [{ id: 'org-1', name: 'Test Org', billingLimit: 100, createdAt: '2026-03-14T12:00:00Z' }] },
                loading: false,
            };
        }
        return { data: null, loading: false, refetch: vi.fn() };
    }),
    useMutation: vi.fn(() => [vi.fn().mockResolvedValue({ data: {} }), { loading: false }]),
}));

vi.mock('@/stores/authStore', () => ({
    useAuthStore: vi.fn(() => ({ user: { id: 'u-1', email: 'admin@test.com', role: 'admin' } })),
}));

describe('OrganizationMembersPage', () => {
    beforeEach(() => { vi.clearAllMocks(); });

    it('should render without crash', async () => {
        const { container } = render(<OrganizationMembersPage />);
        await waitFor(() => {
            expect(container.textContent).toBeTruthy();
        });
    });

    it('should show organization selector', async () => {
        render(<OrganizationMembersPage />);
        await waitFor(() => {
            expect(screen.getByDisplayValue('Test Org')).toBeInTheDocument();
        });
    });
});
