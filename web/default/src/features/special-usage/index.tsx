import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  BarChart3,
  Check,
  Download,
  RefreshCw,
  Save,
  Settings2,
  TriangleAlert,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import {
  downloadSpecialUsageExport,
  getSpecialUsageForecast,
  getSpecialUsageMetadata,
  getSpecialUsageOverview,
  getSpecialUsageProfit,
  getSpecialUsageRecords,
  saveSpecialUsageConfig,
  type SpecialUsageExportFormat,
  type SpecialUsageQuery,
} from './api'
import { SpecialUsageCharts } from './components/special-usage-charts'
import type {
  SpecialUsageChannel,
  SpecialUsageChannelStat,
  SpecialUsageConfig,
  SpecialUsageDateRange,
  SpecialUsageForecast,
} from './types'

type PeriodKey = 'today' | '24h' | '7d' | '30d' | 'custom'

const nowSeconds = () => Math.floor(Date.now() / 1000)

function dateInputSeconds(value: string, endOfDay = false): number {
  if (!value) return Number.NaN
  const date = new Date(`${value}T00:00:00`)
  const timestamp = date.getTime()
  return Number.isFinite(timestamp)
    ? Math.floor(timestamp / 1000) + (endOfDay ? 86400 : 0)
    : Number.NaN
}

function periodRange(
  period: PeriodKey,
  customStart: string,
  customEnd: string
): SpecialUsageDateRange {
  const end = nowSeconds()
  if (period === 'today') {
    const current = new Date()
    current.setHours(0, 0, 0, 0)
    return { start: Math.floor(current.getTime() / 1000), end }
  }
  if (period === '24h') return { start: end - 24 * 3600, end }
  if (period === '7d') return { start: end - 7 * 86400, end }
  if (period === '30d') return { start: end - 30 * 86400, end }
  return {
    start: dateInputSeconds(customStart),
    end: dateInputSeconds(customEnd, true),
  }
}

function formatTokens(value: number): string {
  return Intl.NumberFormat().format(Math.round(value))
}

function formatUSD(value: number, digits = 4): string {
  return `$${value.toFixed(digits)}`
}

function formatDate(timestamp: number): string {
  if (!timestamp) return '-'
  return new Date(timestamp * 1000).toLocaleString()
}

function ToggleList(props: {
  title: string
  values: string[]
  selected: string[]
  onChange: (values: string[]) => void
  disabled?: boolean
}) {
  const { t } = useTranslation()
  const allSelected =
    props.values.length > 0 && props.selected.length === props.values.length
  const toggle = (value: string) => {
    if (props.selected.includes(value)) {
      props.onChange(props.selected.filter((item) => item !== value))
    } else {
      props.onChange([...props.selected, value])
    }
  }
  return (
    <div className='space-y-2'>
      <div className='flex items-center justify-between gap-2'>
        <div className='text-sm font-medium'>{props.title}</div>
        <button
          type='button'
          disabled={props.disabled || props.values.length === 0}
          className='text-primary text-xs hover:underline disabled:opacity-50'
          onClick={() => props.onChange(allSelected ? [] : props.values)}
        >
          {allSelected ? t('Clear all') : t('Select all')}
        </button>
      </div>
      <div className='bg-muted/30 grid max-h-36 gap-1 overflow-y-auto rounded-lg border p-2 sm:grid-cols-2 lg:grid-cols-3'>
        {props.values.map((value) => (
          <label
            key={value}
            className='hover:bg-muted flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 text-xs'
          >
            <Checkbox
              checked={props.selected.includes(value)}
              disabled={props.disabled}
              onCheckedChange={() => toggle(value)}
            />
            <span className='truncate' title={value}>
              {value}
            </span>
          </label>
        ))}
        {props.values.length === 0 && (
          <span className='text-muted-foreground px-2 py-3 text-xs'>
            {t('No options')}
          </span>
        )}
      </div>
    </div>
  )
}

function MetricCard(props: { label: string; value: string; detail?: string }) {
  return (
    <Card size='sm'>
      <CardContent className='space-y-1'>
        <div className='text-muted-foreground text-xs'>{props.label}</div>
        <div className='text-xl font-semibold tracking-tight'>
          {props.value}
        </div>
        {props.detail && (
          <div className='text-muted-foreground text-[11px]'>
            {props.detail}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function ChannelConfigTable(props: {
  channels: SpecialUsageChannel[]
  selectedGroups: string[]
  selectedModels: string[]
  channelStats?: SpecialUsageChannelStat[]
  multipliers: Record<string, number>
  onMultiplierChange: (channelID: number, value: number) => void
  onGroupMultiplier: (group: string, value: number) => void
}) {
  const { t } = useTranslation()
  const visibleChannels = props.channels.filter(
    (channel) =>
      channel.groups.some((group) => props.selectedGroups.includes(group)) &&
      channel.models.some((model) => props.selectedModels.includes(model))
  )
  const [batchGroup, setBatchGroup] = useState(props.selectedGroups[0] ?? '')
  const [batchValue, setBatchValue] = useState('1')
  const activeBatchGroup = props.selectedGroups.includes(batchGroup)
    ? batchGroup
    : (props.selectedGroups[0] ?? '')
  return (
    <div className='space-y-3'>
      <div className='bg-muted/20 flex flex-col gap-2 rounded-lg border p-3 sm:flex-row sm:items-end'>
        <label className='flex-1 space-y-1'>
          <span className='text-muted-foreground text-xs'>
            {t('Batch multiplier by group')}
          </span>
          <Select
            value={activeBatchGroup}
            onValueChange={(value) => setBatchGroup(value ?? '')}
          >
            <SelectTrigger className='w-full'>
              <SelectValue placeholder={t('Select group')} />
            </SelectTrigger>
            <SelectContent>
              {props.selectedGroups.map((group) => (
                <SelectItem key={group} value={group}>
                  {group}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </label>
        <label className='w-full space-y-1 sm:w-32'>
          <span className='text-muted-foreground text-xs'>
            {t('Multiplier')}
          </span>
          <Input
            type='number'
            min='0.01'
            step='0.01'
            value={batchValue}
            onChange={(event) => setBatchValue(event.target.value)}
          />
        </label>
        <Button
          type='button'
          variant='outline'
          onClick={() =>
            activeBatchGroup &&
            props.onGroupMultiplier(activeBatchGroup, Number(batchValue))
          }
        >
          {t('Apply')}
        </Button>
      </div>
      <div className='overflow-x-auto rounded-lg border'>
        <table className='w-full min-w-[720px] text-left text-xs'>
          <thead className='bg-muted/40 text-muted-foreground border-b'>
            <tr>
              <th className='px-3 py-2'>{t('Channel')}</th>
              <th className='px-3 py-2'>{t('Groups')}</th>
              <th className='px-3 py-2'>{t('Supported models')}</th>
              <th className='px-3 py-2'>{t('Pricing')}</th>
              <th className='px-3 py-2'>{t('Multiplier priority')}</th>
              <th className='px-3 py-2'>{t('Multiplier')}</th>
            </tr>
          </thead>
          <tbody className='divide-y'>
            {visibleChannels.map((channel) => {
              const channelMultiplier = props.multipliers[String(channel.id)]
              const groupMultiplier = channel.groups
                .map((group) => props.multipliers[`group:${group}`])
                .find((value) => Number.isFinite(value) && value > 0)
              const multiplier =
                channelMultiplier ?? groupMultiplier ?? channel.multiplier ?? 1
              const channelStat = props.channelStats?.find(
                (stat) => stat.channel_id === channel.id
              )
              const anomaly = channelStat?.anomaly === true
              const anomalyReason = channelStat?.anomaly_reason
              const multiplierPriority =
                channelMultiplier !== undefined
                  ? t('Channel')
                  : groupMultiplier !== undefined
                    ? t('Group')
                    : t('Default')
              return (
                <tr
                  key={channel.id}
                  className={
                    anomaly
                      ? 'bg-destructive/10 text-destructive'
                      : channel.special_billing && !channel.has_special_price
                        ? 'bg-amber-500/5'
                        : undefined
                  }
                >
                  <td className='px-3 py-2 font-medium'>
                    {anomaly && (
                      <TriangleAlert
                        className='mr-1 inline size-3.5'
                        aria-label={t('Cost anomaly')}
                      />
                    )}{' '}
                    {channel.name || `#${channel.id}`}{' '}
                    <span className='text-muted-foreground'>#{channel.id}</span>
                    {anomalyReason && (
                      <div className='text-destructive mt-1 text-[11px] font-normal'>
                        {anomalyReason}
                      </div>
                    )}
                  </td>
                  <td className='px-3 py-2'>{channel.groups.join(', ')}</td>
                  <td
                    className='max-w-64 truncate px-3 py-2'
                    title={channel.models.join(', ')}
                  >
                    {channel.models.join(', ')}
                  </td>
                  <td className='px-3 py-2'>
                    {channel.special_billing ? (
                      <Badge
                        variant={
                          channel.has_special_price
                            ? 'secondary'
                            : 'destructive'
                        }
                      >
                        {channel.has_special_price
                          ? t('Special price')
                          : t('Missing price')}
                      </Badge>
                    ) : (
                      t('Global price')
                    )}
                  </td>
                  <td className='px-3 py-2'>
                    <Badge variant='outline'>{multiplierPriority}</Badge>
                  </td>
                  <td className='w-32 px-3 py-2'>
                    <Input
                      type='number'
                      min='0.01'
                      step='0.01'
                      value={multiplier}
                      onChange={(event) =>
                        props.onMultiplierChange(
                          channel.id,
                          Number(event.target.value)
                        )
                      }
                    />
                  </td>
                </tr>
              )
            })}
            {visibleChannels.length === 0 && (
              <tr>
                <td
                  colSpan={6}
                  className='text-muted-foreground px-3 py-8 text-center'
                >
                  {t('Select at least one group and model')}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function ForecastPanel(props: {
  forecast: SpecialUsageForecast | undefined
  onChange: (basis: string, days: number, todayRemaining: boolean) => void
}) {
  const { t } = useTranslation()
  const [basis, setBasis] = useState('today_current')
  const [period, setPeriod] = useState('7')
  const [todayRemaining, setTodayRemaining] = useState(false)
  const updateForecast = (
    nextBasis: string,
    nextPeriod: string,
    nextTodayRemaining = todayRemaining
  ) => {
    const days = Number(nextPeriod)
    props.onChange(
      nextBasis,
      Number.isFinite(days) && days > 0 ? days : 1,
      nextTodayRemaining
    )
  }
  return (
    <Card className='border-indigo-500/20 bg-indigo-500/[0.035]'>
      <CardHeader className='pb-2'>
        <div className='flex items-center justify-between gap-3'>
          <div>
            <CardTitle className='text-sm'>
              {t('Traffic trend cost forecast')}
            </CardTitle>
            <CardDescription>
              {t('Forecast values are separate from real statistics.')}
            </CardDescription>
          </div>
          <Badge variant='outline'>{t('Trend forecast')}</Badge>
        </div>
      </CardHeader>
      <CardContent className='space-y-4'>
        <div className='grid gap-3 sm:grid-cols-3'>
          <label className='space-y-1'>
            <span className='text-muted-foreground text-xs'>
              {t('Forecast basis')}
            </span>
            <Select
              value={basis}
              onValueChange={(value) => {
                const nextBasis = value ?? 'today_current'
                setBasis(nextBasis)
                updateForecast(nextBasis, period)
              }}
            >
              <SelectTrigger className='w-full'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='today_current'>
                  {t('Today current period average')}
                </SelectItem>
                <SelectItem value='historical_daily'>
                  {t('Historical daily token average')}
                </SelectItem>
              </SelectContent>
            </Select>
          </label>
          <label className='space-y-1'>
            <span className='text-muted-foreground text-xs'>
              {t('Forecast period')}
            </span>
            <Select
              value={
                todayRemaining
                  ? 'remaining'
                  : ['7', '14', '29'].includes(period)
                    ? period
                    : 'custom'
              }
              onValueChange={(value) => {
                const isRemaining = value === 'remaining'
                const nextPeriod = isRemaining
                  ? '1'
                  : value === 'custom'
                    ? '30'
                    : (value ?? '7')
                setTodayRemaining(isRemaining)
                setPeriod(nextPeriod)
                updateForecast(basis, nextPeriod, isRemaining)
              }}
            >
              <SelectTrigger className='w-full'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='remaining'>
                  {t('Today remaining')}
                </SelectItem>
                <SelectItem value='7'>7 {t('days')}</SelectItem>
                <SelectItem value='14'>14 {t('days')}</SelectItem>
                <SelectItem value='29'>29 {t('days')}</SelectItem>
                <SelectItem value='custom'>{t('Custom days')}</SelectItem>
              </SelectContent>
            </Select>
          </label>
          <label className='space-y-1'>
            <span className='text-muted-foreground text-xs'>
              {t('Custom days')}
            </span>
            <Input
              type='number'
              min='0.01'
              max='3650'
              disabled={todayRemaining}
              value={period}
              onChange={(event) => {
                const nextPeriod = event.target.value
                setTodayRemaining(false)
                setPeriod(nextPeriod)
                updateForecast(basis, nextPeriod, false)
              }}
            />
          </label>
        </div>
        <div className='grid gap-3 sm:grid-cols-3'>
          <div className='bg-background/70 rounded-lg border p-3'>
            <div className='text-muted-foreground text-xs'>
              {t('Forecast tokens')}
            </div>
            <div className='mt-1 text-lg font-semibold'>
              {formatTokens(props.forecast?.forecast_tokens ?? 0)}
            </div>
          </div>
          <div className='bg-background/70 rounded-lg border p-3'>
            <div className='text-muted-foreground text-xs'>
              {t('Forecast upstream cost')}
            </div>
            <div className='mt-1 text-lg font-semibold'>
              {formatUSD(props.forecast?.forecast_cost_usd ?? 0)}
            </div>
          </div>
          <div className='bg-background/70 rounded-lg border p-3'>
            <div className='text-muted-foreground text-xs'>
              {t('Daily token baseline')}
            </div>
            <div className='mt-1 text-lg font-semibold'>
              {formatTokens(props.forecast?.daily_tokens ?? 0)}
            </div>
          </div>
        </div>
        <p className='text-muted-foreground text-[11px]'>
          {t(
            'Forecast assumes traffic remains stable. Sudden traffic growth or shorter conversations can cause actual cost to differ.'
          )}
        </p>
      </CardContent>
    </Card>
  )
}

export function SpecialUsageMonitor() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [period, setPeriod] = useState<PeriodKey>('7d')
  const [customStart, setCustomStart] = useState('')
  const [customEnd, setCustomEnd] = useState('')
  const [selectedGroupsDraft, setSelectedGroupsDraft] = useState<
    string[] | null
  >(null)
  const [selectedModelsDraft, setSelectedModelsDraft] = useState<
    string[] | null
  >(null)
  const [selectedChannelsDraft, setSelectedChannelsDraft] = useState<
    number[] | null
  >(null)
  const [recordPage, setRecordPage] = useState(1)
  const [recordPageSize, setRecordPageSize] = useState(20)
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [exportFormat, setExportFormat] =
    useState<SpecialUsageExportFormat>('xlsx')
  const [isRefreshing, setIsRefreshing] = useState(false)
  const [configDraft, setConfigDraft] = useState<SpecialUsageConfig | null>(
    null
  )
  const [multipliersDraft, setMultipliersDraft] = useState<Record<
    string,
    number
  > | null>(null)
  const [forecastSettings, setForecastSettings] = useState({
    basis: 'today_current',
    days: 7,
    todayRemaining: false,
  })
  const [profitMode, setProfitMode] = useState<'auto' | 'manual'>('auto')
  const [manualRevenue, setManualRevenue] = useState('')
  const metadataQuery = useQuery({
    queryKey: ['special-usage', 'metadata'],
    queryFn: getSpecialUsageMetadata,
    staleTime: 60_000,
    refetchOnWindowFocus: false,
  })
  const metadata = metadataQuery.data?.data
  const metadataConfig = metadata?.config
  const defaultSelectedGroups =
    metadataConfig?.group_names.length && metadataConfig.group_names.length > 0
      ? metadataConfig.group_names
      : (metadata?.groups ?? [])
  const defaultSelectedModels =
    metadataConfig?.model_names.length && metadataConfig.model_names.length > 0
      ? metadataConfig.model_names
      : (metadata?.models ?? [])
  const defaultSelectedChannels = metadataConfig?.channel_ids_set
    ? metadataConfig.channel_ids
    : (metadata?.channels.map((channel) => channel.id) ?? [])
  const config = configDraft ?? metadataConfig ?? null
  const selectedGroups = selectedGroupsDraft ?? defaultSelectedGroups
  const selectedModels = selectedModelsDraft ?? defaultSelectedModels
  const selectedChannels = selectedChannelsDraft ?? defaultSelectedChannels
  const multipliers =
    multipliersDraft ?? metadataConfig?.channel_multipliers ?? {}
  const editableConfig: SpecialUsageConfig = config ?? {
    enabled: true,
    group_names: selectedGroups,
    model_names: selectedModels,
    channel_ids: selectedChannels,
    special_billing: false,
    channel_multipliers: multipliers,
    updated_at: 0,
  }

  const range = useMemo(
    () => periodRange(period, customStart, customEnd),
    [period, customStart, customEnd]
  )
  const rangeValid =
    Number.isFinite(range.start) &&
    Number.isFinite(range.end) &&
    range.start < range.end
  const query = useMemo<SpecialUsageQuery>(
    () => ({
      start: range.start,
      end: range.end,
      groups: selectedGroups,
      models: selectedModels,
      channels: selectedChannels,
    }),
    [range.end, range.start, selectedGroups, selectedModels, selectedChannels]
  )
  const queryEnabled =
    config?.enabled === true &&
    rangeValid &&
    selectedGroups.length > 0 &&
    selectedModels.length > 0
  const overviewQuery = useQuery({
    queryKey: ['special-usage', 'overview', query],
    queryFn: () => getSpecialUsageOverview(query),
    enabled: queryEnabled,
    refetchInterval: autoRefresh ? 300_000 : false,
    refetchIntervalInBackground: false,
    refetchOnWindowFocus: false,
  })
  const recordsQuery = useQuery({
    queryKey: ['special-usage', 'records', query, recordPage, recordPageSize],
    queryFn: () =>
      getSpecialUsageRecords({
        ...query,
        page: recordPage,
        page_size: recordPageSize,
      }),
    enabled: queryEnabled,
    refetchInterval: autoRefresh ? 300_000 : false,
    refetchIntervalInBackground: false,
    refetchOnWindowFocus: false,
  })
  const forecastQuery = useQuery({
    queryKey: ['special-usage', 'forecast', query, forecastSettings],
    queryFn: () =>
      getSpecialUsageForecast(
        query,
        forecastSettings.basis,
        forecastSettings.days,
        forecastSettings.todayRemaining
      ),
    enabled: queryEnabled,
    refetchInterval: autoRefresh ? 300_000 : false,
    refetchIntervalInBackground: false,
    refetchOnWindowFocus: false,
  })
  const manualRevenueValue = Number(manualRevenue)
  const manualRevenueValid =
    manualRevenue.trim() !== '' &&
    Number.isFinite(manualRevenueValue) &&
    manualRevenueValue >= 0
  const profitQuery = useQuery({
    queryKey: [
      'special-usage',
      'profit',
      query,
      profitMode,
      manualRevenueValue,
    ],
    queryFn: () =>
      getSpecialUsageProfit({
        ...query,
        mode: profitMode,
        ...(profitMode === 'manual' ? { revenue: manualRevenueValue } : {}),
      }),
    enabled: queryEnabled && (profitMode === 'auto' || manualRevenueValid),
    refetchInterval: autoRefresh ? 300_000 : false,
    refetchIntervalInBackground: false,
    refetchOnWindowFocus: false,
  })
  const overview = overviewQuery.data?.data
  const forecast = forecastQuery.data?.data
  const profit = profitQuery.data?.data ?? {
    revenue: 0,
    cost: 0,
    profit: 0,
    margin: 0,
  }
  const configMutation = useMutation({
    mutationFn: saveSpecialUsageConfig,
    onSuccess: (response) => {
      if (response.data) {
        setConfigDraft(response.data)
        setMultipliersDraft(response.data.channel_multipliers)
      }
      toast.success(t('Special usage configuration saved'))
      queryClient.invalidateQueries({ queryKey: ['special-usage'] })
    },
    onError: () => toast.error(t('Failed to save special usage configuration')),
  })
  const exportMutation = useMutation({
    mutationFn: ([exportQuery, format]: [
      SpecialUsageQuery,
      SpecialUsageExportFormat,
    ]) => downloadSpecialUsageExport(exportQuery, format),
    onSuccess: () => toast.success(t('Export completed')),
    onError: (error) =>
      toast.error(error instanceof Error ? error.message : t('Export failed')),
  })

  const saveConfig = () => {
    const enabled = editableConfig.enabled === true
    if (enabled && (!selectedGroups.length || !selectedModels.length)) {
      toast.error(t('Select at least one group and model'))
      return
    }
    configMutation.mutate({
      enabled,
      group_names: selectedGroups,
      model_names: selectedModels,
      channel_ids: selectedChannels,
      special_billing: editableConfig.special_billing === true,
      channel_multipliers: multipliers,
    })
  }
  const matchingChannelIDs = (groups: string[], models: string[]) =>
    new Set(
      (metadata?.channels ?? [])
        .filter(
          (channel) =>
            channel.groups.some((group) => groups.includes(group)) &&
            channel.models.some((model) => models.includes(model))
        )
        .map((channel) => channel.id)
    )
  const updateSelectedGroups = (values: string[]) => {
    setSelectedGroupsDraft(values)
    setRecordPage(1)
    if (values.length === 0) {
      setSelectedModelsDraft([])
      setSelectedChannelsDraft([])
      return
    }
    const matchingIDs = matchingChannelIDs(values, selectedModels)
    setSelectedChannelsDraft(
      selectedChannels.filter((channelID) => matchingIDs.has(channelID))
    )
  }
  const updateSelectedModels = (values: string[]) => {
    setSelectedModelsDraft(values)
    setRecordPage(1)
    const matchingIDs = matchingChannelIDs(selectedGroups, values)
    setSelectedChannelsDraft(
      selectedChannels.filter((channelID) => matchingIDs.has(channelID))
    )
  }
  const updateSelectedChannels = (values: number[]) => {
    setSelectedChannelsDraft(values)
    setRecordPage(1)
  }
  const updatePeriod = (value: PeriodKey) => {
    setPeriod(value)
    setRecordPage(1)
  }
  const updateCustomStart = (value: string) => {
    setCustomStart(value)
    setRecordPage(1)
  }
  const updateCustomEnd = (value: string) => {
    setCustomEnd(value)
    setRecordPage(1)
  }
  const updateConfig = (patch: Partial<SpecialUsageConfig>) =>
    setConfigDraft({ ...editableConfig, ...patch })
  const updateGroupMultiplier = (group: string, value: number) => {
    const groupKey = `group:${group}`
    const nextValue = Number.isFinite(value) && value > 0 ? value : 1
    setMultipliersDraft({ ...multipliers, [groupKey]: nextValue })
  }
  const updateChannelMultiplier = (channelID: number, value: number) =>
    setMultipliersDraft({
      ...multipliers,
      [String(channelID)]: Number.isFinite(value) && value > 0 ? value : 1,
    })
  const refresh = async () => {
    setIsRefreshing(true)
    try {
      await Promise.all([
        metadataQuery.refetch(),
        queryClient.refetchQueries({ queryKey: ['special-usage'] }),
      ])
      toast.success(t('Refresh completed'))
    } catch {
      toast.error(t('Refresh failed'))
    } finally {
      setIsRefreshing(false)
    }
  }

  const totals = overview?.totals
  const records = recordsQuery.data?.data.items ?? []
  const recordTotal = recordsQuery.data?.data.total ?? 0
  const filteredChannels =
    metadata?.channels.filter(
      (channel) =>
        channel.groups.some((group) => selectedGroups.includes(group)) &&
        channel.models.some((model) => selectedModels.includes(model))
    ) ?? []
  return (
    <div className='flex h-full w-full flex-1 flex-col'>
      <div className='faded-bottom h-full w-full overflow-y-auto scroll-smooth pe-4 pb-12'>
        <div className='mx-auto max-w-[1600px] space-y-4'>
          <div className='flex flex-col justify-between gap-3 sm:flex-row sm:items-end'>
            <div>
              <div className='flex items-center gap-2'>
                <BarChart3 className='text-primary size-5' />
                <h1 className='text-xl font-semibold'>
                  {t('Special usage monitoring')}
                </h1>
              </div>
              <p className='text-muted-foreground mt-1 text-xs'>
                {t(
                  'Real statistics and trend forecasts for selected upstream traffic'
                )}
              </p>
            </div>
            <div className='flex flex-wrap items-center justify-end gap-2'>
              <label className='text-muted-foreground flex items-center gap-2 text-xs'>
                <Switch
                  checked={autoRefresh}
                  onCheckedChange={setAutoRefresh}
                />
                {t('Auto-refresh (5 min)')}
              </label>
              <Button
                variant='outline'
                size='sm'
                onClick={refresh}
                disabled={isRefreshing}
              >
                <RefreshCw
                  className={`size-3.5 ${isRefreshing ? 'animate-spin' : ''}`}
                />
                {isRefreshing ? t('Refreshing...') : t('Refresh')}
              </Button>
              <Select
                value={exportFormat}
                onValueChange={(value) =>
                  setExportFormat(value as SpecialUsageExportFormat)
                }
              >
                <SelectTrigger className='w-24' aria-label={t('Export format')}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='xlsx'>XLSX</SelectItem>
                  <SelectItem value='csv'>CSV</SelectItem>
                </SelectContent>
              </Select>
              <Button
                variant='outline'
                size='sm'
                onClick={() => exportMutation.mutate([query, exportFormat])}
                disabled={exportMutation.isPending || !rangeValid}
              >
                <Download className='size-3.5' />
                {exportMutation.isPending ? t('Exporting...') : t('Export')}
              </Button>
            </div>
          </div>

          <div className='bg-card flex flex-col gap-2 rounded-xl border p-3 sm:flex-row sm:items-center'>
            <span className='text-muted-foreground text-xs'>
              {t('Statistics period')}
            </span>
            <div className='flex flex-wrap gap-1'>
              {(['today', '24h', '7d', '30d', 'custom'] as PeriodKey[]).map(
                (value) => (
                  <Button
                    key={value}
                    size='sm'
                    variant={period === value ? 'default' : 'ghost'}
                    onClick={() => updatePeriod(value)}
                  >
                    {value === 'today'
                      ? t('Today')
                      : value === '24h'
                        ? t('Last 24 hours')
                        : value === '7d'
                          ? t('Last 7 days')
                          : value === '30d'
                            ? t('Last 30 days')
                            : t('Custom')}
                  </Button>
                )
              )}
            </div>
            {period === 'custom' && (
              <div className='flex flex-wrap gap-2'>
                <Input
                  type='date'
                  value={customStart}
                  onChange={(event) => updateCustomStart(event.target.value)}
                />
                <Input
                  type='date'
                  value={customEnd}
                  onChange={(event) => updateCustomEnd(event.target.value)}
                />
              </div>
            )}
          </div>

          <Card>
            <CardHeader className='pb-3'>
              <div className='flex items-center justify-between gap-3'>
                <div>
                  <CardTitle className='text-sm'>
                    {t('Monitoring configuration')}
                  </CardTitle>
                  <CardDescription>
                    {t(
                      'Choose groups first, then models, then adjust matching channels.'
                    )}
                  </CardDescription>
                </div>
                <div className='flex items-center gap-2'>
                  <span className='text-muted-foreground text-xs'>
                    {config?.enabled ? t('Enabled') : t('Disabled')}
                  </span>
                  <Switch
                    checked={config?.enabled ?? false}
                    onCheckedChange={(checked) =>
                      updateConfig({ enabled: checked })
                    }
                  />
                </div>
              </div>
            </CardHeader>
            <CardContent className='space-y-5'>
              <div className='grid gap-4 lg:grid-cols-3'>
                <ToggleList
                  title={t('Step 1 · Monitoring groups')}
                  values={metadata?.groups ?? []}
                  selected={selectedGroups}
                  onChange={updateSelectedGroups}
                />
                <ToggleList
                  title={t('Step 2 · Monitoring models')}
                  values={metadata?.models ?? []}
                  selected={selectedModels}
                  onChange={updateSelectedModels}
                  disabled={selectedGroups.length === 0}
                />
                <ToggleList
                  title={t('Step 3 · Monitoring channels')}
                  values={filteredChannels.map((channel) => String(channel.id))}
                  selected={selectedChannels.map(String)}
                  onChange={(values) =>
                    updateSelectedChannels(values.map(Number))
                  }
                />
              </div>
              <div className='bg-muted/20 flex flex-col gap-3 rounded-lg border p-3 sm:flex-row sm:items-center sm:justify-between'>
                <div>
                  <div className='flex items-center gap-2 text-sm font-medium'>
                    <Settings2 className='size-4' />
                    {t('Step 3 · Channel pricing')}
                  </div>
                  <div className='text-muted-foreground mt-1 text-xs'>
                    {t('{{count}} matching channels', {
                      count: filteredChannels.length,
                    })}
                  </div>
                </div>
                <label className='flex items-center gap-2 text-xs'>
                  <Switch
                    checked={config?.special_billing ?? false}
                    onCheckedChange={(checked) =>
                      updateConfig({ special_billing: checked })
                    }
                  />
                  {t('Use channel special billing price')}
                </label>
              </div>
              <ChannelConfigTable
                channels={metadata?.channels ?? []}
                selectedGroups={selectedGroups}
                selectedModels={selectedModels}
                channelStats={overview?.channels}
                multipliers={multipliers}
                onMultiplierChange={updateChannelMultiplier}
                onGroupMultiplier={updateGroupMultiplier}
              />
              <div className='flex justify-end'>
                <Button
                  onClick={saveConfig}
                  disabled={configMutation.isPending}
                >
                  <Save className='size-4' />
                  {configMutation.isPending
                    ? t('Saving...')
                    : t('Save and apply')}
                </Button>
              </div>
            </CardContent>
          </Card>

          <div className='flex flex-wrap items-center gap-2'>
            <Badge variant='secondary'>{t('Real statistics')}</Badge>
            <span className='text-muted-foreground text-xs'>
              {formatDate(overview?.last_updated_at ?? 0)}
            </span>
            {overviewQuery.isFetching && (
              <span className='text-muted-foreground text-xs'>
                {t('Updating...')}
              </span>
            )}
          </div>
          {!rangeValid && (
            <Card className='border-destructive/40'>
              <CardContent className='text-destructive py-4 text-sm'>
                {t(
                  'Choose a valid date range. The end date must be after the start date.'
                )}
              </CardContent>
            </Card>
          )}
          {metadataQuery.isLoading && (
            <Card>
              <CardContent className='text-muted-foreground py-8 text-center text-sm'>
                {t('Loading monitoring metadata...')}
              </CardContent>
            </Card>
          )}
          {metadataQuery.isError && (
            <Card className='border-destructive/40'>
              <CardContent className='text-destructive py-8 text-center text-sm'>
                {t('Failed to load monitoring metadata. Please refresh.')}
              </CardContent>
            </Card>
          )}
          {queryEnabled && overviewQuery.isError && (
            <Card className='border-destructive/40'>
              <CardContent className='text-destructive py-4 text-sm'>
                {t('Failed to load statistics. Please refresh.')}
              </CardContent>
            </Card>
          )}
          {queryEnabled && overviewQuery.isLoading && (
            <Card>
              <CardContent className='text-muted-foreground py-8 text-center text-sm'>
                {t('Loading real statistics...')}
              </CardContent>
            </Card>
          )}
          <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
            <MetricCard
              label={t('Total requests')}
              value={formatTokens(totals?.request_count ?? 0)}
            />
            <MetricCard
              label={t('Average request cost')}
              value={formatUSD(
                (totals?.request_count ?? 0) > 0
                  ? (totals?.upstream_cost_usd ?? 0) /
                      (totals?.request_count ?? 1)
                  : 0
              )}
            />
            <MetricCard
              label={t('Average cost per million tokens')}
              value={formatUSD(
                (totals?.input_tokens ?? 0) + (totals?.output_tokens ?? 0) > 0
                  ? ((totals?.upstream_cost_usd ?? 0) /
                      ((totals?.input_tokens ?? 0) +
                        (totals?.output_tokens ?? 0))) *
                      1_000_000
                  : 0
              )}
            />
            <MetricCard
              label={t('Total upstream cost')}
              value={formatUSD(totals?.upstream_cost_usd ?? 0)}
            />
          </div>
          <ForecastPanel
            forecast={forecast}
            onChange={(basis, days, todayRemaining) =>
              setForecastSettings({ basis, days, todayRemaining })
            }
          />
          {overview && <SpecialUsageCharts overview={overview} />}

          <Card>
            <CardHeader className='pb-3'>
              <CardTitle className='text-sm'>
                {t('Profit accounting')}
              </CardTitle>
              <CardDescription>
                {t(
                  'Revenue and gross profit follow the selected statistics period.'
                )}
              </CardDescription>
            </CardHeader>
            <CardContent className='space-y-4'>
              <div className='flex flex-wrap items-center gap-2'>
                <Button
                  size='sm'
                  variant={profitMode === 'auto' ? 'default' : 'outline'}
                  onClick={() => setProfitMode('auto')}
                >
                  {t('Automatic revenue')}
                </Button>
                <Button
                  size='sm'
                  variant={profitMode === 'manual' ? 'default' : 'outline'}
                  onClick={() => setProfitMode('manual')}
                >
                  {t('Manual revenue')}
                </Button>
                {profitMode === 'manual' && (
                  <Input
                    className='w-40'
                    type='number'
                    min='0'
                    step='0.0001'
                    placeholder={t('Revenue in USD')}
                    value={manualRevenue}
                    onChange={(event) => setManualRevenue(event.target.value)}
                  />
                )}
              </div>
              <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
                <MetricCard
                  label={t('Revenue')}
                  value={formatUSD(profit.revenue)}
                />
                <MetricCard
                  label={t('Upstream cost')}
                  value={formatUSD(profit.cost)}
                />
                <MetricCard
                  label={t('Gross profit')}
                  value={formatUSD(profit.profit)}
                />
                <MetricCard
                  label={t('Gross margin')}
                  value={`${profit.margin.toFixed(2)}%`}
                />
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className='pb-3'>
              <CardTitle className='text-sm'>{t('Request details')}</CardTitle>
              <CardDescription>
                {t('Independent monitoring ledger records')}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className='overflow-x-auto rounded-lg border'>
                <table className='w-full min-w-[980px] text-left text-xs'>
                  <thead className='bg-muted/40 text-muted-foreground border-b'>
                    <tr>
                      <th className='px-3 py-2'>{t('Time')}</th>
                      <th className='px-3 py-2'>{t('Request ID')}</th>
                      <th className='px-3 py-2'>{t('Channel')}</th>
                      <th className='px-3 py-2'>{t('Group')}</th>
                      <th className='px-3 py-2'>{t('Model')}</th>
                      <th className='px-3 py-2'>{t('Input tokens')}</th>
                      <th className='px-3 py-2'>{t('Output tokens')}</th>
                      <th className='px-3 py-2'>{t('Price / multiplier')}</th>
                      <th className='px-3 py-2'>{t('Usage source')}</th>
                      <th className='px-3 py-2'>{t('Cost')}</th>
                      <th className='px-3 py-2'>{t('Charge')}</th>
                      <th className='px-3 py-2'>{t('Status / error')}</th>
                      <th className='px-3 py-2'>{t('Attempt')}</th>
                    </tr>
                  </thead>
                  <tbody className='divide-y'>
                    {records.map((record) => (
                      <tr key={record.id}>
                        <td className='px-3 py-2 whitespace-nowrap'>
                          {formatDate(record.request_time)}
                        </td>
                        <td
                          className='max-w-36 truncate px-3 py-2'
                          title={record.request_id}
                        >
                          {record.request_id}
                        </td>
                        <td className='px-3 py-2'>
                          {record.channel_name || `#${record.channel_id}`}
                        </td>
                        <td className='px-3 py-2'>{record.group_name}</td>
                        <td className='max-w-40 truncate px-3 py-2'>
                          {record.model_name}
                        </td>
                        <td className='px-3 py-2'>
                          {formatTokens(record.input_tokens)}
                        </td>
                        <td className='px-3 py-2'>
                          {formatTokens(record.output_tokens)}
                        </td>
                        <td className='px-3 py-2 whitespace-nowrap'>
                          {formatUSD(record.input_price_usd ?? 0)} /{' '}
                          {formatUSD(record.output_price_usd ?? 0)}
                          <span className='text-muted-foreground'>
                            {' '}
                            × {record.multiplier ?? 1}
                          </span>
                        </td>
                        <td className='px-3 py-2'>
                          {record.usage_source ??
                            (record.used_special_price
                              ? t('Special price')
                              : t('Global price'))}
                        </td>
                        <td className='px-3 py-2'>
                          {formatUSD(record.upstream_cost_usd)}
                        </td>
                        <td className='px-3 py-2'>
                          {formatUSD(record.user_charge_usd)}
                        </td>
                        <td className='max-w-56 px-3 py-2'>
                          {record.status === 'success' ? (
                            <Badge variant='secondary'>
                              <Check className='size-3' />
                              {t('Success')}
                            </Badge>
                          ) : (
                            <div className='space-y-1'>
                              <Badge variant='destructive'>
                                <TriangleAlert className='size-3' />
                                {t('Failed')}
                              </Badge>
                              {record.error_message && (
                                <div
                                  className='text-destructive truncate'
                                  title={record.error_message}
                                >
                                  {record.error_message}
                                </div>
                              )}
                            </div>
                          )}
                        </td>
                        <td className='px-3 py-2'>{record.attempt ?? '-'}</td>
                      </tr>
                    ))}
                    {records.length === 0 && (
                      <tr>
                        <td
                          colSpan={13}
                          className='text-muted-foreground px-3 py-10 text-center'
                        >
                          {t('No monitoring records in this period')}
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
              <div className='flex flex-wrap items-center justify-between gap-2 pt-3 text-xs'>
                <span className='text-muted-foreground'>
                  {t('Showing')}{' '}
                  {recordTotal === 0
                    ? 0
                    : (recordPage - 1) * recordPageSize + 1}
                  -{Math.min(recordPage * recordPageSize, recordTotal)}{' '}
                  {t('of')} {recordTotal}
                </span>
                <div className='flex items-center gap-2'>
                  <Select
                    value={String(recordPageSize)}
                    onValueChange={(value) => {
                      setRecordPageSize(Number(value))
                      setRecordPage(1)
                    }}
                  >
                    <SelectTrigger className='w-24'>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {[20, 50, 100].map((size) => (
                        <SelectItem key={size} value={String(size)}>
                          {size}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <Button
                    size='sm'
                    variant='outline'
                    disabled={recordPage <= 1}
                    onClick={() => setRecordPage((page) => page - 1)}
                  >
                    {t('Previous')}
                  </Button>
                  <span>
                    {recordPage} /{' '}
                    {Math.max(1, Math.ceil(recordTotal / recordPageSize))}
                  </span>
                  <Button
                    size='sm'
                    variant='outline'
                    disabled={
                      recordPage >= Math.ceil(recordTotal / recordPageSize)
                    }
                    onClick={() => setRecordPage((page) => page + 1)}
                  >
                    {t('Next')}
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  )
}
