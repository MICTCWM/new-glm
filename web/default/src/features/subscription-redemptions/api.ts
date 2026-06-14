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
import { api } from '@/lib/api'
import type {
  SubscriptionRedemption,
  ApiResponse,
  GetSubscriptionRedemptionsParams,
  GetSubscriptionRedemptionsResponse,
  SearchSubscriptionRedemptionsParams,
  SubscriptionRedemptionFormData,
} from './types'

// ============================================================================
// Subscription Redemption Code Management
// ============================================================================

// Get paginated subscription redemption codes list
export async function getSubscriptionRedemptions(
  params: GetSubscriptionRedemptionsParams = {}
): Promise<GetSubscriptionRedemptionsResponse> {
  const { p = 1, page_size = 10 } = params
  const res = await api.get(
    `/api/subscription-redemption/?p=${p}&page_size=${page_size}`
  )
  return res.data
}

// Search subscription redemption codes by keyword
export async function searchSubscriptionRedemptions(
  params: SearchSubscriptionRedemptionsParams
): Promise<GetSubscriptionRedemptionsResponse> {
  const { keyword = '', p = 1, page_size = 10 } = params
  const res = await api.get(
    `/api/subscription-redemption/search?keyword=${keyword}&p=${p}&page_size=${page_size}`
  )
  return res.data
}

// Get single subscription redemption code by ID
export async function getSubscriptionRedemption(
  id: number
): Promise<ApiResponse<SubscriptionRedemption>> {
  const res = await api.get(`/api/subscription-redemption/${id}`)
  return res.data
}

// Create subscription redemption code(s)
export async function createSubscriptionRedemption(
  data: SubscriptionRedemptionFormData
): Promise<ApiResponse<string[]>> {
  const res = await api.post('/api/subscription-redemption/', data)
  return res.data
}

// Update subscription redemption code
export async function updateSubscriptionRedemption(
  data: SubscriptionRedemptionFormData & { id: number }
): Promise<ApiResponse<SubscriptionRedemption>> {
  const res = await api.put('/api/subscription-redemption/', data)
  return res.data
}

// Update subscription redemption code status (enable/disable)
export async function updateSubscriptionRedemptionStatus(
  id: number,
  status: number
): Promise<ApiResponse<SubscriptionRedemption>> {
  const res = await api.put('/api/subscription-redemption/?status_only=true', {
    id,
    status,
  })
  return res.data
}

// Delete a single subscription redemption code
export async function deleteSubscriptionRedemption(
  id: number
): Promise<ApiResponse> {
  const res = await api.delete(`/api/subscription-redemption/${id}`)
  return res.data
}

// Delete invalid subscription redemption codes (used, disabled, expired)
export async function deleteInvalidSubscriptionRedemptions(): Promise<ApiResponse<number>> {
  const res = await api.delete('/api/subscription-redemption/invalid', {
    skipBusinessError: true,
  } as Record<string, unknown>)
  return res.data
}

export async function deleteUnusedSubscriptionRedemptions(): Promise<ApiResponse<number>> {
  const res = await api.delete('/api/subscription-redemption/unused', {
    skipBusinessError: true,
  } as Record<string, unknown>)
  return res.data
}

// Delete unused subscription redemption codes
export async function deleteUnusedSubscriptionRedemptions(): Promise<ApiResponse<number>> {
  const res = await api.delete('/api/subscription-redemption/unused')
  return res.data
}