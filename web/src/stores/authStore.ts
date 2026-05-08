import { create } from 'zustand';
import { persist, createJSONStorage } from 'zustand/middleware';
import { User } from '@/lib/types';

const AUTH_STORAGE_KEY = 'auth-storage';

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
      name: AUTH_STORAGE_KEY,
      // Keep the login session browser-wide so a user can open multiple tabs
      // without re-entering credentials. Logout still clears every tab through
      // the storage event listener below.
      storage: createJSONStorage(() => localStorage),
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

if (typeof window !== 'undefined') {
  window.addEventListener('storage', (event) => {
    if (event.storageArea !== localStorage || event.key !== AUTH_STORAGE_KEY) {
      return;
    }

    void useAuthStore.persist.rehydrate();
  });
}
