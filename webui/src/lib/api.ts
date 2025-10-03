// API client for interacting with the backend

const API_BASE = '/api';

export interface Provider {
  ID: number;
  Name: string;
  Type: string;
  Config: string;
  Console: string;
}

export interface Model {
  ID: number;
  Name: string;
  Remark: string;
  MaxRetry: number;
  TimeOut: number;
}

export interface ModelWithProvider {
  ID: number;
  ModelID: number;
  ProviderModel: string;
  ProviderID: number;
  ToolCall: boolean;
  StructuredOutput: boolean;
  Image: boolean;
  Weight: number;
}

export interface SystemConfig {
  enable_smart_routing: boolean;
  success_rate_weight: number;
  response_time_weight: number;
  decay_threshold_hours: number;
  min_weight: number;
}

export interface SystemStatus {
  total_providers: number;
  total_models: number;
  active_requests: number;
  uptime: string;
  version: string;
}

export interface ProviderMetric {
  provider_id: number;
  provider_name: string;
  success_rate: number;
  avg_response_time: number;
  total_requests: number;
  success_count: number;
  failure_count: number;
}

// Generic API request function
async function apiRequest<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
  const url = `${API_BASE}${endpoint}`;

  // Get token from localStorage
  const token = localStorage.getItem("authToken");

  const response = await fetch(url, {
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
      ...options.headers,
    },
    ...options,
  });

  // Handle 401 Unauthorized response
  if (response.status === 401) {
    // Redirect to login page
    window.location.href = '/login';
    throw new Error('Unauthorized');
  }

  if (!response.ok) {
    throw new Error(`API request failed: ${response.status} ${response.statusText}`);
  }

  const data = await response.json();
  if (data.code !== 200) {
    throw new Error(`API request failed: ${data.code} ${data.message}`);
  }
  return data.data as T;
}

// Provider API functions
export async function getProviders(): Promise<Provider[]> {
  return apiRequest<Provider[]>('/providers');
}

export async function createProvider(provider: {
  name: string;
  type: string;
  config: string;
  console: string;
}): Promise<Provider> {
  return apiRequest<Provider>('/providers', {
    method: 'POST',
    body: JSON.stringify(provider),
  });
}

export async function updateProvider(id: number, provider: {
  name?: string;
  type?: string;
  config?: string;
  console?: string;
}): Promise<Provider> {
  return apiRequest<Provider>(`/providers/${id}`, {
    method: 'PUT',
    body: JSON.stringify(provider),
  });
}

export async function deleteProvider(id: number): Promise<void> {
  await apiRequest<void>(`/providers/${id}`, {
    method: 'DELETE',
  });
}

// Model API functions
export async function getModels(): Promise<Model[]> {
  return apiRequest<Model[]>('/models');
}

export async function createModel(model: {
  name: string;
  remark: string;
  max_retry: number;
  time_out: number;
}): Promise<Model> {
  return apiRequest<Model>('/models', {
    method: 'POST',
    body: JSON.stringify(model),
  });
}

export async function updateModel(id: number, model: {
  name?: string;
  remark?: string;
  max_retry?: number;
  time_out?: number;
}): Promise<Model> {
  return apiRequest<Model>(`/models/${id}`, {
    method: 'PUT',
    body: JSON.stringify(model),
  });
}

export async function deleteModel(id: number): Promise<void> {
  await apiRequest<void>(`/models/${id}`, {
    method: 'DELETE',
  });
}

// Model-Provider API functions
export async function getModelProviders(modelId: number): Promise<ModelWithProvider[]> {
  return apiRequest<ModelWithProvider[]>(`/model-providers?model_id=${modelId}`);
}

export async function getModelProviderStatus(providerId: number, modelName: string, providerModel: string): Promise<boolean[]> {
  const params = new URLSearchParams({
    provider_id: providerId.toString(),
    model_name: modelName,
    provider_model: providerModel
  });
  return apiRequest<boolean[]>(`/model-providers/status?${params.toString()}`);
}

export async function createModelProvider(association: {
  model_id: number;
  provider_name: string;
  provider_id: number;
  tool_call: boolean;
  structured_output: boolean;
  image: boolean;
  weight: number;
}): Promise<ModelWithProvider> {
  return apiRequest<ModelWithProvider>('/model-providers', {
    method: 'POST',
    body: JSON.stringify(association),
  });
}

export async function updateModelProvider(id: number, association: {
  model_id?: number;
  provider_name?: string;
  provider_id?: number;
  tool_call?: boolean;
  structured_output?: boolean;
  image?: boolean;
  weight?: number;
}): Promise<ModelWithProvider> {
  return apiRequest<ModelWithProvider>(`/model-providers/${id}`, {
    method: 'PUT',
    body: JSON.stringify(association),
  });
}

export async function deleteModelProvider(id: number): Promise<void> {
  await apiRequest<void>(`/model-providers/${id}`, {
    method: 'DELETE',
  });
}

// System API functions
export async function getSystemStatus(): Promise<SystemStatus> {
  return apiRequest<SystemStatus>('/status');
}

export async function getProviderMetrics(): Promise<ProviderMetric[]> {
  return apiRequest<ProviderMetric[]>('/metrics/providers');
}

export async function getSystemConfig(): Promise<SystemConfig> {
  return apiRequest<SystemConfig>('/config');
}

export async function updateSystemConfig(config: SystemConfig): Promise<SystemConfig> {
  return apiRequest<SystemConfig>('/config', {
    method: 'PUT',
    body: JSON.stringify(config),
  });
}

// Metrics API functions
export interface MetricsData {
  reqs: number;
  tokens: number;
}

export interface ModelCount {
  model: string;
  calls: number;
}

export async function getMetrics(days: number): Promise<MetricsData> {
  return apiRequest<MetricsData>(`/metrics/use/${days}`);
}

export async function getModelCounts(): Promise<ModelCount[]> {
  return apiRequest<ModelCount[]>('/metrics/counts');
}

// Test API functions
export async function testModelProvider(id: number): Promise<any> {
  return apiRequest<any>(`/test/${id}`);
}

// Provider Templates API functions
export interface ProviderTemplate {
  type: string;
  template: string;
}

export async function getProviderTemplates(): Promise<ProviderTemplate[]> {
  return apiRequest<ProviderTemplate[]>('/providers/template');
}

// Provider Models API functions
export interface ProviderModel {
  id: string;
  object: string;
  created: number;
  owned_by: string;
}

export async function getProviderModels(providerId: number): Promise<ProviderModel[]> {
  return apiRequest<ProviderModel[]>(`/providers/models/${providerId}`);
}

// Logs API functions
export interface ChatLog {
  ID: number;
  CreatedAt: string;
  Name: string;
  ProviderModel: string;
  ProviderName: string;
  Status: string;
  Style: string;
  Error: string;
  Retry: number;
  ProxyTime: number;
  FirstChunkTime: number;
  ChunkTime: number;
  Tps: number;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
}

export interface LogsResponse {
  data: ChatLog[];
  total: number;
  page: number;
  page_size: number;
  pages: number;
}

export async function getLogs(
  page: number = 1,
  pageSize: number = 20,
  filters: {
    name?: string;
    providerModel?: string;
    providerName?: string;
    status?: string;
    style?: string;
  } = {}
): Promise<LogsResponse> {
  const params = new URLSearchParams();
  params.append("page", page.toString());
  params.append("page_size", pageSize.toString());

  if (filters.name) params.append("name", filters.name);
  if (filters.providerModel) params.append("provider_model", filters.providerModel);
  if (filters.providerName) params.append("provider_name", filters.providerName);
  if (filters.status) params.append("status", filters.status);
  if (filters.style) params.append("style", filters.style);

  return apiRequest<LogsResponse>(`/logs?${params.toString()}`);
}

// Enhanced API functions for user experience improvements

// Provider Health Check
export interface ProviderHealth {
  provider_id: number;
  provider_name: string;
  provider_type: string;
  status: string; // healthy, degraded, unhealthy, unknown
  response_time_ms: number;
  last_checked: string;
  error_message?: string;
  success_rate_24h: number;
  total_requests_24h: number;
  avg_response_time_ms: number;
}

export async function getProviderHealth(providerId: number): Promise<ProviderHealth> {
  return apiRequest<ProviderHealth>(`/providers/health/${providerId}`);
}

export async function getAllProvidersHealth(): Promise<ProviderHealth[]> {
  return apiRequest<ProviderHealth[]>('/providers/health');
}

// Dashboard Stats
export interface DashboardStats {
  total_providers: number;
  healthy_providers: number;
  total_models: number;
  total_requests_24h: number;
  success_requests_24h: number;
  failed_requests_24h: number;
  avg_response_time_ms: number;
  total_tokens_24h: number;
  top_models: Array<{
    model_name: string;
    request_count: number;
    success_rate: number;
    total_tokens: number;
    avg_response_time_ms: number;
  }>;
  top_providers: Array<{
    provider_name: string;
    request_count: number;
    success_rate: number;
    total_tokens: number;
    avg_response_time_ms: number;
  }>;
}

export async function getDashboardStats(): Promise<DashboardStats> {
  return apiRequest<DashboardStats>('/dashboard/stats');
}

// Realtime Stats
export interface RealtimeStats {
  requests_1h: number;
  success_rate_1h: number;
  avg_response_time_1h: number;
  timestamp: number;
}

export async function getRealtimeStats(): Promise<RealtimeStats> {
  return apiRequest<RealtimeStats>('/dashboard/realtime');
}

// Batch Operations
export async function batchDeleteProviders(ids: number[]): Promise<{ deleted_count: number; deleted_ids: number[] }> {
  return apiRequest<{ deleted_count: number; deleted_ids: number[] }>('/providers/batch-delete', {
    method: 'POST',
    body: JSON.stringify({ ids }),
  });
}

export async function batchDeleteModels(ids: number[]): Promise<{ deleted_count: number; deleted_ids: number[] }> {
  return apiRequest<{ deleted_count: number; deleted_ids: number[] }>('/models/batch-delete', {
    method: 'POST',
    body: JSON.stringify({ ids }),
  });
}

// Provider Validation
export interface ProviderValidation {
  valid: boolean;
  error_message?: string;
  models?: string[];
  response_time_ms: number;
}

export async function validateProviderConfig(provider: {
  name: string;
  type: string;
  config: string;
  console: string;
}): Promise<ProviderValidation> {
  return apiRequest<ProviderValidation>('/providers/validate', {
    method: 'POST',
    body: JSON.stringify(provider),
  });
}

// Export Functions
export function exportLogs(filters: {
  name?: string;
  provider_name?: string;
  status?: string;
  style?: string;
  days?: number;
} = {}): string {
  const params = new URLSearchParams();
  const token = localStorage.getItem("authToken");
  
  if (filters.name) params.append("name", filters.name);
  if (filters.provider_name) params.append("provider_name", filters.provider_name);
  if (filters.status) params.append("status", filters.status);
  if (filters.style) params.append("style", filters.style);
  if (filters.days) params.append("days", filters.days.toString());
  
  const queryString = params.toString();
  const url = `/api/logs/export${queryString ? '?' + queryString : ''}`;
  
  // Return URL for download
  return url + (token ? `${queryString ? '&' : '?'}token=${token}` : '');
}

export function exportConfig(): string {
  const token = localStorage.getItem("authToken");
  return `/api/config/export${token ? '?token=' + token : ''}`;
}

// Import Configuration
export interface ImportConfigData {
  providers: Provider[];
  models: Model[];
  model_providers: ModelWithProvider[];
}

export async function importConfig(config: ImportConfigData): Promise<{ imported_count: number; message: string }> {
  return apiRequest<{ imported_count: number; message: string }>('/config/import', {
    method: 'POST',
    body: JSON.stringify(config),
  });
}

// Clear Logs
export async function clearLogs(days: number): Promise<{ deleted_count: number; cutoff_date: string }> {
  return apiRequest<{ deleted_count: number; cutoff_date: string }>(`/logs/clear?days=${days}`, {
    method: 'DELETE',
  });
}