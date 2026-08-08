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
import type { TFunction } from 'i18next'
import type { SubscriptionPlan, PlanPayload } from '../types'

export function getPlanFormSchema(t: TFunction) {
  return z
    .object({
      title: z.string().min(1, t('Please enter plan title')),
      subtitle: z.string().optional(),
      price_amount: z.coerce.number().min(0, t('Please enter amount')),
      duration_unit: z.enum(['year', 'month', 'day', 'hour', 'custom']),
      duration_value: z.coerce.number().min(1),
      custom_seconds: z.coerce.number().min(0).optional(),
      quota_reset_period: z.enum([
        'never',
        'daily',
        'weekly',
        'monthly',
        'custom',
      ]),
      quota_reset_custom_seconds: z.coerce.number().min(0).optional(),
      enabled: z.boolean(),
      sort_order: z.coerce.number(),
      max_purchase_per_user: z.coerce.number().min(0),
      total_amount: z.coerce.number().min(0),
      weekly_amount_limit: z.coerce.number().min(0).default(0),
      special_quota_enabled: z.boolean().default(false),
      hourly_reset_hours: z.coerce.number().min(0).default(5),
      hourly_amount_limit: z.coerce.number().min(0).default(0),
      special_weekly_reset_weeks: z.coerce.number().min(0).default(1),
      special_weekly_amount_limit: z.coerce.number().min(0).default(0),
      upgrade_group: z.string().optional(),
      accessible_groups: z.array(z.string()).default([]),
      restricted_groups: z.array(z.string()).default([]),
      stripe_price_id: z.string().optional(),
      creem_product_id: z.string().optional(),
    })
    .superRefine((values, ctx) => {
      const accessibleGroups = new Set(values.accessible_groups)
      const overlappingGroup = values.restricted_groups.find((group) =>
        accessibleGroups.has(group)
      )
      if (overlappingGroup) {
        const message = t(
          'Accessible and restricted groups cannot contain the same group: {{group}}',
          { group: overlappingGroup }
        )
        ctx.addIssue({
          code: 'custom',
          path: ['accessible_groups'],
          message,
        })
        ctx.addIssue({
          code: 'custom',
          path: ['restricted_groups'],
          message,
        })
      }
      if (values.special_quota_enabled && values.hourly_reset_hours <= 0) {
        ctx.addIssue({
          code: 'custom',
          path: ['hourly_reset_hours'],
          message: t('Hourly reset period must be greater than 0'),
        })
      }
      if (values.special_quota_enabled && values.hourly_amount_limit <= 0) {
        ctx.addIssue({
          code: 'custom',
          path: ['hourly_amount_limit'],
          message: t('Hourly quota must be greater than 0'),
        })
      }
      if (
        values.special_quota_enabled &&
        ![1, 2].includes(values.special_weekly_reset_weeks)
      ) {
        ctx.addIssue({
          code: 'custom',
          path: ['special_weekly_reset_weeks'],
          message: t('Weekly reset period must be 1 or 2 weeks'),
        })
      }
      if (
        values.special_quota_enabled &&
        values.special_weekly_amount_limit <= 0
      ) {
        ctx.addIssue({
          code: 'custom',
          path: ['special_weekly_amount_limit'],
          message: t('Special weekly quota must be greater than 0'),
        })
      }
    })
}

export type PlanFormValues = z.infer<ReturnType<typeof getPlanFormSchema>>

export const PLAN_FORM_DEFAULTS: PlanFormValues = {
  title: '',
  subtitle: '',
  price_amount: 0,
  duration_unit: 'month',
  duration_value: 1,
  custom_seconds: 0,
  quota_reset_period: 'never',
  quota_reset_custom_seconds: 0,
  enabled: true,
  sort_order: 0,
  max_purchase_per_user: 0,
  total_amount: 0,
  weekly_amount_limit: 0,
  special_quota_enabled: false,
  hourly_reset_hours: 5,
  hourly_amount_limit: 0,
  special_weekly_reset_weeks: 1,
  special_weekly_amount_limit: 0,
  upgrade_group: '',
  accessible_groups: [],
  restricted_groups: [],
  stripe_price_id: '',
  creem_product_id: '',
}

export function planToFormValues(plan: SubscriptionPlan): PlanFormValues {
  return {
    title: plan.title || '',
    subtitle: plan.subtitle || '',
    price_amount: Number(plan.price_amount || 0),
    duration_unit: plan.duration_unit || 'month',
    duration_value: Number(plan.duration_value || 1),
    custom_seconds: Number(plan.custom_seconds || 0),
    quota_reset_period: plan.quota_reset_period || 'never',
    quota_reset_custom_seconds: Number(plan.quota_reset_custom_seconds || 0),
    enabled: plan.enabled !== false,
    sort_order: Number(plan.sort_order || 0),
    max_purchase_per_user: Number(plan.max_purchase_per_user || 0),
    total_amount: Number(plan.total_amount || 0),
    weekly_amount_limit: Number(plan.weekly_amount_limit ?? 0),
    special_quota_enabled: plan.special_quota_enabled === true,
    hourly_reset_hours: Number(plan.hourly_reset_hours || 5),
    hourly_amount_limit: Number(plan.hourly_amount_limit || 0),
    special_weekly_reset_weeks: Number(plan.special_weekly_reset_weeks || 1),
    special_weekly_amount_limit: Number(plan.special_weekly_amount_limit || 0),
    upgrade_group: plan.upgrade_group || '',
    accessible_groups: plan.accessible_groups || [],
    restricted_groups: plan.restricted_groups || [],
    stripe_price_id: plan.stripe_price_id || '',
    creem_product_id: plan.creem_product_id || '',
  }
}

export function formValuesToPlanPayload(values: PlanFormValues): PlanPayload {
  return {
    plan: {
      ...values,
      price_amount: Number(values.price_amount || 0),
      currency: 'USD',
      duration_value: Number(values.duration_value || 0),
      custom_seconds: Number(values.custom_seconds || 0),
      quota_reset_period: values.quota_reset_period || 'never',
      quota_reset_custom_seconds:
        values.quota_reset_period === 'custom'
          ? Number(values.quota_reset_custom_seconds || 0)
          : 0,
      sort_order: Number(values.sort_order || 0),
      max_purchase_per_user: Number(values.max_purchase_per_user || 0),
      total_amount: Number(values.total_amount || 0),
      weekly_amount_limit: Number(values.weekly_amount_limit || 0),
      special_quota_enabled: values.special_quota_enabled === true,
      hourly_reset_hours: Number(values.hourly_reset_hours || 0),
      hourly_amount_limit: Number(values.hourly_amount_limit || 0),
      special_weekly_reset_weeks: Number(
        values.special_weekly_reset_weeks || 0
      ),
      special_weekly_amount_limit: Number(
        values.special_weekly_amount_limit || 0
      ),
      upgrade_group: values.upgrade_group || '',
      accessible_groups: values.accessible_groups || [],
      restricted_groups: values.restricted_groups || [],
    },
  }
}
