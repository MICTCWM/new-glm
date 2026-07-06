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
import { z } from 'zod'

/**
 * Supply type for a vendor.
 * 0 = 自有供应 (self-supplied, non-third-party)
 * 1 = 合作供应 (partner-supplied, third-party)
 */
export const SUPPLY_TYPE = {
  SELF: 0,
  PARTNER: 1,
} as const

export type SupplyType = (typeof SUPPLY_TYPE)[keyof typeof SUPPLY_TYPE]

/**
 * Vendor entity from API
 */
export interface Vendor {
  id: number
  name: string
  description?: string
  icon?: string
  status: number
  /** 0=自有供应, 1=合作供应 */
  supply_type: number
  created_time?: number
  updated_time?: number
}

/**
 * Get vendors response
 */
export interface GetVendorsResponse {
  success: boolean
  message?: string
  data?: {
    items: Vendor[]
    total: number
    page: number
    page_size: number
  }
}

/**
 * Get vendor response
 */
export interface GetVendorResponse {
  success: boolean
  message?: string
  data?: Vendor
}

/**
 * Vendor form schema
 */
export const vendorFormSchema = z.object({
  id: z.number().optional(),
  name: z.string().min(1, 'Vendor name is required'),
  description: z.string().default(''),
  icon: z.string().default(''),
  status: z.number().default(1),
  supply_type: z
    .union([z.literal(0), z.literal(1)])
    .default(SUPPLY_TYPE.SELF),
})

export type VendorFormValues = z.infer<typeof vendorFormSchema>
