import { create } from 'zustand';
import { persist, createJSONStorage } from 'zustand/middleware';
import { User } from '@/lib/types';

interface AuthState {
  token: string | null;
  user: User | null;
  isAuthenticated: boolean;
  isAdmin: boolean;
  adminView: boolean;
  selectedOrgId: string | null;
  setAuth: (token: string, user: User) => void;
  logout: () => void;
  updateUser: (user: User) => void;
  toggleAdminView: () => void;
  setSelectedOrgId: (orgId: string) => void;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      token: null,
      user: null,
      isAuthenticated: false,
      isAdmin: false,
      adminView: false,
      selectedOrgId: null,
      setAuth: (token: string, user: User) =>
        set({
          token,
          user,
          isAuthenticated: true,
          isAdmin: user.role === 'admin',
          adminView: user.role === 'admin',
        }),
      logout: () =>
        set({
          token: null,
          user: null,
          isAuthenticated: false,
          isAdmin: false,
          adminView: false,
          selectedOrgId: null,
        }),
      updateUser: (user: User) =>
        set({ user, isAdmin: user.role === 'admin' }),
      toggleAdminView: () =>
        set({ adminView: !get().adminView }),
      setSelectedOrgId: (orgId: string) =>
        set({ selectedOrgId: orgId }),
    }),
    {
      name: 'auth-storage',
      // Use sessionStorage instead of localStorage to limit XSS exposure:
      // - Scoped to the current tab (not shared across tabs)
      // - Cleared when the tab is closed
      // - Not accessible from other browser windows
      storage: createJSONStorage(() => sessionStorage),
      partialize: (state) => ({
        token: state.token,
        user: state.user,
        isAuthenticated: state.isAuthenticated,
        isAdmin: state.isAdmin,
        adminView: state.adminView,
        selectedOrgId: state.selectedOrgId,
      }),
    }
  )
);
