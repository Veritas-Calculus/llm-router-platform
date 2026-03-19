import { Routes, Route, Navigate } from 'react-router-dom';
import { useAuthStore } from '@/stores/authStore';
import Layout from '@/components/Layout';
import LoginPage from '@/pages/LoginPage';
import ForgotPasswordPage from '@/pages/ForgotPasswordPage';
import ResetPasswordPage from '@/pages/ResetPasswordPage';
import ForcePasswordChangePage from '@/pages/ForcePasswordChangePage';

// User pages
import ApiKeysPage from '@/pages/ApiKeysPage';
import UsagePage from '@/pages/UsagePage';
import SubscriptionPage from '@/pages/SubscriptionPage';
import RedeemPage from '@/pages/RedeemPage';
import DocsPage from '@/pages/DocsPage';
import SettingsPage from '@/pages/SettingsPage';
import PlansPage from '@/pages/PlansPage';
import BillingPage from '@/pages/BillingPage';

// Admin pages
import DashboardPage from '@/pages/DashboardPage';
import UsersPage from '@/pages/UsersPage';
import UserDetailPage from '@/pages/UserDetailPage';
import AnnouncementsPage from '@/pages/AnnouncementsPage';
import AdminPlansPage from '@/pages/AdminPlansPage';
import RedeemCodesPage from '@/pages/RedeemCodesPage';
import CouponsPage from '@/pages/CouponsPage';
import ProxiesPage from '@/pages/ProxiesPage';
import McpPage from '@/pages/McpPage';
import AdminDocsPage from '@/pages/AdminDocsPage';
import AdminSettingsPage from '@/pages/AdminSettingsPage';
import HealthPage from '@/pages/HealthPage';
import ProvidersPage from '@/pages/ProvidersPage';

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
    return <Navigate to="/api-keys" replace />;
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
        <Route index element={<Navigate to="/api-keys" replace />} />

        {/* ── User pages ── */}
        <Route path="api-keys" element={<ApiKeysPage />} />
        <Route path="usage" element={<UsagePage />} />
        <Route path="subscription" element={<SubscriptionPage />} />
        <Route path="redeem" element={<RedeemPage />} />
        <Route path="docs" element={<DocsPage />} />
        <Route path="profile" element={<SettingsPage />} />
        {/* Legacy routes — keep for backward compat */}
        <Route path="plans" element={<PlansPage />} />
        <Route path="billing" element={<BillingPage />} />
        <Route path="dashboard" element={<DashboardPage />} />

        {/* ── Admin pages ── */}
        <Route path="admin/dashboard" element={<AdminRoute><DashboardPage /></AdminRoute>} />
        <Route path="admin/usage" element={<AdminRoute><UsagePage /></AdminRoute>} />
        <Route path="admin/users" element={<AdminRoute><UsersPage /></AdminRoute>} />
        <Route path="admin/users/:id" element={<AdminRoute><UserDetailPage /></AdminRoute>} />
        <Route path="admin/announcements" element={<AdminRoute><AnnouncementsPage /></AdminRoute>} />
        <Route path="admin/plans" element={<AdminRoute><AdminPlansPage /></AdminRoute>} />
        <Route path="admin/redeem-codes" element={<AdminRoute><RedeemCodesPage /></AdminRoute>} />
        <Route path="admin/coupons" element={<AdminRoute><CouponsPage /></AdminRoute>} />
        <Route path="admin/proxies" element={<AdminRoute><ProxiesPage /></AdminRoute>} />
        <Route path="admin/mcp" element={<AdminRoute><McpPage /></AdminRoute>} />
        <Route path="admin/docs" element={<AdminRoute><AdminDocsPage /></AdminRoute>} />
        <Route path="admin/settings" element={<AdminRoute><AdminSettingsPage /></AdminRoute>} />
        <Route path="admin/providers" element={<AdminRoute><ProvidersPage /></AdminRoute>} />
        <Route path="admin/health" element={<AdminRoute><HealthPage /></AdminRoute>} />
        {/* Legacy admin routes */}
        <Route path="health" element={<AdminRoute><HealthPage /></AdminRoute>} />
        <Route path="providers" element={<AdminRoute><ProvidersPage /></AdminRoute>} />
        <Route path="users" element={<AdminRoute><UsersPage /></AdminRoute>} />
        <Route path="users/:id" element={<AdminRoute><UserDetailPage /></AdminRoute>} />
        <Route path="mcp" element={<AdminRoute><McpPage /></AdminRoute>} />
        <Route path="proxies" element={<AdminRoute><ProxiesPage /></AdminRoute>} />
        <Route path="settings" element={<AdminRoute><AdminSettingsPage /></AdminRoute>} />
      </Route>
    </Routes>
  );
}

export default App;
