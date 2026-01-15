import axios, { AxiosError, AxiosInstance, AxiosRequestConfig } from 'axios';
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
      (error: AxiosError) => {
        if (error.response?.status === 401) {
          useAuthStore.getState().logout();
          window.location.href = '/login';
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
  created_at: string;
}

export interface ApiKey {
  id: string;
  name: string;
  key: string;
  prefix: string;
  is_active: boolean;
  created_at: string;
  last_used_at: string | null;
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
  alias: string;
  status: string;
  latency_ms: number;
  last_checked: string;
  error_message?: string;
}

export interface ProxyHealth {
  id: string;
  name: string;
  host: string;
  port: number;
  status: string;
  latency_ms: number;
  last_checked: string;
  success_rate: number;
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
  created_at: string;
}

export interface ProviderApiKey {
  id: string;
  provider_id: string;
  alias: string;
  status: string;
  priority: number;
  last_used_at?: string;
  created_at: string;
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
  getCurrentUser: () => api.get<User>('/me'),
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
  revoke: (id: string) => api.delete(`/api-keys/${id}`),
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
};

export const alertsApi = {
  list: (status?: string) =>
    api.get<{ data: Alert[]; total: number }>(`/alerts${status ? `?status=${status}` : ''}`),
  acknowledge: (id: string) => api.post(`/alerts/${id}/acknowledge`),
  resolve: (id: string) => api.post(`/alerts/${id}/resolve`),
};

export const providersApi = {
  list: () => api.get<{ data: Provider[] }>('/providers'),
  getApiKeys: (providerId: string) =>
    api.get<{ data: ProviderApiKey[] }>(`/providers/${providerId}/api-keys`),
  createApiKey: (providerId: string, data: { api_key: string; alias: string }) =>
    api.post<ProviderApiKey>(`/providers/${providerId}/api-keys`, data),
  deleteApiKey: (providerId: string, keyId: string) =>
    api.delete(`/providers/${providerId}/api-keys/${keyId}`),
};

export const proxiesApi = {
  list: () => api.get<{ data: Proxy[] }>('/proxies'),
  create: (data: { url: string; type: string; region: string }) =>
    api.post<Proxy>('/proxies', data),
  update: (id: string, data: Partial<Proxy>) => api.put<Proxy>(`/proxies/${id}`, data),
  delete: (id: string) => api.delete(`/proxies/${id}`),
};
