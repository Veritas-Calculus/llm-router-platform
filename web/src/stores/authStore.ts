import { create } from 'zustand';
import { persist, createJSONStorage } from 'zustand/middleware';
import { User } from '@/lib/types';

const AUTH_STORAGE_KEY = 'auth-storage';

interface AuthState {
  // token is kept in memory for the runtime lifetime of the page but no
  // longer persisted to localStorage (C-02). A successful Login/Register
  // sets it in memory and the HttpOnly cookie carries it to subsequent
  // requests; on a hard refresh we rely on the cookie + the /me bootstrap
  // query to re-hydrate the user, and token returns to null until the
  // next mutation that returns one.
  token: string | null;
  user: User | null;
  isAuthenticated: boolean;
  isAdmin: boolean;
  adminView: boolean;
  selectedOrgId: string | null;
  setAuth: (token: string, user: User) => void;
  setAccessToken: (token: string) => void;
  /** Called on bootstrap when the cookie is still valid but the in-memory
   *  state lost its token (page reload). Sets only the user portion and
   *  marks the session authenticated; token stays null. */
  setUserFromCookie: (user: User) => void;
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
      setAuth: (token, user) =>
        set({
          token,
          user,
          isAuthenticated: true,
          isAdmin: user.role === 'admin',
          adminView: user.role === 'admin',
        }),
      setAccessToken: (token) =>
        set(() => ({
          token,
        })),
      setUserFromCookie: (user) =>
        set({
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
      // Persist only the parts of the auth state that DON'T grant access:
      // user profile, role flag, UI preferences. The access token is
      // intentionally excluded (C-02) — XSS reading localStorage no
      // longer yields a usable bearer. The session itself is carried
      // by the HttpOnly cookie + the /me bootstrap query.
      storage: createJSONStorage(() => localStorage),
      partialize: (state) => ({
        user: state.user,
        isAuthenticated: state.isAuthenticated,
        isAdmin: state.isAdmin,
        adminView: state.adminView,
        selectedOrgId: state.selectedOrgId,
      }),
      version: 3,
      migrate: (persistedState, _version) => {
        if (persistedState && typeof persistedState === 'object') {
          const s = persistedState as Record<string, unknown>;
          // v2 → v3: drop the legacy persisted access token. A user who
          // last logged in on an older build still has a valid HttpOnly
          // cookie on disk, so the bootstrap query rehydrates them
          // transparently. If the cookie is gone, the next protected
          // query fails and the errorLink redirects to /login.
          delete s.token;
          delete s.refreshToken;
        }
        return persistedState as AuthState;
      },
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
