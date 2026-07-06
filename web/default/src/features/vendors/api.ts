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
