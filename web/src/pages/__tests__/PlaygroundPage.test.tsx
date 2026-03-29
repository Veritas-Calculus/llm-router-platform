/* eslint-disable @typescript-eslint/no-explicit-any */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, waitFor } from '@testing-library/react';
import PlaygroundPage from '@/pages/PlaygroundPage';

// Mock scrollIntoView which jsdom doesn't support
window.HTMLElement.prototype.scrollIntoView = vi.fn();

vi.mock('react-markdown', () => ({
    default: ({ children }: any) => <div>{children}</div>,
}));

vi.mock('remark-gfm', () => ({
    default: () => {},
}));

describe('PlaygroundPage', () => {
    beforeEach(() => { vi.clearAllMocks(); });

    it('should render without crash', async () => {
        const { container } = render(<PlaygroundPage />);
        await waitFor(() => {
            expect(container.textContent).toBeTruthy();
        });
    });

    it('should show settings sidebar with API key input', async () => {
        const { container } = render(<PlaygroundPage />);
        await waitFor(() => {
            expect(container.textContent).toContain('Settings');
            expect(container.querySelector('input[type="password"]')).toBeInTheDocument();
        });
    });

    it('should render model controls', async () => {
        const { container } = render(<PlaygroundPage />);
        await waitFor(() => {
            expect(container.textContent).toContain('Temperature');
            expect(container.textContent).toContain('Max Tokens');
        });
    });

    it('should show chat area with empty state', async () => {
        const { container } = render(<PlaygroundPage />);
        await waitFor(() => {
            expect(container.textContent).toContain('Send a message');
        });
    });
});
