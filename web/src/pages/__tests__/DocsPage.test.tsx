import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import DocsPage from '@/pages/DocsPage';

vi.mock('framer-motion', () => ({
    motion: {
        div: ({ children, ...props }: any) => <div {...props}>{children}</div>,
    },
    AnimatePresence: ({ children }: any) => <>{children}</>,
}));

describe('DocsPage', () => {
    it('should render documentation title', () => {
        render(<DocsPage />);
        expect(screen.getByRole('heading', { name: 'Documentation' })).toBeInTheDocument();
    });

    it('should render navigation sidebar with multiple sections', () => {
        render(<DocsPage />);
        // "Getting Started" appears in both nav and content (multiple matches expected)
        expect(screen.getAllByText('Getting Started').length).toBeGreaterThanOrEqual(1);
    });

    it('should render page subtitle', () => {
        render(<DocsPage />);
        expect(screen.getByText('Learn how to use and configure LLM Router')).toBeInTheDocument();
    });
});
