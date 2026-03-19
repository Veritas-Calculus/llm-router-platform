// ── Frontend Type Definitions ──────────────────────────────────────
// Extracted from api.ts after GraphQL migration.
// These types describe the data shapes used by page components.

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

export interface ForgotPasswordRequest {
  email: string;
}

export interface ResetPasswordRequest {
  token: string;
  new_password: string;
}

export interface User {
  id: string;
  email: string;
  name: string;
  role: string;
  require_password_change?: boolean;
  monthly_token_limit?: number;
  monthly_budget_usd?: number;
  balance?: number;
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
  mcp_call_count: number;
  mcp_error_count: number;
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

export interface AlertConfig {
  id?: string;
  target_type: string;
  target_id: string;
  is_enabled: boolean;
  failure_threshold: number;
  webhook_url: string;
  email: string;
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
  response_time: number;
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

export interface McpServer {
  id: string;
  name: string;
  type: 'stdio' | 'sse';
  command?: string;
  args?: string[];
  env?: Record<string, string>;
  url?: string;
  is_active: boolean;
  status: 'connected' | 'disconnected' | 'error';
  last_error?: string;
  last_checked_at: string;
  tools?: McpTool[];
  created_at: string;
}

export interface McpTool {
  id: string;
  server_id: string;
  name: string;
  description: string;
  input_schema: Record<string, unknown>;
  is_active: boolean;
}

export interface Plan {
  id: string;
  name: string;
  description: string;
  price_month: number;
  token_limit: number;
  rate_limit: number;
  support_level: string;
  is_active: boolean;
  features: string;
}

export interface Subscription {
  id: string;
  user_id: string;
  plan_id: string;
  status: 'active' | 'trialing' | 'canceled' | 'past_due';
  current_period_start: string;
  current_period_end: string;
  cancel_at_period_end: boolean;
  plan: Plan;
}

export interface Order {
  id: string;
  order_no: string;
  amount: number;
  currency: string;
  status: 'pending' | 'paid' | 'failed' | 'expired';
  payment_method: string;
  created_at: string;
  plan?: Plan;
}

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
