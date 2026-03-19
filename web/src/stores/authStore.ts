import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { User } from '@/lib/types';

interface AuthState {
  token: string | null;
  user: User | null;
  isAuthenticated: boolean;
  isAdmin: boolean;
  adminView: boolean;
  setAuth: (token: string, user: User) => void;
  logout: () => void;
  updateUser: (user: User) => void;
  toggleAdminView: () => void;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      token: null,
      user: null,
      isAuthenticated: false,
      isAdmin: false,
      adminView: false,
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
        }),
      updateUser: (user: User) =>
        set({ user, isAdmin: user.role === 'admin' }),
      toggleAdminView: () =>
        set({ adminView: !get().adminView }),
    }),
    {
      name: 'auth-storage',
      partialize: (state) => ({
        token: state.token,
        user: state.user,
        isAuthenticated: state.isAuthenticated,
        isAdmin: state.isAdmin,
        adminView: state.adminView,
      }),
    }
  )
);
