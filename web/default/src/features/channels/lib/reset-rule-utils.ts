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

// 5 supported reset rule types
export const RULE_TYPES = [
  'daily',
  'weekly',
  'monthly',
  'custom_interval',
  'specific_time',
] as const

export type RuleType = (typeof RULE_TYPES)[number]

export const WEEKDAYS = [0, 1, 2, 3, 4, 5, 6] as const

/**
 * Build the rule_config JSON string for a given rule type and form values.
 */
export function buildRuleConfig(
  ruleType: string,
  values: Record<string, unknown>
): string {
  let config: Record<string, unknown> = {}
  switch (ruleType) {
    case 'daily':
      config = { hour: values.hour ?? 0, minute: values.minute ?? 0 }
      break
    case 'weekly':
      config = {
        weekday: values.weekday ?? 0,
        hour: values.hour ?? 0,
        minute: values.minute ?? 0,
      }
      break
    case 'monthly':
      config = {
        day_of_month: values.day_of_month ?? 1,
        hour: values.hour ?? 0,
        minute: values.minute ?? 0,
      }
      break
    case 'custom_interval':
      config = { interval_seconds: values.interval_seconds ?? 3600 }
      break
    case 'specific_time':
      // specific_time is stored as a unix timestamp (seconds).
      // Form values hold it as milliseconds (from date input).
      config = {
        specific_time: values.specific_time
          ? Math.floor(Number(values.specific_time) / 1000)
          : 0,
      }
      break
    default:
      return JSON.stringify({})
  }
  return JSON.stringify(config)
}

/**
 * Parse a rule_config string back into form-friendly values.
 * For specific_time, the value is converted from seconds to milliseconds
 * so it can be consumed by a date/time input.
 */
export function parseRuleConfig(
  ruleType: string,
  ruleConfigStr: string
): Record<string, unknown> {
  if (!ruleConfigStr) return {}
  try {
    const parsed =
      typeof ruleConfigStr === 'string'
        ? JSON.parse(ruleConfigStr)
        : ruleConfigStr
    if (!parsed || typeof parsed !== 'object') return {}
    if (ruleType === 'specific_time' && parsed.specific_time) {
      return { ...parsed, specific_time: parsed.specific_time * 1000 }
    }
    return parsed
  } catch {
    return {}
  }
}

/**
 * Render a human-readable summary of a rule's configuration.
 */
export function renderRuleConfigSummary(
  ruleType: string,
  ruleConfigStr: string
): string {
  if (!ruleConfigStr) return '-'
  const config = parseRuleConfig(ruleType, ruleConfigStr)
  switch (ruleType) {
    case 'daily': {
      const h = String(config.hour ?? 0).padStart(2, '0')
      const m = String(config.minute ?? 0).padStart(2, '0')
      return `${h}:${m}`
    }
    case 'weekly': {
      const weekdayLabels = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']
      const w = weekdayLabels[Number(config.weekday ?? 0)] ?? 'Sun'
      const h = String(config.hour ?? 0).padStart(2, '0')
      const m = String(config.minute ?? 0).padStart(2, '0')
      return `${w} ${h}:${m}`
    }
    case 'monthly': {
      const d = config.day_of_month ?? 1
      const h = String(config.hour ?? 0).padStart(2, '0')
      const m = String(config.minute ?? 0).padStart(2, '0')
      return `Day ${d} ${h}:${m}`
    }
    case 'custom_interval': {
      return `${config.interval_seconds ?? 3600}s`
    }
    case 'specific_time': {
      const ms = Number(config.specific_time ?? 0)
      if (!ms) return '-'
      return new Date(ms).toLocaleString()
    }
    default:
      return '-'
  }
}
