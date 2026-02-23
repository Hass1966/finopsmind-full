export interface CostSummary {
  total_cost: number
  currency: string
  by_service: { name: string; amount: number; percentage: number }[]
  by_provider?: { name: string; amount: number; percentage: number }[]
  date_range?: { start: string; end: string }
  change_percent: number
}

export interface CostTrend {
  data_points: { date: string; total: number }[]
  total_cost: number
  change_percent: number
}

export interface Anomaly {
  id: string
  date: string
  severity: string
  status: string
  deviation_pct: number
  service: string
  provider: string
  actual_amount: number
  expected_amount: number
  root_cause?: string
  notes?: string
}

export interface Budget {
  id: string
  name: string
  amount: number
  current_spend: number
  status: string
  period: string
  alert_threshold: number
}

export interface Recommendation {
  id: string
  type: string
  estimated_savings: number
  impact: string
  resource_type: string
  resource_id: string
  current_config: string
  recommended_config: string
  status: string
  provider: string
}

export interface Forecast {
  id: string
  model_version: string
  granularity: string
  predictions: { date: string; predicted: number; lower_bound: number; upper_bound: number }[]
  total_forecasted: number
  confidence_level: number
  created_at: string
}

export interface Provider {
  id: string
  name: string
  provider_type: string
  type?: string
  healthy: boolean
  enabled: boolean
  status: string
  status_message?: string
  last_sync_at?: string
  created_at?: string
}

export interface CloudProviderConfig {
  id: string
  name: string
  provider_type: string
  enabled: boolean
  status: string
  status_message: string
  last_sync_at?: string
  healthy: boolean
}

export interface AWSCredentialsInput {
  access_key_id: string
  secret_key: string
  region: string
  assume_role_arn?: string
  external_id?: string
}

export interface AzureCredentialsInput {
  tenant_id: string
  client_id: string
  client_secret: string
  subscription_id: string
}

export interface CreateProviderRequest {
  provider_type: string
  name: string
  credentials: AWSCredentialsInput | AzureCredentialsInput
}

export interface AuthUser {
  id: string
  organization_id: string
  email: string
  first_name: string
  last_name: string
  role: string
  last_login_at?: string
}

export interface LoginResponse {
  token: string
  expires_at: string
  user: AuthUser
}

export interface OrgSettings {
  default_currency: string
  timezone: string
  fiscal_year_start: number
  alerts_enabled: boolean
}
