import { describe, it, expect, beforeEach } from 'vitest';
import { useAuthStore } from '@/stores/authStore';

describe('authStore', () => {
    beforeEach(() => {
        // Reset store between tests
        useAuthStore.setState({
            token: null,
            user: null,
            isAuthenticated: false,
            isAdmin: false,
        });
    });

    it('should start with no authentication', () => {
        const state = useAuthStore.getState();
        expect(state.isAuthenticated).toBe(false);
        expect(state.token).toBeNull();
        expect(state.user).toBeNull();
        expect(state.isAdmin).toBe(false);
    });

    it('should set auth state on login', () => {
        const mockUser = {
            id: 'user-1',
            email: 'test@example.com',
            name: 'Test User',
            role: 'user',
            is_active: true,
            require_password_change: false,
            monthly_token_limit: 0,
            monthly_budget_usd: 0,
            created_at: new Date().toISOString(),
        };

        useAuthStore.getState().setAuth('test-token', mockUser);

        const state = useAuthStore.getState();
        expect(state.isAuthenticated).toBe(true);
        expect(state.token).toBe('test-token');
        expect(state.user?.email).toBe('test@example.com');
        expect(state.isAdmin).toBe(false);
    });

    it('should detect admin role', () => {
        const adminUser = {
            id: 'admin-1',
            email: 'admin@example.com',
            name: 'Admin',
            role: 'admin',
            is_active: true,
            require_password_change: false,
            monthly_token_limit: 0,
            monthly_budget_usd: 0,
            created_at: new Date().toISOString(),
        };

        useAuthStore.getState().setAuth('admin-token', adminUser);

        const state = useAuthStore.getState();
        expect(state.isAdmin).toBe(true);
    });

    it('should clear state on logout', () => {
        const mockUser = {
            id: 'user-1',
            email: 'test@example.com',
            name: 'Test User',
            role: 'user',
            is_active: true,
            require_password_change: false,
            monthly_token_limit: 0,
            monthly_budget_usd: 0,
            created_at: new Date().toISOString(),
        };

        useAuthStore.getState().setAuth('test-token', mockUser);
        useAuthStore.getState().logout();

        const state = useAuthStore.getState();
        expect(state.isAuthenticated).toBe(false);
        expect(state.token).toBeNull();
        expect(state.user).toBeNull();
        expect(state.isAdmin).toBe(false);
    });

    it('should update user without changing token', () => {
        const mockUser = {
            id: 'user-1',
            email: 'test@example.com',
            name: 'Test User',
            role: 'user',
            is_active: true,
            require_password_change: false,
            monthly_token_limit: 0,
            monthly_budget_usd: 0,
            created_at: new Date().toISOString(),
        };

        useAuthStore.getState().setAuth('test-token', mockUser);

        const updatedUser = { ...mockUser, name: 'Updated Name', role: 'admin' };
        useAuthStore.getState().updateUser(updatedUser);

        const state = useAuthStore.getState();
        expect(state.user?.name).toBe('Updated Name');
        expect(state.isAdmin).toBe(true);
        expect(state.token).toBe('test-token'); // Token unchanged
    });
});
