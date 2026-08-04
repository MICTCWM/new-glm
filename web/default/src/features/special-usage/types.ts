export type SpecialUsageConfig = {
  enabled: boolean
  group_names: string[]
  model_names: string[]
  channel_ids: number[]
  channel_ids_set?: boolean
  special_billing: boolean
  channel_multipliers: Record<string, number>
  updated_at: number
}

export type SpecialUsageChannel = {
  id: number
  name: string
  groups: string[]
  models: string[]
  multiplier: number
  special_billing: boolean
  has_special_price: boolean
}

export type SpecialUsageMetadata = {
  groups: string[]
  models: string[]
  channels: SpecialUsageChannel[]
  config: SpecialUsageConfig
}

export type SpecialUsageTotals = {
  request_count: number
  input_tokens: number
  output_tokens: number
  upstream_cost_usd: number
  user_charge_usd: number
}

export type SpecialUsageSeriesPoint = {
  time: number
  request_count: number
  input_tokens: number
  output_tokens: number
  upstream_cost_usd: number
  user_charge_usd: number
}

export type SpecialUsageNamedValue = {
  name: string
  request_count: number
  input_tokens: number
  output_tokens: number
  upstream_cost_usd: number
  user_charge_usd: number
}

export type SpecialUsageChannelStat = {
  channel_id: number
  channel_name: string
  request_count: number
  input_tokens: number
  output_tokens: number
  upstream_cost_usd: number
  user_charge_usd: number
  average_cost_usd: number
  baseline_cost_usd?: number
  anomaly: boolean
  anomaly_reason?: string
}

export type SpecialUsageTreeNode = {
  name: string
  value: number
  children?: SpecialUsageTreeNode[]
}

export type SpecialUsageOverview = {
  totals: SpecialUsageTotals
  series: SpecialUsageSeriesPoint[]
  group_costs: SpecialUsageNamedValue[]
  model_tokens: SpecialUsageNamedValue[]
  channels: SpecialUsageChannelStat[]
  input_output: SpecialUsageNamedValue[]
  group_profit: SpecialUsageNamedValue[]
  channel_cost_tree: SpecialUsageTreeNode[]
  last_updated_at: number
}

export type SpecialUsageForecast = {
  basis: string
  days: number
  today_remaining?: boolean
  historical_days?: number
  daily_tokens: number
  daily_cost_usd?: number
  forecast_tokens: number
  average_cost_per_token: number
  forecast_cost_usd: number
}

export type SpecialUsageRecord = {
  id: number
  request_id: string
  user_id: number
  channel_id: number
  channel_name: string
  group_name: string
  model_name: string
  input_tokens: number
  output_tokens: number
  upstream_cost_usd: number
  user_charge_usd: number
  input_price_usd?: number
  output_price_usd?: number
  multiplier?: number
  used_special_price?: boolean
  usage_source?: string
  attempt?: number
  status: string
  request_time: number
  error_message?: string
}

export type SpecialUsageDateRange = {
  start: number
  end: number
}

export type SpecialUsageProfit = {
  revenue: number
  cost: number
  profit: number
  margin: number
}

export type SpecialUsageRecordsPage = {
  items: SpecialUsageRecord[]
  total: number
  page: number
  page_size: number
}
