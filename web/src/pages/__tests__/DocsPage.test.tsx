/* eslint-disable @typescript-eslint/no-explicit-any */
import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';
import DocsPage from '@/pages/DocsPage';

vi.mock('framer-motion', () => ({
    motion: new Proxy({}, {
        get: (_target: object, prop: string) =>
            ({ children, ...props }: any) => React.createElement(prop, props, children),
    }),
    AnimatePresence: ({ children }: any) => <>{children}</>,
}));

vi.mock('@apollo/client/react', () => ({
    useQuery: vi.fn(() => ({ data: null, loading: false })),
    useMutation: vi.fn(() => [vi.fn(), { loading: false }]),
}));

describe('DocsPage', () => {
    it('should render documentation title', () => {
        render(<BrowserRouter><DocsPage /></BrowserRouter>);
        expect(screen.getByRole('heading', { name: 'Documentation' })).toBeInTheDocument();
    });

    it('should render navigation sidebar with multiple sections', () => {
        render(<BrowserRouter><DocsPage /></BrowserRouter>);
        expect(screen.getAllByText('Quick Start').length).toBeGreaterThanOrEqual(1);
    });

    it('should render page subtitle', () => {
        render(<BrowserRouter><DocsPage /></BrowserRouter>);
        expect(screen.getByText('API reference, SDKs, and integration guides')).toBeInTheDocument();
    });
});
