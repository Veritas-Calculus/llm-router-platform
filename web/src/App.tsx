import { lazy, Suspense } from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import { useAuthStore } from '@/stores/authStore';
import Layout from '@/components/Layout';
import OnboardingTour from '@/components/OnboardingTour';

/* ── Lazy-loaded pages ─────────────────────────────────────────────── */

// Auth pages (kept eager since they're the entry point)
import LoginPage from '@/pages/LoginPage';
const ForgotPasswordPage = lazy(() => import('@/pages/ForgotPasswordPage'));
const ResetPasswordPage = lazy(() => import('@/pages/ResetPasswordPage'));
const VerifyEmailPage = lazy(() => import('@/pages/VerifyEmailPage'));
const ForcePasswordChangePage = lazy(() => import('@/pages/ForcePasswordChangePage'));
const OAuthCallbackPage = lazy(() => import('@/pages/OAuthCallbackPage'));

// User pages
const ApiKeysPage = lazy(() => import('@/pages/ApiKeysPage'));
const OrganizationMembersPage = lazy(() => import('@/pages/OrganizationMembersPage'));
const PlaygroundPage = lazy(() => import('@/pages/PlaygroundPage'));
const UsagePage = lazy(() => import('@/pages/UsagePage'));
const SubscriptionPage = lazy(() => import('@/pages/SubscriptionPage'));
const DocsPage = lazy(() => import('@/pages/DocsPage'));
const SettingsPage = lazy(() => import('@/pages/SettingsPage'));
const WebhooksPage = lazy(() => import('@/pages/WebhooksPage'));
const DlpSettingsPage = lazy(() => import('@/pages/DlpSettingsPage'));
const UserDashboardPage = lazy(() => import('@/pages/UserDashboardPage'));


// Admin pages
const AdminDashboardPage = lazy(() => import('@/pages/DashboardPage'));
const UsersPage = lazy(() => import('@/pages/UsersPage'));
const UserDetailPage = lazy(() => import('@/pages/UserDetailPage'));
const AnnouncementsPage = lazy(() => import('@/pages/AnnouncementsPage'));
const AdminPlansPage = lazy(() => import('@/pages/AdminPlansPage'));
const ProxiesPage = lazy(() => import('@/pages/ProxiesPage'));
const McpPage = lazy(() => import('@/pages/McpPage'));
const AdminDocsPage = lazy(() => import('@/pages/AdminDocsPage'));
const AdminSettingsPage = lazy(() => import('@/pages/AdminSettingsPage'));
const ProvidersPage = lazy(() => import('@/pages/ProvidersPage'));
const SemanticCachePage = lazy(() => import('@/pages/SemanticCachePage'));
const AuditLogsPage = lazy(() => import('@/pages/AuditLogsPage'));
const ErrorLogsPage = lazy(() => import('@/pages/ErrorLogsPage'));

const RoutingRulesPage = lazy(() => import('@/pages/RoutingRulesPage'));
const PromptRegistryPage = lazy(() => import('@/pages/PromptRegistryPage'));
const NotificationChannelsPage = lazy(() => import('@/pages/NotificationChannelsPage'));
const RateLimitDashboardPage = lazy(() => import('@/pages/RateLimitDashboardPage'));

// Merged pages
const AnalyticsPage = lazy(() => import('@/pages/AnalyticsPage'));
const PromotionsPage = lazy(() => import('@/pages/PromotionsPage'));
const MonitoringPage = lazy(() => import('@/pages/MonitoringPage'));

/* ── Suspense fallback ─────────────────────────────────────────────── */

function PageLoader() {
  return (
    <div className="flex items-center justify-center min-h-[40vh]">
      <div className="flex flex-col items-center gap-3">
        <div className="w-8 h-8 border-[3px] border-apple-gray-200 border-t-apple-blue rounded-full animate-spin" />
        <p className="text-sm text-apple-gray-400 font-medium">Loading…</p>
      </div>
    </div>
  );
}

/* ── Route guards ──────────────────────────────────────────────────── */

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

/* ── App ───────────────────────────────────────────────────────────── */

function App() {
  return (
    <Suspense fallback={<PageLoader />}>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/oauth/callback" element={<OAuthCallbackPage />} />
        <Route path="/forgot-password" element={<ForgotPasswordPage />} />
        <Route path="/reset-password" element={<ResetPasswordPage />} />
        <Route path="/verify-email" element={<VerifyEmailPage />} />
        <Route path="/change-password" element={
          <AuthenticatedRoute>
            <ForcePasswordChangePage />
          </AuthenticatedRoute>
        } />
        <Route
          path="/"
          element={
            <ProtectedRoute>
              <OnboardingTour />
              <Layout />
            </ProtectedRoute>
          }
        >
          <Route index element={<Navigate to="/dashboard" replace />} />

          {/* ── User pages ── */}
          <Route path="dashboard" element={<UserDashboardPage />} />
          <Route path="api-keys" element={<ApiKeysPage />} />
          <Route path="members" element={<OrganizationMembersPage />} />
          <Route path="playground" element={<PlaygroundPage />} />
          <Route path="usage" element={<UsagePage />} />
          <Route path="subscription" element={<SubscriptionPage />} />
          <Route path="docs" element={<DocsPage />} />
          <Route path="profile" element={<SettingsPage />} />
          <Route path="webhooks" element={<WebhooksPage />} />
          <Route path="dlp" element={<DlpSettingsPage />} />
          <Route path="plans" element={<Navigate to="/subscription" replace />} />
          <Route path="billing" element={<Navigate to="/subscription" replace />} />
          {/* Redirect old user routes */}
          <Route path="redeem" element={<Navigate to="/subscription" replace />} />

          {/* ── Admin pages ── */}
          <Route path="admin/dashboard" element={<AdminRoute><AdminDashboardPage /></AdminRoute>} />
          <Route path="admin/analytics" element={<AdminRoute><AnalyticsPage /></AdminRoute>} />
          <Route path="admin/users" element={<AdminRoute><UsersPage /></AdminRoute>} />
          <Route path="admin/users/:id" element={<AdminRoute><UserDetailPage /></AdminRoute>} />
          <Route path="admin/announcements" element={<AdminRoute><AnnouncementsPage /></AdminRoute>} />
          <Route path="admin/plans" element={<AdminRoute><AdminPlansPage /></AdminRoute>} />
          <Route path="admin/promotions" element={<AdminRoute><PromotionsPage /></AdminRoute>} />
          <Route path="admin/proxies" element={<AdminRoute><ProxiesPage /></AdminRoute>} />
          <Route path="admin/mcp" element={<AdminRoute><McpPage /></AdminRoute>} />
          <Route path="admin/docs" element={<AdminRoute><AdminDocsPage /></AdminRoute>} />
          <Route path="admin/settings" element={<AdminRoute><AdminSettingsPage /></AdminRoute>} />
          <Route path="admin/providers" element={<AdminRoute><ProvidersPage /></AdminRoute>} />
          <Route path="admin/cache" element={<AdminRoute><SemanticCachePage /></AdminRoute>} />
          <Route path="admin/monitoring" element={<AdminRoute><MonitoringPage /></AdminRoute>} />
          <Route path="admin/audit" element={<AdminRoute><AuditLogsPage /></AdminRoute>} />
          <Route path="admin/error-logs" element={<AdminRoute><ErrorLogsPage /></AdminRoute>} />
          <Route path="admin/integrations" element={<Navigate to="/admin/settings" replace />} />
          <Route path="admin/routing-rules" element={<AdminRoute><RoutingRulesPage /></AdminRoute>} />
          <Route path="admin/prompts" element={<AdminRoute><PromptRegistryPage /></AdminRoute>} />
          <Route path="admin/sso" element={<Navigate to="/admin/settings" replace />} />
          <Route path="admin/notifications" element={<AdminRoute><NotificationChannelsPage /></AdminRoute>} />
          <Route path="admin/rate-limits" element={<AdminRoute><RateLimitDashboardPage /></AdminRoute>} />
          {/* Redirect old admin routes to merged pages */}
          <Route path="admin/usage" element={<Navigate to="/admin/analytics" replace />} />
          <Route path="admin/cost-analysis" element={<Navigate to="/admin/analytics" replace />} />
          <Route path="admin/redeem-codes" element={<Navigate to="/admin/promotions" replace />} />
          <Route path="admin/coupons" element={<Navigate to="/admin/promotions" replace />} />
          <Route path="admin/health" element={<Navigate to="/admin/monitoring" replace />} />
          <Route path="admin/sla" element={<Navigate to="/admin/monitoring" replace />} />
          <Route path="admin/sla/alerts" element={<Navigate to="/admin/monitoring" replace />} />
          <Route path="admin/visual-router" element={<Navigate to="/admin/routing-rules" replace />} />
        </Route>
      </Routes>
    </Suspense>
  );
}

export default App;
