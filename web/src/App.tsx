import { Routes, Route, Navigate } from 'react-router-dom';
import { useAuthStore } from '@/stores/authStore';
import Layout from '@/components/Layout';
import LoginPage from '@/pages/LoginPage';
import ForgotPasswordPage from '@/pages/ForgotPasswordPage';
import ResetPasswordPage from '@/pages/ResetPasswordPage';
import DashboardPage from '@/pages/DashboardPage';
import UsagePage from '@/pages/UsagePage';
import ApiKeysPage from '@/pages/ApiKeysPage';
import HealthPage from '@/pages/HealthPage';
import ProvidersPage from '@/pages/ProvidersPage';
import McpPage from '@/pages/McpPage';
import PlansPage from '@/pages/PlansPage';
import BillingPage from '@/pages/BillingPage';
import ProxiesPage from '@/pages/ProxiesPage';
import SettingsPage from '@/pages/SettingsPage';
import AdminSettingsPage from '@/pages/AdminSettingsPage';
import DocsPage from '@/pages/DocsPage';
import UsersPage from '@/pages/UsersPage';
import UserDetailPage from '@/pages/UserDetailPage';
import ForcePasswordChangePage from '@/pages/ForcePasswordChangePage';

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, user } = useAuthStore((state) => state);

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  if (user?.require_password_change) {
    return <Navigate to="/change-password" replace />;
  }

  return <>{children}</>;
}

function AdminRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isAdmin, user } = useAuthStore();

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  if (user?.require_password_change) {
    return <Navigate to="/change-password" replace />;
  }

  if (!isAdmin) {
    return <Navigate to="/dashboard" replace />;
  }

  return <>{children}</>;
}

function AuthenticatedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated } = useAuthStore();
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }
  return <>{children}</>;
}

function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/forgot-password" element={<ForgotPasswordPage />} />
      <Route path="/reset-password" element={<ResetPasswordPage />} />
      <Route path="/change-password" element={
        <AuthenticatedRoute>
          <ForcePasswordChangePage />
        </AuthenticatedRoute>
      } />
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
        <Route path="plans" element={<ProtectedRoute><PlansPage /></ProtectedRoute>} />
        <Route path="billing" element={<ProtectedRoute><BillingPage /></ProtectedRoute>} />
        <Route path="profile" element={<ProtectedRoute><SettingsPage /></ProtectedRoute>} />
        <Route path="docs" element={<DocsPage />} />
        {/* Admin only */}
        <Route path="users" element={<AdminRoute><UsersPage /></AdminRoute>} />
        <Route path="users/:id" element={<AdminRoute><UserDetailPage /></AdminRoute>} />
        <Route path="health" element={<AdminRoute><HealthPage /></AdminRoute>} />
        <Route path="providers" element={<AdminRoute><ProvidersPage /></AdminRoute>} />
        <Route path="mcp" element={<AdminRoute><McpPage /></AdminRoute>} />
        <Route path="proxies" element={<AdminRoute><ProxiesPage /></AdminRoute>} />
        <Route path="settings" element={<AdminRoute><AdminSettingsPage /></AdminRoute>} />
      </Route>
    </Routes>
  );
}

export default App;
