import { api } from '@/lib/api'
import type {
  SpecialUsageConfig,
  SpecialUsageDateRange,
  SpecialUsageForecast,
  SpecialUsageMetadata,
  SpecialUsageOverview,
  SpecialUsageProfit,
  SpecialUsageRecordsPage,
} from './types'

type ApiResponse<T> = {
  success: boolean
  message?: string
  data: T
}

export type SpecialUsageQuery = SpecialUsageDateRange & {
  groups: string[]
  models: string[]
  channels?: number[]
}

export type SpecialUsageRecordsQuery = SpecialUsageQuery & {
  page: number
  page_size: number
}

type SpecialUsageProfitQuery = SpecialUsageQuery & {
  mode: 'auto' | 'manual'
  revenue?: number
}

function paramsFromQuery(query: SpecialUsageQuery): Record<string, string | number> {
  const params: Record<string, string | number> = {
    start_time: query.start,
    end_time: query.end,
    group: query.groups.join(','),
    model: query.models.join(','),
  }
  if (query.channels && query.channels.length > 0) params.channel_id = query.channels.join(',')
  return params
}

export async function getSpecialUsageMetadata(): Promise<ApiResponse<SpecialUsageMetadata>> {
  const response = await api.get('/api/special-usage/metadata')
  return response.data as ApiResponse<SpecialUsageMetadata>
}

export async function saveSpecialUsageConfig(
  config: Omit<SpecialUsageConfig, 'updated_at'>
): Promise<ApiResponse<SpecialUsageConfig>> {
  const response = await api.put('/api/special-usage/config', config)
  return response.data as ApiResponse<SpecialUsageConfig>
}

export async function getSpecialUsageOverview(
  query: SpecialUsageQuery
): Promise<ApiResponse<SpecialUsageOverview>> {
  const response = await api.get('/api/special-usage/overview', {
    params: paramsFromQuery(query),
  })
  return response.data as ApiResponse<SpecialUsageOverview>
}

export async function getSpecialUsageForecast(
  query: SpecialUsageQuery,
  basis: string,
  days: number
): Promise<ApiResponse<SpecialUsageForecast>> {
  const response = await api.get('/api/special-usage/forecast', {
    params: { ...paramsFromQuery(query), basis, days },
  })
  return response.data as ApiResponse<SpecialUsageForecast>
}

export async function getSpecialUsageRecords(
  query: SpecialUsageRecordsQuery
): Promise<ApiResponse<SpecialUsageRecordsPage>> {
  const response = await api.get('/api/special-usage/records', {
    params: { ...paramsFromQuery(query), page: query.page, page_size: query.page_size },
  })
  return response.data as ApiResponse<SpecialUsageRecordsPage>
}

export async function getSpecialUsageProfit(
  query: SpecialUsageProfitQuery
): Promise<ApiResponse<SpecialUsageProfit>> {
  const params: Record<string, string | number> = {
    ...paramsFromQuery(query),
    mode: query.mode,
  }
  if (query.mode === 'manual') params.revenue = query.revenue ?? 0
  const response = await api.get('/api/special-usage/profit', { params })
  return response.data as ApiResponse<SpecialUsageProfit>
}

export async function downloadSpecialUsageExport(query: SpecialUsageQuery): Promise<void> {
  const response = await api.get('/api/special-usage/export', {
    params: paramsFromQuery(query),
    responseType: 'blob',
    disableDuplicate: true,
  } as never)
  const blob = new Blob([response.data], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = `special-usage-${new Date().toISOString().slice(0, 10)}.csv`
  anchor.click()
  URL.revokeObjectURL(url)
}
