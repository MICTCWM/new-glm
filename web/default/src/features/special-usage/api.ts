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

export type ApiResponse<T> = {
  success: boolean
  message?: string
  data?: T
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

export type SpecialUsageExportFormat = 'xlsx' | 'csv'

function apiErrorMessage(message: unknown, fallback: string): string {
  return typeof message === 'string' && message.trim() ? message : fallback
}

function unwrapApiResponse<T>(value: unknown): ApiResponse<T> {
  const response = value as Partial<ApiResponse<T>> | null
  if (!response || response.success !== true) {
    throw new Error(apiErrorMessage(response?.message, 'Request failed'))
  }
  if (!('data' in response)) {
    throw new Error(
      apiErrorMessage(response.message, 'Response data is missing')
    )
  }
  return response as ApiResponse<T>
}

function paramsFromQuery(
  query: SpecialUsageQuery
): Record<string, string | number> {
  const params: Record<string, string | number> = {
    start_time: query.start,
    end_time: query.end,
    group: query.groups.join(','),
    model: query.models.join(','),
  }
  if (query.channels) params.channel_id = query.channels.join(',')
  return params
}

export async function getSpecialUsageMetadata(): Promise<
  ApiResponse<SpecialUsageMetadata>
> {
  const response = await api.get('/api/special-usage/metadata')
  return unwrapApiResponse<SpecialUsageMetadata>(response.data)
}

export async function saveSpecialUsageConfig(
  config: Omit<SpecialUsageConfig, 'updated_at'>
): Promise<ApiResponse<SpecialUsageConfig>> {
  const response = await api.put('/api/special-usage/config', config)
  return unwrapApiResponse<SpecialUsageConfig>(response.data)
}

export async function getSpecialUsageOverview(
  query: SpecialUsageQuery
): Promise<ApiResponse<SpecialUsageOverview>> {
  const response = await api.get('/api/special-usage/overview', {
    params: paramsFromQuery(query),
  })
  return unwrapApiResponse<SpecialUsageOverview>(response.data)
}

export async function getSpecialUsageForecast(
  query: SpecialUsageQuery,
  basis: string,
  days: number,
  todayRemaining: boolean
): Promise<ApiResponse<SpecialUsageForecast>> {
  const response = await api.get('/api/special-usage/forecast', {
    params: {
      ...paramsFromQuery(query),
      basis,
      days,
      today_remaining: todayRemaining,
    },
  })
  return unwrapApiResponse<SpecialUsageForecast>(response.data)
}

export async function getSpecialUsageRecords(
  query: SpecialUsageRecordsQuery
): Promise<ApiResponse<SpecialUsageRecordsPage>> {
  const response = await api.get('/api/special-usage/records', {
    params: {
      ...paramsFromQuery(query),
      page: query.page,
      page_size: query.page_size,
    },
  })
  return unwrapApiResponse<SpecialUsageRecordsPage>(response.data)
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
  return unwrapApiResponse<SpecialUsageProfit>(response.data)
}

async function blobErrorMessage(blob: Blob): Promise<string> {
  try {
    const text = await blob.text()
    if (!text.trim()) return 'Export failed'
    try {
      const payload = JSON.parse(text) as { message?: unknown }
      return apiErrorMessage(payload.message, text)
    } catch {
      return text.slice(0, 500)
    }
  } catch {
    return 'Export failed'
  }
}

function filenameFromHeaders(
  contentDisposition: unknown,
  fallback: string
): string {
  if (typeof contentDisposition !== 'string') return fallback
  const utf8 = contentDisposition.match(/filename\*=UTF-8''([^;]+)/i)?.[1]
  const plain = contentDisposition.match(/filename="?([^";]+)"?/i)?.[1]
  const value = utf8 ?? plain
  if (!value) return fallback
  try {
    return decodeURIComponent(value)
  } catch {
    return value
  }
}

export async function downloadSpecialUsageExport(
  query: SpecialUsageQuery,
  format: SpecialUsageExportFormat = 'xlsx'
): Promise<void> {
  const response = await api.get('/api/special-usage/export', {
    params: { ...paramsFromQuery(query), format },
    responseType: 'blob',
    disableDuplicate: true,
    skipBusinessError: true,
  } as never)
  const contentType = String(
    response.headers?.['content-type'] ?? ''
  ).toLowerCase()
  const blob =
    response.data instanceof Blob ? response.data : new Blob([response.data])
  if (contentType.includes('json') || contentType.includes('text/html')) {
    throw new Error(await blobErrorMessage(blob))
  }

  // Older servers return CSV regardless of the requested format. Keep the
  // file usable by honoring the actual response type instead of naming CSV as
  // an XLSX workbook.
  const actualFormat: SpecialUsageExportFormat = contentType.includes('csv')
    ? 'csv'
    : format
  const fallback = `special-usage-${new Date().toISOString().slice(0, 10)}.${actualFormat}`
  const serverFilename = filenameFromHeaders(
    response.headers?.['content-disposition'],
    fallback
  )
  const filename = /\.[a-z0-9]+$/i.test(serverFilename)
    ? serverFilename
    : `${serverFilename}.${actualFormat}`
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
  window.setTimeout(() => URL.revokeObjectURL(url), 0)
}

export function downloadSpecialUsageCsv(
  query: SpecialUsageQuery
): Promise<void> {
  return downloadSpecialUsageExport(query, 'csv')
}
