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

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const isAuthenticated = useAuthStore((state) => state.isAuthenticated);

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
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
        <Route path="dashboard" element={<DashboardPage />} />
        <Route path="usage" element={<UsagePage />} />
        <Route path="api-keys" element={<ApiKeysPage />} />
        <Route path="health" element={<HealthPage />} />
        <Route path="providers" element={<ProvidersPage />} />
        <Route path="proxies" element={<ProxiesPage />} />
        <Route path="settings" element={<SettingsPage />} />
      </Route>
    </Routes>
  );
}

export default App;
