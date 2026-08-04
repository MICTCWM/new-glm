import { useEffect, useMemo, useRef } from 'react'
import * as echarts from 'echarts'
import type { ECharts, EChartsOption } from 'echarts'
import { useTranslation } from 'react-i18next'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import type { SpecialUsageOverview, SpecialUsageTreeNode } from '../types'

type EChartProps = {
  option: EChartsOption
  empty?: boolean
  emptyLabel: string
  className?: string
}

function EChart(props: EChartProps) {
  const elementRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<ECharts | null>(null)

  useEffect(() => {
    if (!elementRef.current) return
    const chart = echarts.init(elementRef.current)
    chartRef.current = chart
    const observer = new ResizeObserver(() => chart.resize())
    observer.observe(elementRef.current)
    return () => {
      observer.disconnect()
      chart.dispose()
      chartRef.current = null
    }
  }, [])

  useEffect(() => {
    if (!props.empty)
      chartRef.current?.setOption(props.option, { notMerge: true })
  }, [props.empty, props.option])

  return (
    <div className={`relative ${props.className ?? 'h-72 w-full'}`}>
      <div ref={elementRef} className='h-full w-full' />
      {props.empty && (
        <div className='bg-card/90 text-muted-foreground absolute inset-0 flex items-center justify-center text-sm'>
          {props.emptyLabel}
        </div>
      )}
    </div>
  )
}

const chartColors = [
  '#0f766e',
  '#2563eb',
  '#d97706',
  '#9333ea',
  '#dc2626',
  '#0891b2',
  '#65a30d',
  '#c026d3',
]

function currency(value: number): string {
  return `$${value.toFixed(4)}`
}

function tokens(value: number): string {
  return Intl.NumberFormat().format(Math.round(value))
}

function chartTheme() {
  const dark = document.documentElement.classList.contains('dark')
  return {
    text: dark ? '#d4d4d8' : '#3f3f46',
    muted: dark ? '#71717a' : '#a1a1aa',
    grid: dark ? '#27272a' : '#e4e4e7',
  }
}

function baseOption(): EChartsOption {
  const theme = chartTheme()
  return {
    animationDuration: 350,
    textStyle: { color: theme.text },
    tooltip: { trigger: 'item' },
    color: chartColors,
  }
}

function pieOption(
  data: Array<{ name: string; value: number }>,
  formatter: (value: number) => string
): EChartsOption {
  return {
    ...baseOption(),
    tooltip: {
      trigger: 'item',
      formatter: (params) => {
        const item = (Array.isArray(params) ? params[0] : params) as {
          name?: string
          value?: number
          percent?: number
        }
        return `${item.name ?? ''}<br/>${formatter(Number(item.value ?? 0))} (${Number(item.percent ?? 0).toFixed(1)}%)`
      },
    },
    legend: {
      type: 'scroll',
      bottom: 0,
      textStyle: { color: chartTheme().text },
    },
    series: [
      { type: 'pie', radius: ['42%', '72%'], avoidLabelOverlap: true, data },
    ],
  }
}

function treeValue(data: SpecialUsageTreeNode[]): SpecialUsageTreeNode[] {
  return data.map((group) => ({
    ...group,
    children: group.children?.map((channel) => ({ ...channel })),
  }))
}

export function SpecialUsageCharts(props: { overview: SpecialUsageOverview }) {
  const { t } = useTranslation()
  const overview = props.overview
  const theme = chartTheme()

  const groupCostOption = useMemo(
    () =>
      pieOption(
        overview.group_costs.map((item) => ({
          name: item.name,
          value: item.upstream_cost_usd,
        })),
        currency
      ),
    [overview.group_costs]
  )
  const modelTokenOption = useMemo(
    () =>
      pieOption(
        overview.model_tokens.map((item) => ({
          name: item.name,
          value: item.input_tokens + item.output_tokens,
        })),
        tokens
      ),
    [overview.model_tokens]
  )
  const inputOutputOption = useMemo(
    () =>
      pieOption(
        [
          { name: t('Input tokens'), value: overview.totals.input_tokens },
          { name: t('Output tokens'), value: overview.totals.output_tokens },
        ],
        tokens
      ),
    [overview.totals.input_tokens, overview.totals.output_tokens, t]
  )
  const trendOption = useMemo<EChartsOption>(() => {
    const labels = overview.series.map((item) =>
      new Date(item.time * 1000).toLocaleString([], {
        month: 'numeric',
        day: 'numeric',
        hour: '2-digit',
      })
    )
    return {
      ...baseOption(),
      tooltip: { trigger: 'axis' },
      legend: { top: 0, textStyle: { color: theme.text } },
      grid: { left: 48, right: 48, top: 36, bottom: 28 },
      xAxis: {
        type: 'category',
        data: labels,
        axisLabel: { color: theme.muted },
      },
      yAxis: [
        {
          type: 'value',
          name: t('USD'),
          axisLabel: { color: theme.muted },
          splitLine: { lineStyle: { color: theme.grid } },
        },
        {
          type: 'value',
          name: t('Requests'),
          axisLabel: { color: theme.muted },
          splitLine: { show: false },
        },
      ],
      series: [
        {
          name: t('Upstream cost'),
          type: 'line',
          smooth: true,
          data: overview.series.map((item) => item.upstream_cost_usd),
          yAxisIndex: 0,
          areaStyle: { opacity: 0.08 },
        },
        {
          name: t('Requests'),
          type: 'line',
          smooth: true,
          data: overview.series.map((item) => item.request_count),
          yAxisIndex: 1,
        },
      ],
    }
  }, [overview.series, t, theme.grid, theme.muted, theme.text])
  const channelCostOption = useMemo<EChartsOption>(
    () => ({
      ...baseOption(),
      tooltip: {
        trigger: 'axis',
        axisPointer: { type: 'shadow' },
        valueFormatter: (value) => currency(Number(value)),
      },
      grid: { left: 48, right: 18, top: 20, bottom: 72 },
      xAxis: {
        type: 'category',
        data: overview.channels.map((item) => item.channel_name),
        axisLabel: {
          color: theme.muted,
          rotate: overview.channels.length > 5 ? 28 : 0,
        },
      },
      yAxis: {
        type: 'value',
        name: t('USD per request'),
        axisLabel: { color: theme.muted },
        splitLine: { lineStyle: { color: theme.grid } },
      },
      series: [
        {
          type: 'bar',
          data: overview.channels.map((item) => ({
            value: item.average_cost_usd,
            itemStyle: { color: item.anomaly ? '#dc2626' : undefined },
          })),
        },
      ],
    }),
    [overview.channels, t, theme.grid, theme.muted]
  )
  const profitOption = useMemo<EChartsOption>(
    () => ({
      ...baseOption(),
      tooltip: {
        trigger: 'axis',
        axisPointer: { type: 'shadow' },
        valueFormatter: (value) => currency(Number(value)),
      },
      legend: { top: 0, textStyle: { color: theme.text } },
      grid: { left: 48, right: 18, top: 36, bottom: 60 },
      xAxis: {
        type: 'category',
        data: overview.group_profit.map((item) => item.name),
        axisLabel: {
          color: theme.muted,
          rotate: overview.group_profit.length > 5 ? 28 : 0,
        },
      },
      yAxis: {
        type: 'value',
        name: t('USD'),
        axisLabel: { color: theme.muted },
        splitLine: { lineStyle: { color: theme.grid } },
      },
      series: [
        {
          name: t('User charges'),
          type: 'bar',
          data: overview.group_profit.map((item) => item.user_charge_usd),
        },
        {
          name: t('Upstream cost'),
          type: 'bar',
          data: overview.group_profit.map((item) => item.upstream_cost_usd),
        },
      ],
    }),
    [overview.group_profit, t, theme.grid, theme.muted, theme.text]
  )
  const treeOption = useMemo<EChartsOption>(
    () => ({
      ...baseOption(),
      tooltip: {
        trigger: 'item',
        formatter: (params) =>
          `${String((params as { name?: string }).name ?? '')}<br/>${currency(Number((params as { value?: number }).value ?? 0))}`,
      },
      series: [
        {
          type: 'treemap',
          roam: false,
          nodeClick: false,
          breadcrumb: { show: false },
          label: { show: true, color: '#fff' },
          data: treeValue(overview.channel_cost_tree),
        },
      ],
    }),
    [overview.channel_cost_tree]
  )

  const charts = [
    { title: t('Group cost share'), option: groupCostOption },
    { title: t('Model token share'), option: modelTokenOption },
    { title: t('Input / output tokens'), option: inputOutputOption },
    { title: t('Cost and request trend'), option: trendOption, wide: true },
    { title: t('Average channel cost'), option: channelCostOption },
    { title: t('Group profit comparison'), option: profitOption },
    { title: t('Channel cost distribution'), option: treeOption, wide: true },
  ]

  return (
    <div className='grid gap-4 lg:grid-cols-2'>
      {charts.map((chart) => (
        <Card
          key={chart.title}
          className={chart.wide ? 'lg:col-span-2' : undefined}
        >
          <CardHeader className='pb-0'>
            <CardTitle className='text-sm'>{chart.title}</CardTitle>
          </CardHeader>
          <CardContent>
            <EChart option={chart.option} />
          </CardContent>
        </Card>
      ))}
    </div>
  )
}
