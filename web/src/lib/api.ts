import axios, { AxiosError, AxiosInstance, AxiosRequestConfig } from 'axios';
import toast from 'react-hot-toast';
import { useAuthStore } from '@/stores/authStore';

const BASE_URL = '/api/v1';

class ApiClient {
  private client: AxiosInstance;

  constructor() {
    this.client = axios.create({
      baseURL: BASE_URL,
      headers: {
        'Content-Type': 'application/json',
      },
    });

    this.setupInterceptors();
  }

  private setupInterceptors(): void {
    this.client.interceptors.request.use(
      (config) => {
        const token = useAuthStore.getState().token;
        if (token) {
          config.headers.Authorization = `Bearer ${token}`;
        }
        return config;
      },
      (error) => Promise.reject(error)
    );

    this.client.interceptors.response.use(
      (response) => response,
      (error: AxiosError<{ error?: string; message?: string }>) => {
        const status = error.response?.status;
        const msg = error.response?.data?.error || error.response?.data?.message;

        if (status === 401) {
          useAuthStore.getState().logout();
          window.location.href = '/login';
        } else if (status === 403) {
          toast.error(msg || 'Access denied');
        } else if (status && status >= 500) {
          toast.error(msg || 'Server error — please try again later');
        }

        return Promise.reject(error);
      }
    );
  }

  async get<T>(url: string, config?: AxiosRequestConfig): Promise<T> {
    const response = await this.client.get<T>(url, config);
    return response.data;
  }

  async post<T>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<T> {
    const response = await this.client.post<T>(url, data, config);
    return response.data;
  }

  async put<T>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<T> {
    const response = await this.client.put<T>(url, data, config);
    return response.data;
  }

  async delete<T>(url: string, config?: AxiosRequestConfig): Promise<T> {
    const response = await this.client.delete<T>(url, config);
    return response.data;
  }
}

export const api = new ApiClient();

// ── Error Helpers ────────────────────────────────────────────
// Use these instead of importing axios directly in pages.

/**
 * Type-guard: checks if an unknown error is an Axios API error.
 */
export function isApiError(error: unknown): error is AxiosError<{ error?: string; message?: string }> {
  return axios.isAxiosError(error);
}

/**
 * Extracts a user-friendly error message from an API error.
 * Returns the fallback string if the error is not an API error.
 */
export function getApiErrorMessage(error: unknown, fallback = 'An error occurred'): string {
  if (isApiError(error)) {
    return error.response?.data?.error || error.response?.data?.message || fallback;
  }
  return fallback;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface LoginResponse {
  token: string;
  user: User;
}

export interface RegisterRequest {
  email: string;
  password: string;
  name: string;
}

export interface User {
  id: string;
  email: string;
  name: string;
  role: string;
  require_password_change?: boolean;
  monthly_token_limit?: number;
  monthly_budget_usd?: number;
  created_at?: string;
}

export interface ApiKey {
  id: string;
  name: string;
  key: string;
  key_prefix: string;
  is_active: boolean;
  rate_limit: number;
  daily_limit: number;
  expires_at: string | null;
  last_used_at: string | null;
  created_at: string;
}

export interface OverviewStats {
  total_requests: number;
  success_rate: number;
  total_tokens: number;
  total_cost: number;
  average_latency_ms: number;
  active_users: number;
  active_providers: number;
  active_proxies: number;
  requests_today: number;
  cost_today: number;
  tokens_today: number;
  error_count: number;
  api_keys: {
    total: number;
    healthy: number;
  };
  proxies: {
    total: number;
    healthy: number;
  };
}

export interface UsageChartData {
  date: string;
  requests: number;
  tokens: number;
  cost: number;
}

export interface ProviderStats {
  provider_id: string;
  provider_name: string;
  requests: number;
  tokens: number;
  success_rate: number;
  avg_latency_ms: number;
  total_cost: number;
}

export interface ModelStats {
  model_id: string;
  model_name: string;
  requests: number;
  input_tokens: number;
  output_tokens: number;
  total_cost: number;
}

export interface ApiKeyHealth {
  id: string;
  provider_id: string;
  provider_name: string;
  key_prefix: string;
  is_active: boolean;
  is_healthy: boolean;
  last_check: string;
  response_time: number;
  success_rate: number;
}

export interface ProxyHealth {
  id: string;
  url: string;
  type: string;
  region: string;
  is_active: boolean;
  is_healthy: boolean;
  response_time: number;
  last_check: string;
  success_rate: number;
}

export interface ProviderHealth {
  id: string;
  name: string;
  base_url: string;
  is_active: boolean;
  is_healthy: boolean;
  use_proxy: boolean;
  response_time: number;
  last_check: string;
  success_rate: number;
  error_message?: string;
}

export interface Alert {
  id: string;
  target_type: string;
  target_id: string;
  alert_type: string;
  message: string;
  status: string;
  resolved_at?: string;
  acknowledged_at?: string;
  created_at: string;
}

export interface Provider {
  id: string;
  name: string;
  base_url: string;
  is_active: boolean;
  priority: number;
  weight: number;
  max_retries: number;
  timeout: number;
  use_proxy: boolean;
  default_proxy_id?: string | null;
  requires_api_key: boolean;
  created_at: string;
}

export interface ProviderApiKey {
  id: string;
  provider_id: string;
  alias: string;
  key_prefix: string;
  is_active: boolean;
  priority: number;
  weight: number;
  rate_limit: number;
  usage_count: number;
  last_used_at?: string;
  created_at: string;
}

export interface ProviderHealthStatus {
  id: string;
  name: string;
  base_url: string;
  is_active: boolean;
  is_healthy: boolean;
  use_proxy: boolean;
  response_time: number;  // milliseconds
  last_check: string;
  success_rate: number;
  error_message?: string;
}

export interface Proxy {
  id: string;
  url: string;
  type: string;
  region: string;
  is_active: boolean;
  weight: number;
  success_count: number;
  failure_count: number;
  avg_latency: number;
  last_checked: string;
  created_at: string;
  username?: string;
  has_auth?: boolean;
  upstream_proxy_id?: string;
}

export interface UsageRecord {
  id: string;
  model_name: string;
  input_tokens: number;
  output_tokens: number;
  cost: number;
  latency_ms: number;
  is_success: boolean;
  created_at: string;
}

export interface DailyStats {
  date: string;
  requests: number;
  total_tokens: number;
  total_cost: number;
}

export interface MonthlyUsage {
  total_requests: number;
  success_rate: number;
  total_tokens: number;
  total_cost: number;
}

export const authApi = {
  login: (data: LoginRequest) => api.post<LoginResponse>('/auth/login', data),
  register: (data: RegisterRequest) => api.post<LoginResponse>('/auth/register', data),
  getCurrentUser: () => api.get<User>('/user/profile'),
};

export const userApi = {
  getProfile: () => api.get<User>('/user/profile'),
  changePassword: (data: { old_password: string; new_password: string }) => api.put('/user/password', data),
};

export const dashboardApi = {
  getOverview: () => api.get<OverviewStats>('/dashboard/overview'),
  getUsageChart: () => api.get<{ data: UsageChartData[] }>('/dashboard/usage-chart'),
  getProviderStats: () => api.get<{ data: ProviderStats[] }>('/dashboard/provider-stats'),
  getModelStats: () => api.get<{ data: ModelStats[] }>('/dashboard/model-stats'),
};

export const apiKeysApi = {
  list: () => api.get<{ data: ApiKey[] }>('/api-keys'),
  create: (name: string) => api.post<ApiKey>('/api-keys', { name }),
  revoke: (id: string) => api.post(`/api-keys/${id}/revoke`),
  delete: (id: string) => api.delete(`/api-keys/${id}`),
};

export const usageApi = {
  getRecords: (page: number, pageSize: number) =>
    api.get<{ data: UsageRecord[]; total: number }>(`/usage/recent?page=${page}&page_size=${pageSize}`),
  getDailyStats: (days: number) => api.get<{ data: DailyStats[] }>(`/usage/daily?days=${days}`),
  getMonthlyUsage: () => api.get<MonthlyUsage>('/usage/summary'),
};

export const healthApi = {
  getApiKeysHealth: () => api.get<{ data: ApiKeyHealth[] }>('/health/api-keys'),
  checkApiKey: (id: string) => api.post<ApiKeyHealth>(`/health/api-keys/${id}/check`),
  getProxiesHealth: () => api.get<{ data: ProxyHealth[] }>('/health/proxies'),
  checkProxy: (id: string) => api.post<ProxyHealth>(`/health/proxies/${id}/check`),
  getProvidersHealth: () => api.get<ProviderHealth[]>('/health/providers'),
  checkProvider: (id: string) => api.post<ProviderHealth>(`/health/providers/${id}/check`),
  checkAllProviders: () => api.post<{ message: string }>('/health/providers/check-all'),
};

export const alertsApi = {
  list: (status?: string) =>
    api.get<{ data: Alert[]; total: number }>(`/alerts${status ? `?status=${status}` : ''}`),
  acknowledge: (id: string) => api.post(`/alerts/${id}/acknowledge`),
  resolve: (id: string) => api.post(`/alerts/${id}/resolve`),
};

export const providersApi = {
  list: () => api.get<{ data: Provider[] }>('/providers'),
  update: (id: string, data: Partial<Provider>) => api.put<Provider>(`/providers/${id}`, data),
  toggle: (id: string) => api.post<Provider>(`/providers/${id}/toggle`),
  toggleProxy: (id: string) => api.post<Provider>(`/providers/${id}/toggle-proxy`),
  checkHealth: (id: string) => api.get<ProviderHealthStatus>(`/providers/${id}/health`),
  getApiKeys: (providerId: string) =>
    api.get<{ data: ProviderApiKey[] }>(`/providers/${providerId}/api-keys`),
  createApiKey: (providerId: string, data: { api_key: string; alias: string; priority?: number; weight?: number; rate_limit?: number }) =>
    api.post<ProviderApiKey>(`/providers/${providerId}/api-keys`, data),
  updateApiKey: (providerId: string, keyId: string, data: { priority?: number; weight?: number; rate_limit?: number }) =>
    api.put<ProviderApiKey>(`/providers/${providerId}/api-keys/${keyId}`, data),
  toggleApiKey: (providerId: string, keyId: string) =>
    api.post<ProviderApiKey>(`/providers/${providerId}/api-keys/${keyId}/toggle`),
  deleteApiKey: (providerId: string, keyId: string) =>
    api.delete(`/providers/${providerId}/api-keys/${keyId}`),
};

export const proxiesApi = {
  list: () => api.get<{ data: Proxy[] }>('/proxies'),
  create: (data: { url: string; type: string; region: string; username?: string; password?: string; upstream_proxy_id?: string }) =>
    api.post<Proxy>('/proxies', data),
  batchCreate: (proxies: Array<{ url: string; type?: string; region?: string }>) =>
    api.post<{ success: number; failed: number; proxies: Proxy[]; errors?: string[] }>(
      '/proxies/batch',
      { proxies }
    ),
  update: (id: string, data: Partial<Proxy> & { password?: string; upstream_proxy_id?: string }) => api.put<Proxy>(`/proxies/${id}`, data),
  delete: (id: string) => api.delete(`/proxies/${id}`),
  toggle: (id: string) => api.post<Proxy>(`/proxies/${id}/toggle`),
  test: (id: string) =>
    api.post<{ id: string; url: string; is_healthy: boolean; latency_ms: number; error?: string }>(
      `/proxies/${id}/test`
    ),
  testAll: () =>
    api.post<{
      results: Array<{ id: string; url: string; is_healthy: boolean; latency_ms: number; error?: string }>;
    }>('/proxies/test-all'),
};

// ── Admin: User Management ──────────────────────────────────

export interface UserListItem {
  id: string;
  email: string;
  name: string;
  role: string;
  is_active: boolean;
  last_login_at: string;
  created_at: string;
  api_key_count: number;
}

export interface UserDetail {
  id: string;
  email: string;
  name: string;
  role: string;
  is_active: boolean;
  created_at: string;
  api_keys: number;
  monthly_token_limit: number;
  monthly_budget_usd: number;
  usage_month: {
    total_requests: number;
    total_tokens: number;
    total_cost: number;
    avg_latency: number;
    success_rate: number;
    error_count: number;
  };
}

export const usersApi = {
  list: (q?: string) =>
    api.get<{ data: UserListItem[]; total: number }>(`/users${q ? `?q=${encodeURIComponent(q)}` : ''}`),
  getById: (id: string) => api.get<UserDetail>(`/users/${id}`),
  getUsage: (id: string, days?: number) =>
    api.get<{ data: DailyStats[] }>(`/users/${id}/usage?days=${days || 30}`),
  getApiKeys: (id: string) =>
    api.get<{ data: ApiKey[] }>(`/users/${id}/api-keys`),
  toggle: (id: string) =>
    api.post<{ id: string; email: string; name: string; role: string; is_active: boolean }>(`/users/${id}/toggle`),
  updateRole: (id: string, role: string) =>
    api.put<{ id: string; email: string; name: string; role: string }>(`/users/${id}/role`, { role }),
  updateQuota: (id: string, data: { monthly_token_limit?: number; monthly_budget_usd?: number }) =>
    api.put(`/users/${id}/quota`, data),
};

