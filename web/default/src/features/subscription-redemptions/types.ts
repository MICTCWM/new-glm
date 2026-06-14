/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { z } from 'zod'

// ============================================================================
// Subscription Redemption Schema & Types
// ============================================================================

export const subscriptionRedemptionSchema = z.object({
  id: z.number(),
  plan_id: z.number(),
  name: z.string(),
  key: z.string(),
  status: z.number(), // 1: enabled, 2: used, 3: disabled
  created_time: z.number(),
  expired_time: z.number(), // 0 for never expires
  used_user_id: z.number().optional(),
  used_time: z.number().optional(),
  created_by: z.number().optional(),
})

export type SubscriptionRedemption = z.infer<typeof subscriptionRedemptionSchema>

// ============================================================================
// Subscription Plan (for selection)
// ============================================================================

export interface SubscriptionPlanOption {
  id: number
  title: string
  price_amount: number
  currency: string
  duration_unit: string
  duration_value: number
  enabled: boolean
}

// ============================================================================
// API Request/Response Types
// ============================================================================

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export interface GetSubscriptionRedemptionsParams {
  p?: number
  page_size?: number
}

export interface GetSubscriptionRedemptionsResponse {
  success: boolean
  message?: string
  data?: {
    items: SubscriptionRedemption[]
    total: number
    page: number
    page_size: number
  }
}

export interface SearchSubscriptionRedemptionsParams {
  keyword?: string
  p?: number
  page_size?: number
}

export interface SubscriptionRedemptionFormData {
  id?: number
  name: string
  plan_id: number
  expired_time: number
  count?: number // Only for create
  status?: number // Only for status update
}

// ============================================================================
// Dialog Types
// ============================================================================

export type SubscriptionRedemptionsDialogType = 'create' | 'update' | 'delete' | 'view'