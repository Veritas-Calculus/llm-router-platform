/* eslint-disable @typescript-eslint/no-explicit-any */
import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import DocsPage from '@/pages/DocsPage';

vi.mock('framer-motion', () => ({
    motion: new Proxy({}, {
        get: (_target: object, prop: string) =>
            ({ children, ...props }: any) => React.createElement(prop, props, children),
    }),
    AnimatePresence: ({ children }: any) => <>{children}</>,
}));

describe('DocsPage', () => {
    it('should render documentation title', () => {
        render(<DocsPage />);
        expect(screen.getByRole('heading', { name: 'Documentation' })).toBeInTheDocument();
    });

    it('should render navigation sidebar with multiple sections', () => {
        render(<DocsPage />);
        expect(screen.getAllByText('Quick Start').length).toBeGreaterThanOrEqual(1);
    });

    it('should render page subtitle', () => {
        render(<DocsPage />);
        expect(screen.getByText('Learn how to integrate and use the platform')).toBeInTheDocument();
    });
});
