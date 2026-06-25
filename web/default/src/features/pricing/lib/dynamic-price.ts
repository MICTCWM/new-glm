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
import { formatBillingCurrencyFromUSD } from '@/lib/currency'
import { TOKEN_UNIT_DIVISORS } from '../constants'
import type { PricingModel, TokenUnit } from '../types'
import {
  BILLING_PRICING_VARS,
  parseTiersFromExpr,
  splitBillingExprAndRequestRules,
  tryParseRequestRuleExpr,
  type BillingVar,
  type ParsedTier,
  type TierCondition,
} from './billing-expr'

type DynamicPriceOptions = {
  tokenUnit: TokenUnit
  showRechargePrice?: boolean
  priceRate?: number
  usdExchangeRate?: number
  groupRatioMultiplier?: number
}

export type DynamicPriceEntry = {
  key: string
  field: string
  label: string
  shortLabel: string
  value: number
  formatted: string
  variable: BillingVar
}

export type DynamicPricingSummary = {
  tiers: ParsedTier[]
  tier: ParsedTier | null
  tierCount: number
  hasRequestRules: boolean
  isSpecialExpression: boolean
  rawExpression: string
  entries: DynamicPriceEntry[]
  primaryEntries: DynamicPriceEntry[]
  secondaryEntries: DynamicPriceEntry[]
}

const PRIMARY_DYNAMIC_FIELDS = new Set(['inputPrice', 'outputPrice'])

export function isDynamicPricingModel(model: PricingModel): boolean {
  return model.billing_mode === 'tiered_expr' && Boolean(model.billing_expr)
}

export function getDynamicDisplayGroupRatio(model: PricingModel): number {
  const groups = Array.isArray(model.enable_groups) ? model.enable_groups : []
  const ratios = model.group_ratio || {}
  if (groups.length === 0) return 1

  let minRatio = Number.POSITIVE_INFINITY
  for (const group of groups) {
    const ratio = ratios[group]
    if (ratio !== undefined && ratio < minRatio) {
      minRatio = ratio
    }
  }

  return minRatio === Number.POSITIVE_INFINITY ? 1 : minRatio
}

function applyRechargeRate(
  price: number,
  showWithRecharge: boolean,
  priceRate: number,
  usdExchangeRate: number
): number {
  if (!showWithRecharge) return price
  return (price * priceRate) / usdExchangeRate
}

export function formatDynamicUnitPrice(
  valuePerMillionTokens: number,
  options: DynamicPriceOptions
): string {
  const groupRatio = options.groupRatioMultiplier ?? 1
  const priceRate = options.priceRate ?? 1
  const usdExchangeRate = options.usdExchangeRate ?? 1
  const priceUSD =
    (valuePerMillionTokens * groupRatio) /
    TOKEN_UNIT_DIVISORS[options.tokenUnit]
  const displayPrice = applyRechargeRate(
    priceUSD,
    options.showRechargePrice ?? false,
    priceRate,
    usdExchangeRate
  )

  return formatBillingCurrencyFromUSD(displayPrice, {
    digitsLarge: 4,
    digitsSmall: 6,
    abbreviate: false,
  })
}

export function getDynamicPricingTiers(model: PricingModel): ParsedTier[] {
  if (!isDynamicPricingModel(model)) return []
  const { billingExpr } = splitBillingExprAndRequestRules(
    model.billing_expr || ''
  )
  return parseTiersFromExpr(billingExpr)
}

export function hasDynamicRequestRules(model: PricingModel): boolean {
  if (!isDynamicPricingModel(model)) return false
  const { requestRuleExpr } = splitBillingExprAndRequestRules(
    model.billing_expr || ''
  )
  return Boolean(tryParseRequestRuleExpr(requestRuleExpr || '')?.length)
}

export function getDynamicPriceEntries(
  tier: ParsedTier | null,
  options: DynamicPriceOptions
): DynamicPriceEntry[] {
  if (!tier) return []

  return BILLING_PRICING_VARS.flatMap((variable) => {
    if (!variable.field) return []
    const value = Number(tier[variable.field])
    if (!Number.isFinite(value) || value <= 0) return []

    return [
      {
        key: variable.key,
        field: variable.field,
        label: variable.label,
        shortLabel: variable.shortLabel,
        value,
        formatted: formatDynamicUnitPrice(value, options),
        variable,
      },
    ]
  }).sort((a, b) => {
    const aPrimary = PRIMARY_DYNAMIC_FIELDS.has(a.field)
    const bPrimary = PRIMARY_DYNAMIC_FIELDS.has(b.field)
    if (aPrimary !== bPrimary) return aPrimary ? -1 : 1
    return 0
  })
}

export function getDynamicPricingSummary(
  model: PricingModel,
  options: DynamicPriceOptions
): DynamicPricingSummary | null {
  if (!isDynamicPricingModel(model)) return null

  const tiers = getDynamicPricingTiers(model)
  const tier = tiers[0] || null
  const entries = getDynamicPriceEntries(tier, options)
  const rawExpression = model.billing_expr || ''

  return {
    tiers,
    tier,
    tierCount: tiers.length,
    hasRequestRules: hasDynamicRequestRules(model),
    isSpecialExpression: rawExpression.trim().length > 0 && tiers.length === 0,
    rawExpression,
    entries,
    primaryEntries: entries.filter((entry) =>
      PRIMARY_DYNAMIC_FIELDS.has(entry.field)
    ),
    secondaryEntries: entries.filter(
      (entry) => !PRIMARY_DYNAMIC_FIELDS.has(entry.field)
    ),
  }
}

export type PriceRange = {
  minPrice: number
  maxPrice: number
  minFormatted: string
  maxFormatted: string
}

export type DynamicPricingPriceRange = {
  unitPrice?: PriceRange
  fixedPrice?: PriceRange
  tierCount: number
}

export function formatFixedPriceValue(
  value: number,
  options: DynamicPriceOptions
): string {
  const groupRatio = options.groupRatioMultiplier ?? 1
  const priceRate = options.priceRate ?? 1
  const usdExchangeRate = options.usdExchangeRate ?? 1
  const priceUSD = value * groupRatio
  const displayPrice = applyRechargeRate(
    priceUSD,
    options.showRechargePrice ?? false,
    priceRate,
    usdExchangeRate
  )

  return formatBillingCurrencyFromUSD(displayPrice, {
    digitsLarge: 4,
    digitsSmall: 6,
    abbreviate: false,
  })
}

export function getDynamicPricingPriceRange(
  model: PricingModel,
  options: DynamicPriceOptions
): DynamicPricingPriceRange | null {
  if (!isDynamicPricingModel(model)) return null

  const tiers = getDynamicPricingTiers(model)
  if (tiers.length === 0) return null

  let minUnitPrice = Number.POSITIVE_INFINITY
  let maxUnitPrice = Number.NEGATIVE_INFINITY
  let minFixedPrice = Number.POSITIVE_INFINITY
  let maxFixedPrice = Number.NEGATIVE_INFINITY
  let hasUnitPrice = false
  let hasFixedPrice = false

  for (const tier of tiers) {
    const entries = getDynamicPriceEntries(tier, options)
    if (entries.length > 0) {
      hasUnitPrice = true
      for (const entry of entries) {
        if (entry.value < minUnitPrice) minUnitPrice = entry.value
        if (entry.value > maxUnitPrice) maxUnitPrice = entry.value
      }
    }

    if (tier.fixed_price != null && tier.fixed_price > 0) {
      hasFixedPrice = true
      const fp = tier.fixed_price
      if (fp < minFixedPrice) minFixedPrice = fp
      if (fp > maxFixedPrice) maxFixedPrice = fp
    }
  }

  if (!hasUnitPrice && !hasFixedPrice) {
    return null
  }

  const result: DynamicPricingPriceRange = {
    tierCount: tiers.length,
  }

  if (hasUnitPrice && Number.isFinite(minUnitPrice) && Number.isFinite(maxUnitPrice)) {
    result.unitPrice = {
      minPrice: minUnitPrice,
      maxPrice: maxUnitPrice,
      minFormatted: formatDynamicUnitPrice(minUnitPrice, options),
      maxFormatted: formatDynamicUnitPrice(maxUnitPrice, options),
    }
  }

  if (hasFixedPrice && Number.isFinite(minFixedPrice) && Number.isFinite(maxFixedPrice)) {
    result.fixedPrice = {
      minPrice: minFixedPrice,
      maxPrice: maxFixedPrice,
      minFormatted: formatFixedPriceValue(minFixedPrice, options),
      maxFormatted: formatFixedPriceValue(maxFixedPrice, options),
    }
  }

  return result
}

function formatTokenValue(value: number): string {
  if (value >= 1_000_000) {
    return `${(value / 1_000_000).toFixed(value % 1_000_000 === 0 ? 0 : 1)}M`
  }
  if (value >= 1_000) {
    return `${(value / 1_000).toFixed(value % 1_000 === 0 ? 0 : 1)}K`
  }
  return String(value)
}

export type TierConditionPart = {
  op: string
  opLabel: string
  value: number
  valueLabel: string
  var: 'p' | 'c' | 'len'
}

export function getTierConditionParts(
  conditions: TierCondition[]
): TierConditionPart[] {
  return conditions.map((cond) => ({
    op: cond.op,
    opLabel: cond.op === '<=' ? '≤' : cond.op === '>=' ? '≥' : cond.op,
    value: cond.value,
    valueLabel: formatTokenValue(cond.value),
    var: cond.var,
  }))
}

export function getDynamicPricingTierBreakpoints(
  model: PricingModel
): { label: string; value: number; var: 'p' | 'c' | 'len' }[] {
  if (!isDynamicPricingModel(model)) return []

  const tiers = getDynamicPricingTiers(model)
  const breakpoints = new Map<string, { label: string; value: number; var: 'p' | 'c' | 'len' }>()

  for (const tier of tiers) {
    for (const cond of tier.conditions) {
      const key = `${cond.var}-${cond.value}`
      if (!breakpoints.has(key)) {
        breakpoints.set(key, {
          label: formatTokenValue(cond.value),
          value: cond.value,
          var: cond.var,
        })
      }
    }
  }

  return Array.from(breakpoints.values()).sort((a, b) => a.value - b.value)
}
