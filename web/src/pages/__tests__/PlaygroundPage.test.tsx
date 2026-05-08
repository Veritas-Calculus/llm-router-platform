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

vi.mock('@apollo/client/react', () => ({
    useQuery: vi.fn(() => ({ data: null, loading: false })),
    useMutation: vi.fn(() => [vi.fn(), { loading: false }]),
}));

describe('PlaygroundPage', () => {
    beforeEach(() => { vi.clearAllMocks(); });

    it('should render without crash', async () => {
        const { container } = render(<PlaygroundPage />);
        await waitFor(() => {
            expect(container.textContent).toBeTruthy();
        });
    });

    it('should show settings sidebar with API key selector', async () => {
        const { container } = render(<PlaygroundPage />);
        await waitFor(() => {
            expect(container.textContent).toContain('Settings');
            expect(container.textContent).toContain('Paste a key manually');
            expect(container.querySelector('select')).toBeInTheDocument();
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
