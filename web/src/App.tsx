import { Routes, Route, Navigate } from 'react-router-dom';
import { useAuthStore } from '@/stores/authStore';
import Layout from '@/components/Layout';
import LoginPage from '@/pages/LoginPage';
import DashboardPage from '@/pages/DashboardPage';
import UsagePage from '@/pages/UsagePage';
import ApiKeysPage from '@/pages/ApiKeysPage';
import HealthPage from '@/pages/HealthPage';
import ProvidersPage from '@/pages/ProvidersPage';
import ProxiesPage from '@/pages/ProxiesPage';
import SettingsPage from '@/pages/SettingsPage';
import DocsPage from '@/pages/DocsPage';
import UsersPage from '@/pages/UsersPage';
import UserDetailPage from '@/pages/UserDetailPage';

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const isAuthenticated = useAuthStore((state) => state.isAuthenticated);

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  return <>{children}</>;
}

function AdminRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isAdmin } = useAuthStore();

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  if (!isAdmin) {
    return <Navigate to="/dashboard" replace />;
  }

  return <>{children}</>;
}

function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route
        path="/"
        element={
          <ProtectedRoute>
            <Layout />
          </ProtectedRoute>
        }
      >
        <Route index element={<Navigate to="/dashboard" replace />} />
        {/* All users */}
        <Route path="dashboard" element={<DashboardPage />} />
        <Route path="usage" element={<UsagePage />} />
        <Route path="api-keys" element={<ApiKeysPage />} />
        <Route path="docs" element={<DocsPage />} />
        {/* Admin only */}
        <Route path="users" element={<AdminRoute><UsersPage /></AdminRoute>} />
        <Route path="users/:id" element={<AdminRoute><UserDetailPage /></AdminRoute>} />
        <Route path="health" element={<AdminRoute><HealthPage /></AdminRoute>} />
        <Route path="providers" element={<AdminRoute><ProvidersPage /></AdminRoute>} />
        <Route path="proxies" element={<AdminRoute><ProxiesPage /></AdminRoute>} />
        <Route path="settings" element={<AdminRoute><SettingsPage /></AdminRoute>} />
      </Route>
    </Routes>
  );
}

export default App;
