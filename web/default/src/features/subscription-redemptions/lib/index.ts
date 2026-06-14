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
import { SUBSCRIPTION_REDEMPTION_STATUS } from '../constants'
import type { SubscriptionRedemptionFormData, SubscriptionRedemption } from '../types'

// ============================================================================
// Form Schema & Defaults
// ============================================================================

export const getSubscriptionRedemptionFormSchema = (t: (key: string) => string) =>
  z.object({
    name: z
      .string()
      .min(1, t('Name is required'))
      .max(20, t('Name must be less than 20 characters')),
    plan_id: z.number().min(1, t('Please select a subscription plan')),
    count: z.number().min(1).max(100).default(1),
    expired_time: z.number().optional(),
  })

export type SubscriptionRedemptionFormValues = z.infer<
  ReturnType<typeof getSubscriptionRedemptionFormSchema>
>

export const SUBSCRIPTION_REDEMPTION_FORM_DEFAULT_VALUES: SubscriptionRedemptionFormValues = {
  name: '',
  plan_id: 0,
  count: 1,
  expired_time: 0,
}

// ============================================================================
// Data Transformations
// ============================================================================

export function transformFormDataToPayload(
  data: SubscriptionRedemptionFormValues
): SubscriptionRedemptionFormData {
  return {
    name: data.name,
    plan_id: data.plan_id,
    count: data.count || 1,
    expired_time: data.expired_time || 0,
  }
}

export function transformRedemptionToFormDefaults(
  redemption: SubscriptionRedemption
): SubscriptionRedemptionFormValues {
  return {
    name: redemption.name,
    plan_id: redemption.plan_id,
    expired_time: redemption.expired_time,
  }
}

// ============================================================================
// Helper Functions
// ============================================================================

/**
 * Check if a subscription redemption code is expired
 * @param expiredTime - Unix timestamp in seconds, 0 means never expires
 * @param status - Current status of the redemption code
 * @returns true if the code is expired
 */
export function isSubscriptionRedemptionExpired(
  expiredTime: number,
  status: number
): boolean {
  // Only enabled codes can be expired
  if (status !== SUBSCRIPTION_REDEMPTION_STATUS.ENABLED) {
    return false
  }
  // 0 means never expires
  if (expiredTime === 0) {
    return false
  }
  // Compare with current time (convert to seconds)
  return expiredTime < Date.now() / 1000
}