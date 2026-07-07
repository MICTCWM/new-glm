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
import type { VendorMonitorSample } from '../types'

// ---------------------------------------------------------------------------
// 共用辅助函数
// ---------------------------------------------------------------------------

const MAX_SAMPLES = 30

type SampleStatus = 'success' | 'failed' | 'no_data'

function sampleStatus(sample: VendorMonitorSample): SampleStatus {
  if (!sample.has_data) return 'no_data'
  return sample.success ? 'success' : 'failed'
}

/** 将 Unix 秒级时间戳格式化为 HH:mm。 */
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
// VendorMonitorSparkline — 纯 CSS/Tailwind 迷你柱形图
// ---------------------------------------------------------------------------

export interface VendorMonitorSparklineProps {
  samples: VendorMonitorSample[]
  className?: string
}

/**
 * 供应商监控迷你柱形图，使用纯 Tailwind 渲染。设计为在供应商卡片中低调展示，
 * 悬停时完全显现。
 *
 * - 每个样本对应一根柱子（最多 30 根，按时间升序）。
 * - 柱体颜色反映状态：绿色=成功，红色=失败，灰色=无数据。
 * - 柱体高度按 use_time_ms 与窗口内最大值比例计算。
 * - use_time_ms === 0 与无数据桶使用固定短桩，避免整行塌陷。
 * - 60 秒自动刷新时，柱体高度变化通过 transition 平滑过渡。
 */
export function VendorMonitorSparkline(props: VendorMonitorSparklineProps) {
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
        heightPct = 6 // ~2px 最低可见
      } else if (maxMs <= 0) {
        heightPct = 50
      } else {
        // 钳制在 6%（最低可见）与 100%（最大）之间。
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
      aria-label={t('Vendor Monitor')}
    >
      {bars.map((bar, i) => (
        <div
          key={i}
          className={cn(
            'w-[3px] shrink-0 rounded-[1px] transition-all duration-300 ease-out',
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
// VendorMonitorBarChart — VChart 完整柱形图（用于 Sheet 展开）
// ---------------------------------------------------------------------------

export interface VendorMonitorBarChartProps {
  samples: VendorMonitorSample[]
  className?: string
}

/**
 * 供应商监控完整柱形图，使用 VChart 渲染。每根柱子代表一个 60 秒采样桶；
 * 柱体颜色编码状态（成功 / 失败 / 无数据），柱体高度编码响应时间（毫秒）。
 *
 * 关键设计决策：
 * - Y 轴以**秒**为单位（而非毫秒），避免出现 "7000 ms" 这样的大数字。
 * - 失败的柱子获得**固定可见高度**（轴上相当于 0.5s），这样红色柱子即使在
 *   缩放后也始终可见——与缩略图的行为一致（红色柱子始终显示）。
 * - 无数据柱子获得较小的固定高度（0.2s），以区别于失败柱子。
 * - 缩略图和此图表使用**相同的颜色方案**，因此缩略图中的红色柱子在展开后
 *   仍然是红色的。
 */
export function VendorMonitorBarChart(props: VendorMonitorBarChartProps) {
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
      data: [{ id: 'vendor-monitor', values: data }],
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
          key={`vendor-monitor-${resolvedTheme}`}
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
