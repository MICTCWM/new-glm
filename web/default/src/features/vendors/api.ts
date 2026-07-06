/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
// 复用 models feature 的 vendor API 实现，避免重复定义
// 返回类型对齐到 vendors feature 的类型定义
// （models.Vendor 缺少 supply_type 字段，但运行时后端返回的数据包含该字段）
import { api } from '@/lib/api'
import {
  createVendor as modelsCreateVendor,
  deleteVendor,
  getVendor as modelsGetVendor,
  getVendors as modelsGetVendors,
  updateVendor as modelsUpdateVendor,
} from '@/features/models/api'
import type {
  GetVendorResponse,
  GetVendorsResponse,
  Vendor,
  VendorMonitorSample,
} from './types'

export { deleteVendor }

export async function getVendors(params?: {
  p?: number
  page_size?: number
}): Promise<GetVendorsResponse> {
  return (await modelsGetVendors(params)) as unknown as GetVendorsResponse
}

export async function getVendor(id: number): Promise<GetVendorResponse> {
  return (await modelsGetVendor(id)) as unknown as GetVendorResponse
}

export async function createVendor(
  data: Partial<Vendor>
): Promise<{ success: boolean; message?: string; data?: Vendor }> {
  return (await modelsCreateVendor(data)) as unknown as {
    success: boolean
    message?: string
    data?: Vendor
  }
}

export async function updateVendor(
  data: Partial<Vendor> & { id: number }
): Promise<{ success: boolean; message?: string; data?: Vendor }> {
  return (await modelsUpdateVendor(data)) as unknown as {
    success: boolean
    message?: string
    data?: Vendor
  }
}

// ----------------------------------------------------------------------------
// Vendor Monitor APIs
// ----------------------------------------------------------------------------

// 获取单个供应商的监控样本（最近 ~30 分钟，按时间升序）
export async function getVendorMonitorSamples(
  vendorId: number
): Promise<VendorMonitorSample[]> {
  const res = await api.get(`/api/vendors/monitor/samples`, {
    params: { vendor_id: vendorId },
  })
  return res.data.data
}

// 一次性获取所有供应商的监控样本（供卡片网格使用）
// 返回 Record，key 为 vendor_id 字符串
export async function getAllVendorMonitorSamples(): Promise<
  Record<string, VendorMonitorSample[]>
> {
  const res = await api.get(`/api/vendors/monitor/samples/all`)
  return res.data.data
}
