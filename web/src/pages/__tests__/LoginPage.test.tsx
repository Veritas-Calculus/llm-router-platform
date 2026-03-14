import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';
import LoginPage from '@/pages/LoginPage';

// Mock the auth store
const mockLogin = vi.fn();
vi.mock('@/stores/authStore', () => ({
    useAuthStore: vi.fn(() => ({
        login: mockLogin,
        isAuthenticated: false,
    })),
}));

// Mock react-router navigate
const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
    const actual = await vi.importActual('react-router-dom');
    return {
        ...actual,
        useNavigate: () => mockNavigate,
    };
});

// Mock framer-motion
vi.mock('framer-motion', () => ({
    motion: {
        div: ({ children, ...props }: any) => <div {...props}>{children}</div>,
        form: ({ children, ...props }: any) => <form {...props}>{children}</form>,
    },
    AnimatePresence: ({ children }: any) => <>{children}</>,
}));

function renderLoginPage() {
    return render(
        <BrowserRouter>
            <LoginPage />
        </BrowserRouter>
    );
}

describe('LoginPage', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('should render the login form', () => {
        renderLoginPage();
        expect(screen.getByPlaceholderText(/email/i)).toBeInTheDocument();
        expect(screen.getByPlaceholderText(/password/i)).toBeInTheDocument();
    });

    it('should render sign in button', () => {
        renderLoginPage();
        const buttons = screen.getAllByRole('button', { name: /sign in/i });
        expect(buttons.length).toBeGreaterThanOrEqual(1);
    });

    it('should update input values on type', () => {
        renderLoginPage();
        const emailInput = screen.getByPlaceholderText(/email/i) as HTMLInputElement;
        const passwordInput = screen.getByPlaceholderText(/password/i) as HTMLInputElement;

        fireEvent.change(emailInput, { target: { value: 'test@example.com' } });
        fireEvent.change(passwordInput, { target: { value: 'password123' } });

        expect(emailInput.value).toBe('test@example.com');
        expect(passwordInput.value).toBe('password123');
    });

    it('should render the LLM Router Platform branding', () => {
        renderLoginPage();
        expect(screen.getByText('LLM Router Platform')).toBeInTheDocument();
    });
});
