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
import { useMemo } from 'react'
import { VChart } from '@visactor/react-vchart'
import { useTranslation } from 'react-i18next'
import { useChartTheme } from '@/lib/use-chart-theme'
import { cn } from '@/lib/utils'
import { VCHART_OPTION } from '@/lib/vchart'
import type { ModelMonitorSample } from '../types'

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

const MAX_SAMPLES = 30

type SampleStatus = 'success' | 'failed' | 'no_data'

function sampleStatus(sample: ModelMonitorSample): SampleStatus {
  if (!sample.has_data) return 'no_data'
  return sample.success ? 'success' : 'failed'
}

/** Format a Unix-seconds timestamp as HH:mm. */
function formatHHmm(unixSeconds: number): string {
  const d = new Date(unixSeconds * 1000)
  const hh = String(d.getHours()).padStart(2, '0')
  const mm = String(d.getMinutes()).padStart(2, '0')
  return `${hh}:${mm}`
}

/**
 * Format milliseconds to a human-readable time string.
 * If >= 1000 ms, show as seconds with one decimal place.
 * Otherwise show as integer ms.
 */
function formatLatency(ms: number): string {
  if (ms >= 1000) {
    return `${(ms / 1000).toFixed(1)}s`
  }
  return `${Math.round(ms)}ms`
}

// ---------------------------------------------------------------------------
// ModelMonitorSparkline — pure CSS/Tailwind mini bar chart
// ---------------------------------------------------------------------------

export interface ModelMonitorSparklineProps {
  samples: ModelMonitorSample[]
  className?: string
}

/**
 * Compact monitor sparkline rendered with plain Tailwind bars. Designed to sit
 * unobtrusively inside a model card and reveal itself on hover.
 *
 * - Each sample maps to one vertical bar (max 30 bars, oldest first).
 * - Bar colour reflects status: green=success, red=failed, grey=no data.
 * - Bar height is proportional to `use_time_ms` against the window's max.
 * - `use_time_ms === 0` and no-data buckets fall back to fixed stubs so the
 *   row never collapses to nothing.
 */
export function ModelMonitorSparkline(props: ModelMonitorSparklineProps) {
  const { t } = useTranslation()

  const bars = useMemo(() => {
    const slice = props.samples.slice(-MAX_SAMPLES)
    const maxMs = slice.reduce(
      (m, s) => (s.has_data && s.use_time_ms > m ? s.use_time_ms : m),
      0
    )
    return slice.map((s) => {
      const status = sampleStatus(s)
      let heightPct: number
      if (status === 'no_data') {
        heightPct = 12 // ~4px out of 32px
      } else if (s.use_time_ms <= 0) {
        heightPct = 6 // ~2px minimum visible
      } else if (maxMs <= 0) {
        heightPct = 50
      } else {
        // Clamp between 6% (min visible) and 100% (max).
        heightPct = Math.max(6, Math.min(100, (s.use_time_ms / maxMs) * 100))
      }
      return { status, heightPct }
    })
  }, [props.samples])

  if (bars.length === 0) {
    return (
      <span
        className={cn('text-muted-foreground text-[10px]', props.className)}
      >
        {t('No data')}
      </span>
    )
  }

  const colourFor = (status: SampleStatus) =>
    status === 'success'
      ? 'bg-emerald-500'
      : status === 'failed'
        ? 'bg-red-500'
        : 'bg-zinc-300 dark:bg-zinc-600'

  return (
    <div
      className={cn(
        'flex h-9 items-end gap-[1.5px]',
        'w-full min-w-[150px] max-w-[200px]',
        props.className
      )}
      role='img'
      aria-label={t('Channel Monitor')}
    >
      {bars.map((bar, i) => (
        <div
          key={i}
          className={cn(
            'w-[3px] shrink-0 rounded-[1px] transition-opacity',
            colourFor(bar.status)
          )}
          style={{ height: `${bar.heightPct}%` }}
          aria-hidden
        />
      ))}
    </div>
  )
}

// ---------------------------------------------------------------------------
// ModelMonitorBarChart — full VChart bar chart used in the expanded sheet
// ---------------------------------------------------------------------------

export interface ModelMonitorBarChartProps {
  samples: ModelMonitorSample[]
  className?: string
}

/**
 * Full-size monitor bar chart rendered with VChart. Each bar is one 60s sample
 * bucket; the bar's colour encodes its status (success / failed / no-data)
 * and the bar's height encodes the response time in milliseconds.
 *
 * Key design decisions:
 * - Y-axis shows latency in **seconds** (not ms) to avoid large numbers like "7000 ms".
 * - Failed bars get a **fixed visible height** (equivalent to 0.5s on the axis) so
 *   red bars are always visible even when the chart is zoomed in — matching the
 *   sparkline's behaviour where failed bars are always shown.
 * - No-data bars get a smaller fixed height (0.2s) to distinguish from failed.
 * - The sparkline and this chart use the **same colour scheme**, so red bars in
 *   the thumbnail remain red in the expanded view.
 */
export function ModelMonitorBarChart(props: ModelMonitorBarChartProps) {
  const { t } = useTranslation()
  const { resolvedTheme, themeReady } = useChartTheme()

  const spec = useMemo(() => {
    const slice = props.samples.slice(-MAX_SAMPLES)
    if (slice.length === 0) return null

    // Determine max value for reasonable Y-axis, but always include
    // enough room for the fixed failed/no-data heights.
    const maxMs = slice.reduce(
      (m, s) => (s.has_data && s.use_time_ms > m ? s.use_time_ms : m),
      0
    )
    // Y-axis max: at least 2s to show failed bars, but scale to real data if higher
    const yMax = Math.max(2000, maxMs * 1.15)

    const data = slice.map((s) => {
      const status = sampleStatus(s)
      // Convert ms to seconds for the Y-axis
      let renderValueSec: number
      if (status === 'no_data') {
        renderValueSec = 0.2 // fixed visible stub (200ms equivalent)
      } else if (!s.has_data || s.use_time_ms <= 0) {
        renderValueSec = 0.5 // fixed visible stub for failed/success-without-time
      } else {
        renderValueSec = s.use_time_ms / 1000
      }
      return {
        time: formatHHmm(s.created_at),
        value: renderValueSec,
        status,
        rawSec: s.has_data ? s.use_time_ms / 1000 : 0,
        rawMs: s.use_time_ms,
        hasData: s.has_data,
      }
    })

    return {
      type: 'bar' as const,
      data: [{ id: 'model-monitor', values: data }],
      xField: 'time',
      yField: 'value',
      colorField: 'status',
      color: {
        type: 'ordinal',
        domain: ['success', 'failed', 'no_data'],
        range: [
          '#10b981',
          '#ef4444',
          resolvedTheme === 'dark' ? '#52525b' : '#d4d4d8',
        ],
      },
      bar: {
        style: {
          cornerRadius: 2,
        },
      },
      legends: {
        visible: true,
        orient: 'top',
        item: {
          shape: { style: { symbolType: 'square' } },
          label: {
            formatMethod: (text: string | number) => {
              if (text === 'success') return t('Success')
              if (text === 'failed') return t('Failed')
              if (text === 'no_data') return t('No data')
              return text
            },
            style: { fontSize: 11, fill: 'currentColor' },
          },
        },
      },
      axes: [
        {
          orient: 'bottom',
          label: {
            style: { fill: 'currentColor', fontSize: 10 },
            autoLimit: true,
          },
          tick: { visible: false },
          title: {
            visible: true,
            text: t('Sample time'),
            style: { fill: 'currentColor', fontSize: 11 },
          },
        },
        {
          orient: 'left',
          label: {
            formatMethod: (val: number | string) => `${val}s`,
            style: { fill: 'currentColor', fontSize: 10 },
          },
          grid: { visible: true, style: { lineDash: [3, 3] } },
          // Force zero-based Y-axis so small bars don't get exaggerated
          zero: true,
          max: yMax / 1000,
          title: {
            visible: true,
            text: t('Response time'),
            style: { fill: 'currentColor', fontSize: 11 },
            autoRotate: false,
          },
        },
      ],
      tooltip: {
        mark: {
          title: { value: (d: { time: string }) => d.time },
          content: [
            {
              key: t('Response time'),
              value: (d: { rawMs: number; hasData: boolean; status: string }) =>
                !d.hasData
                  ? t('No data')
                  : d.status === 'failed'
                    ? `${t('Failed')}`
                    : formatLatency(d.rawMs),
            },
            {
              key: t('Status'),
              value: (d: { status: SampleStatus }) => {
                if (d.status === 'success') return t('Success')
                if (d.status === 'failed') return t('Failed')
                return t('No data')
              },
            },
          ],
        },
      },
    }
  }, [props.samples, t, resolvedTheme])

  if (props.samples.length === 0) {
    return (
      <div
        className={cn(
          'text-muted-foreground flex h-48 items-center justify-center rounded-lg border text-xs',
          props.className
        )}
      >
        {t('No data')}
      </div>
    )
  }

  return (
    <div className={cn('h-56 w-full sm:h-60', props.className)}>
      {themeReady && spec && (
        <VChart
          key={`model-monitor-${resolvedTheme}`}
          spec={{
            ...spec,
            theme: resolvedTheme === 'dark' ? 'dark' : 'light',
            background: 'transparent',
          }}
          option={VCHART_OPTION}
        />
      )}
    </div>
  )
}
