import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';
import SettingsPage from '@/pages/SettingsPage';

// Mock framer-motion
vi.mock('framer-motion', () => ({
    motion: {
        div: ({ children, ...props }: any) => <div {...props}>{children}</div>,
    },
    AnimatePresence: ({ children }: any) => <>{children}</>,
}));

// Mock auth store with admin user
vi.mock('@/stores/authStore', () => ({
    useAuthStore: vi.fn(() => ({
        user: {
            id: 'admin-1',
            email: 'admin@example.com',
            name: 'Admin User',
            role: 'admin',
            is_active: true,
            monthly_budget_usd: 100,
            monthly_token_limit: 1000000,
        },
        token: 'test-token',
        isAdmin: true,
    })),
}));

// Mock settings API
vi.mock('@/lib/api', () => ({
    settingsApi: {
        getProfile: vi.fn().mockResolvedValue({
            id: 'admin-1',
            email: 'admin@example.com',
            name: 'Admin User',
            role: 'admin',
        }),
    },
}));

function renderSettingsPage() {
    return render(
        <BrowserRouter>
            <SettingsPage />
        </BrowserRouter>
    );
}

describe('SettingsPage', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('should render profile section', async () => {
        renderSettingsPage();
        await waitFor(() => {
            const profileElements = screen.queryAllByText(/profile/i);
            expect(profileElements.length).toBeGreaterThan(0);
        });
    });

    it('should render danger zone section', async () => {
        renderSettingsPage();
        await waitFor(() => {
            expect(screen.getByText('Danger Zone')).toBeInTheDocument();
        });
    });

    it('should render delete account button', async () => {
        renderSettingsPage();
        await waitFor(() => {
            expect(screen.getByText('Delete Account')).toBeInTheDocument();
        });
    });
});
