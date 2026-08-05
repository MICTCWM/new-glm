/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or (at your
option) any later version.
*/
import { useCallback, useMemo, useState, type MouseEvent } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  BarChart3,
  CalendarDays,
  Check,
  Clock3,
  Copy,
  GripVertical,
  MoreHorizontal,
  Pencil,
  RefreshCw,
  Trash2,
} from 'lucide-react'
import { motion } from 'motion/react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { DateTimePicker } from '@/components/datetime-picker'
import { getAllLogs } from '@/features/usage-logs/api'
import type { UsageLog } from '@/features/usage-logs/data/schema'
import { getChannels } from '../api'
import { CHANNEL_STATUS, CHANNEL_STATUS_CONFIG } from '../constants'
import { channelsQueryKeys } from '../lib'
import { getChannelTypeIcon, getChannelTypeLabel } from '../lib/channel-utils'
import type { Channel } from '../types'
import { useChannels } from './channels-provider'

const FOLDER_STORAGE_KEY = 'channels-modern-folders'
const CHANNEL_PAGE_SIZE = 100
const MAX_LOG_PAGES_PER_CHANNEL = 50
const FOLDER_COLORS = [
  '#2563eb',
  '#0891b2',
  '#059669',
  '#ca8a04',
  '#ea580c',
  '#dc2626',
  '#c026d3',
  '#7c3aed',
]

type ChannelFolder = {
  id: string
  name: string
  color: string
  channelIds: number[]
}

type DateRange = {
  start: Date
  end: Date
  days: number | null
}

type ChartBucket = {
  label: string
  requests: number
  latency: number
}

type ChannelStats = {
  requests: number
  tokens: number
  latencySum: number
  latencySamples: number
  buckets: ChartBucket[]
}

const EMPTY_STATS: ChannelStats = {
  requests: 0,
  tokens: 0,
  latencySum: 0,
  latencySamples: 0,
  buckets: [],
}
const EMPTY_STATS_BY_CHANNEL: Record<number, ChannelStats> = {}

async function mapWithConcurrency<T, R>(
  values: T[],
  concurrency: number,
  mapper: (value: T, index: number) => Promise<R>
): Promise<R[]> {
  const results = new Array<R>(values.length)
  let nextIndex = 0
  const worker = async () => {
    while (nextIndex < values.length) {
      const index = nextIndex
      nextIndex += 1
      results[index] = await mapper(values[index], index)
    }
  }
  await Promise.all(
    Array.from({ length: Math.min(concurrency, values.length) }, worker)
  )
  return results
}

function readFolders(): ChannelFolder[] {
  try {
    const value = localStorage.getItem(FOLDER_STORAGE_KEY)
    if (!value) return []
    const parsed = JSON.parse(value) as unknown
    if (!Array.isArray(parsed)) return []
    return parsed.filter((folder): folder is ChannelFolder =>
      Boolean(
        folder &&
        typeof folder === 'object' &&
        typeof (folder as ChannelFolder).id === 'string' &&
        typeof (folder as ChannelFolder).name === 'string' &&
        typeof (folder as ChannelFolder).color === 'string' &&
        Array.isArray((folder as ChannelFolder).channelIds)
      )
    )
  } catch {
    return []
  }
}

function persistFolders(folders: ChannelFolder[]) {
  localStorage.setItem(FOLDER_STORAGE_KEY, JSON.stringify(folders))
}

function getDateRange(days: number): DateRange {
  const end = new Date()
  const start = new Date(end.getTime() - days * 24 * 60 * 60 * 1000)
  return { start, end, days }
}

function formatCompact(value: number): string {
  if (!Number.isFinite(value) || value === 0) return '0'
  const absolute = Math.abs(value)
  const units = [
    { threshold: 1_000_000_000, suffix: 'B' },
    { threshold: 1_000_000, suffix: 'M' },
    { threshold: 1_000, suffix: 'K' },
  ]
  const unit = units.find((item) => absolute >= item.threshold)
  if (!unit) return Math.round(value).toLocaleString()
  const number = value / unit.threshold
  return `${number >= 100 ? number.toFixed(0) : number >= 10 ? number.toFixed(1) : number.toFixed(2)}${unit.suffix}`
}

async function copyChannelId(id: number, t: (key: string) => string) {
  try {
    await navigator.clipboard.writeText(String(id))
    toast.success(t('Copied'))
  } catch {
    toast.error(t('Copy failed'))
  }
}

function buildBuckets(logs: UsageLog[], start: Date, end: Date): ChartBucket[] {
  const duration = end.getTime() - start.getTime()
  const bucketCount =
    duration <= 3 * 86400000 ? 48 : duration <= 7 * 86400000 ? 56 : 72
  const bucketDuration = duration / bucketCount
  const buckets = Array.from({ length: bucketCount }, (_, index) => ({
    label:
      duration <= 7 * 86400000
        ? new Date(start.getTime() + index * bucketDuration).toLocaleTimeString(
            [],
            { hour: '2-digit', minute: '2-digit' }
          )
        : new Date(start.getTime() + index * bucketDuration).toLocaleDateString(
            [],
            { month: '2-digit', day: '2-digit' }
          ),
    requests: 0,
    latency: 0,
  }))

  for (const log of logs) {
    const index = Math.max(
      0,
      Math.min(
        bucketCount - 1,
        Math.floor(
          ((log.created_at * 1000 - start.getTime()) / duration) * bucketCount
        )
      )
    )
    buckets[index].requests += 1
    buckets[index].latency += Math.max(0, log.use_time || 0)
  }

  return buckets.map((bucket) => ({
    ...bucket,
    latency: bucket.requests > 0 ? bucket.latency / bucket.requests : 0,
  }))
}

async function fetchChannelStats(
  channel: Channel,
  range: DateRange
): Promise<ChannelStats> {
  const params = {
    type: 2,
    channel: channel.id,
    start_timestamp: Math.floor(range.start.getTime() / 1000),
    end_timestamp: Math.floor(range.end.getTime() / 1000),
  }

  const firstPage = await getAllLogs({
    ...params,
    p: 1,
    page_size: CHANNEL_PAGE_SIZE,
  })
  const firstItems = (firstPage.data?.items || []) as UsageLog[]
  const total = firstPage.data?.total || 0
  const pageCount = Math.min(
    Math.ceil(total / CHANNEL_PAGE_SIZE),
    MAX_LOG_PAGES_PER_CHANNEL
  )
  const remainingPages = await mapWithConcurrency(
    Array.from({ length: Math.max(0, pageCount - 1) }),
    8,
    (_, index) =>
      getAllLogs({
        ...params,
        p: index + 2,
        page_size: CHANNEL_PAGE_SIZE,
      })
  )
  const logs = firstItems.concat(
    remainingPages.flatMap((page) => (page.data?.items || []) as UsageLog[])
  )
  const tokens = logs.reduce(
    (sum, log) => sum + (log.prompt_tokens || 0) + (log.completion_tokens || 0),
    0
  )
  const latencySum = logs.reduce(
    (sum, log) => sum + Math.max(0, log.use_time || 0),
    0
  )
  const fallbackLatency = Math.max(0, (channel.response_time || 0) / 1000)

  return {
    requests: total,
    tokens,
    latencySum: logs.length > 0 ? latencySum : fallbackLatency,
    latencySamples: logs.length > 0 ? logs.length : fallbackLatency > 0 ? 1 : 0,
    buckets: buildBuckets(logs, range.start, range.end),
  }
}

async function fetchAllChannels(): Promise<Channel[]> {
  const first = await getChannels({
    p: 1,
    page_size: CHANNEL_PAGE_SIZE,
    id_sort: true,
  })
  const firstData = first.data
  if (!firstData) return []

  const pageCount = Math.ceil((firstData.total || 0) / CHANNEL_PAGE_SIZE)
  const pages = await mapWithConcurrency(
    Array.from({ length: Math.max(0, pageCount - 1) }),
    8,
    (_, index) =>
      getChannels({
        p: index + 2,
        page_size: CHANNEL_PAGE_SIZE,
        id_sort: true,
      })
  )
  return [
    ...(firstData.items || []),
    ...pages.flatMap((page) => page.data?.items || []),
  ]
}

function mergeStats(
  channelIds: number[],
  statsByChannel: Record<number, ChannelStats>
): ChannelStats {
  const stats = channelIds.map((id) => statsByChannel[id]).filter(Boolean)
  if (stats.length === 0) return EMPTY_STATS
  const bucketCount = Math.max(...stats.map((stat) => stat.buckets.length), 0)
  const buckets = Array.from({ length: bucketCount }, (_, index) => ({
    label:
      stats.find((stat) => stat.buckets[index])?.buckets[index]?.label || '',
    requests: stats.reduce(
      (sum, stat) => sum + (stat.buckets[index]?.requests || 0),
      0
    ),
    latency: 0,
  }))
  for (let index = 0; index < bucketCount; index += 1) {
    const requestCount = buckets[index].requests
    buckets[index].latency =
      requestCount > 0
        ? stats.reduce(
            (sum, stat) =>
              sum +
              (stat.buckets[index]?.latency || 0) *
                (stat.buckets[index]?.requests || 0),
            0
          ) / requestCount
        : 0
  }
  return {
    requests: stats.reduce((sum, stat) => sum + stat.requests, 0),
    tokens: stats.reduce((sum, stat) => sum + stat.tokens, 0),
    latencySum: stats.reduce((sum, stat) => sum + stat.latencySum, 0),
    latencySamples: stats.reduce((sum, stat) => sum + stat.latencySamples, 0),
    buckets,
  }
}

function statusLabel(channel: Channel, t: (key: string) => string) {
  const config =
    CHANNEL_STATUS_CONFIG[
      channel.status as keyof typeof CHANNEL_STATUS_CONFIG
    ] || CHANNEL_STATUS_CONFIG[CHANNEL_STATUS.UNKNOWN]
  return t(config.label)
}

function ChannelStatsChart({
  stats,
  pulse,
  onExpand,
  large = false,
}: {
  stats: ChannelStats
  pulse: number
  onExpand?: () => void
  large?: boolean
}) {
  const max = Math.max(...stats.buckets.map((bucket) => bucket.requests), 1)
  const chart = (
    <div
      className={cn(
        'bg-muted/30 flex min-w-0 items-end gap-px overflow-hidden rounded-md px-1.5 py-2',
        large ? 'h-80 gap-1 px-3 py-4' : 'h-20'
      )}
      aria-label='Request volume chart'
    >
      {stats.buckets.map((bucket, index) => {
        const ratio = bucket.requests / max
        const hue = 52 - Math.round(ratio * 48)
        return (
          <motion.div
            key={`${bucket.label}-${index}-${pulse}`}
            initial={{ scaleY: 0.15, opacity: 0.5 }}
            animate={{ scaleY: 1, opacity: 1 }}
            transition={{
              duration: 0.55,
              delay: Math.min(index * 0.008, 0.25),
            }}
            title={`${bucket.label}: ${bucket.requests.toLocaleString()} requests, ${bucket.latency.toFixed(2)}s`}
            className='min-w-0 flex-1 origin-bottom rounded-t-sm'
            style={{
              height: `${Math.max(6, ratio * 100)}%`,
              backgroundColor: `hsl(${hue} 88% 52%)`,
            }}
          />
        )
      })}
    </div>
  )

  if (!onExpand) return chart
  return (
    <div
      role='button'
      tabIndex={0}
      className='focus-visible:ring-ring cursor-zoom-in rounded-md outline-none focus-visible:ring-2'
      onClick={(event) => {
        event.stopPropagation()
        onExpand()
      }}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault()
          event.stopPropagation()
          onExpand()
        }
      }}
    >
      {chart}
    </div>
  )
}

function FolderDialog({
  open,
  folder,
  onOpenChange,
  onSave,
}: {
  open: boolean
  folder?: ChannelFolder
  onOpenChange: (open: boolean) => void
  onSave: (name: string, color: string) => void
}) {
  const { t } = useTranslation()
  const [name, setName] = useState(() => folder?.name || '')
  const [color, setColor] = useState(() => folder?.color || FOLDER_COLORS[0])

  const submit = () => {
    const trimmed = name.trim()
    if (!trimmed) {
      toast.error(t('Folder name is required'))
      return
    }
    onSave(trimmed, color)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle>
            {folder ? t('Edit Folder') : t('Create Folder')}
          </DialogTitle>
          <DialogDescription>
            {t(
              'Folders are stored in this browser and do not change channel configuration.'
            )}
          </DialogDescription>
        </DialogHeader>
        <div className='grid gap-4 py-2'>
          <div className='grid gap-2'>
            <Label htmlFor='channel-folder-name'>{t('Folder Name')}</Label>
            <Input
              id='channel-folder-name'
              value={name}
              maxLength={40}
              onChange={(event) => setName(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') submit()
              }}
            />
          </div>
          <div className='grid gap-2'>
            <Label>{t('Folder Color')}</Label>
            <div className='flex flex-wrap gap-2'>
              {FOLDER_COLORS.map((option) => (
                <button
                  key={option}
                  type='button'
                  aria-label={option}
                  title={option}
                  className={cn(
                    'flex size-8 items-center justify-center rounded-md border-2 transition-transform hover:scale-105',
                    color === option
                      ? 'border-foreground'
                      : 'border-transparent'
                  )}
                  style={{ backgroundColor: option }}
                  onClick={() => setColor(option)}
                >
                  {color === option && <Check className='size-4 text-white' />}
                </button>
              ))}
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button onClick={submit}>
            <Check className='size-4' />
            {t('Save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function ChannelTile({
  channel,
  onOpen,
  onDragStart,
  onDragEnd,
}: {
  channel: Channel
  onOpen: (channel: Channel) => void
  onDragStart: (channelId: number) => void
  onDragEnd: () => void
}) {
  const { t } = useTranslation()
  const enabled = channel.status === CHANNEL_STATUS.ENABLED
  const icon = getLobeIcon(`${getChannelTypeIcon(channel.type)}.Color`, 22)

  const copyId = async (event: MouseEvent) => {
    event.stopPropagation()
    await copyChannelId(channel.id, t)
  }

  return (
    <Card
      size='sm'
      className='cursor-pointer gap-3 border-l-4 py-3 transition-shadow hover:shadow-md'
      style={{ borderLeftColor: enabled ? '#16a34a' : '#dc2626' }}
      onClick={() => onOpen(channel)}
    >
      <CardHeader className='grid-cols-[auto_1fr_auto] items-center gap-2 px-3'>
        <div className='bg-muted flex size-9 items-center justify-center rounded-md'>
          {icon}
        </div>
        <div className='min-w-0'>
          <CardTitle className='truncate text-sm'>{channel.name}</CardTitle>
          <div className='text-muted-foreground mt-0.5 truncate text-xs'>
            {t(getChannelTypeLabel(channel.type))}
          </div>
        </div>
        <Badge variant={enabled ? 'default' : 'destructive'}>
          {statusLabel(channel, t)}
        </Badge>
      </CardHeader>
      <CardContent className='grid gap-2 px-3'>
        <div className='flex items-center justify-between gap-2 text-xs'>
          <button
            type='button'
            draggable
            onClick={copyId}
            onDragStart={(event) => {
              event.stopPropagation()
              event.dataTransfer.effectAllowed = 'move'
              event.dataTransfer.setData('text/plain', String(channel.id))
              onDragStart(channel.id)
            }}
            onDragEnd={onDragEnd}
            className='text-muted-foreground hover:text-foreground inline-flex min-w-0 cursor-grab items-center gap-1 rounded px-1 py-0.5 font-mono active:cursor-grabbing'
            title={t('Click to copy, drag to move')}
          >
            <GripVertical className='size-3.5 shrink-0' />
            <Copy className='size-3 shrink-0' />
            <span className='truncate'>ID {channel.id}</span>
          </button>
          <span className='text-muted-foreground inline-flex items-center gap-1'>
            <Clock3 className='size-3.5' />
            {(Math.max(0, channel.response_time || 0) / 1000).toFixed(2)}s
          </span>
        </div>
        <div className='text-muted-foreground flex items-center justify-between text-xs'>
          <span className='truncate'>{channel.group || 'default'}</span>
          <span className='inline-flex items-center gap-1'>
            <Pencil className='size-3' />
            {t('Edit')}
          </span>
        </div>
      </CardContent>
    </Card>
  )
}

function FolderCard({
  folder,
  channels,
  stats,
  pulse,
  onEdit,
  onDelete,
  onOpenChannel,
  onDropChannel,
  onExpandChart,
  onDragStart,
  onDragEnd,
}: {
  folder: ChannelFolder
  channels: Channel[]
  stats: ChannelStats
  pulse: number
  onEdit: () => void
  onDelete: () => void
  onOpenChannel: (channel: Channel) => void
  onDropChannel: (channelId: number) => void
  onExpandChart: () => void
  onDragStart: (channelId: number) => void
  onDragEnd: () => void
}) {
  const { t } = useTranslation()
  const enabledCount = channels.filter(
    (channel) => channel.status === CHANNEL_STATUS.ENABLED
  ).length
  const unavailableCount = channels.length - enabledCount

  return (
    <Card
      className='group relative aspect-square min-h-[320px] border-t-4 transition-shadow hover:shadow-lg'
      style={{ borderTopColor: folder.color }}
      onDragOver={(event) => event.preventDefault()}
      onDrop={(event) => {
        event.preventDefault()
        const channelId = Number(event.dataTransfer.getData('text/plain'))
        if (Number.isInteger(channelId) && channelId > 0)
          onDropChannel(channelId)
      }}
    >
      <CardHeader className='grid-cols-[1fr_auto] gap-2 px-4 pb-1'>
        <div className='flex min-w-0 items-center gap-2'>
          <span
            className='size-3 shrink-0 rounded-sm'
            style={{ backgroundColor: folder.color }}
          />
          <CardTitle className='truncate text-base'>{folder.name}</CardTitle>
          <span className='text-muted-foreground shrink-0 text-xs'>
            {channels.length}
          </span>
        </div>
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button
                variant='ghost'
                size='icon-sm'
                aria-label={t('Folder actions')}
                onClick={(event) => event.stopPropagation()}
              />
            }
          >
            <MoreHorizontal className='size-4' />
          </DropdownMenuTrigger>
          <DropdownMenuContent align='end'>
            <DropdownMenuItem onClick={onEdit}>
              <Pencil className='size-4' />
              {t('Edit Folder')}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              className='text-destructive focus:text-destructive'
              onClick={onDelete}
            >
              <Trash2 className='size-4' />
              {t('Delete Folder')}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </CardHeader>

      <CardContent className='flex min-h-0 flex-1 flex-col gap-3 px-4'>
        <div className='grid grid-cols-3 gap-2 text-center'>
          <div>
            <div className='text-lg font-semibold tabular-nums'>
              {channels.length}
            </div>
            <div className='text-muted-foreground text-[11px]'>
              {t('Total')}
            </div>
          </div>
          <div>
            <div className='text-success text-lg font-semibold tabular-nums'>
              {enabledCount}
            </div>
            <div className='text-muted-foreground text-[11px]'>
              {t('Available')}
            </div>
          </div>
          <div>
            <div className='text-destructive text-lg font-semibold tabular-nums'>
              {unavailableCount}
            </div>
            <div className='text-muted-foreground text-[11px]'>
              {t('Unavailable')}
            </div>
          </div>
        </div>

        <div className='grid grid-cols-2 gap-2'>
          <div className='bg-muted/40 rounded-md px-2 py-1.5'>
            <div className='text-muted-foreground text-[11px]'>
              {t('Requests')}
            </div>
            <div className='text-sm font-semibold tabular-nums'>
              {formatCompact(stats.requests)}
            </div>
          </div>
          <div className='bg-muted/40 rounded-md px-2 py-1.5'>
            <div className='text-muted-foreground text-[11px]'>
              {t('Tokens')}
            </div>
            <div className='text-sm font-semibold tabular-nums'>
              {formatCompact(stats.tokens)}
            </div>
          </div>
        </div>

        <ChannelStatsChart
          stats={stats}
          pulse={pulse}
          onExpand={onExpandChart}
        />

        <div className='text-muted-foreground flex items-center justify-end gap-1 text-xs'>
          <Clock3 className='size-3.5' />
          {t('Average Latency')}:{' '}
          {stats.latencySamples > 0
            ? `${(stats.latencySum / stats.latencySamples).toFixed(2)}s`
            : '0s'}
        </div>

        <div className='text-muted-foreground flex min-h-0 flex-1 flex-wrap content-start gap-1 overflow-hidden text-xs'>
          {channels.slice(0, 5).map((channel) => (
            <div
              key={channel.id}
              className='bg-muted/50 inline-flex max-w-full items-center gap-1 rounded px-1.5 py-0.5'
            >
              <button
                type='button'
                draggable
                className='hover:text-foreground cursor-grab font-mono active:cursor-grabbing'
                onClick={(event) => {
                  event.stopPropagation()
                  void copyChannelId(channel.id, t)
                }}
                onDragStart={(event) => {
                  event.stopPropagation()
                  event.dataTransfer.effectAllowed = 'move'
                  event.dataTransfer.setData('text/plain', String(channel.id))
                  onDragStart(channel.id)
                }}
                onDragEnd={onDragEnd}
                title={t('Click to copy, drag to move')}
              >
                #{channel.id}
              </button>
              <button
                type='button'
                className='hover:text-foreground max-w-[8rem] truncate'
                onClick={(event) => {
                  event.stopPropagation()
                  onOpenChannel(channel)
                }}
              >
                {channel.name}
              </button>
            </div>
          ))}
          {channels.length > 5 && (
            <span className='px-1.5 py-0.5'>+{channels.length - 5}</span>
          )}
          {channels.length === 0 && (
            <span className='text-muted-foreground/70'>
              {t('Drop channels here')}
            </span>
          )}
        </div>
      </CardContent>
    </Card>
  )
}

function ChartDialog({
  open,
  onOpenChange,
  folderName,
  stats,
  pulse,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  folderName: string
  stats: ChannelStats
  pulse: number
}) {
  const { t } = useTranslation()
  const averageLatency =
    stats.latencySamples > 0 ? stats.latencySum / stats.latencySamples : 0
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-w-5xl'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            <BarChart3 className='size-4' />
            {folderName}
          </DialogTitle>
          <DialogDescription>
            {t('Request volume and latency by time bucket')}
          </DialogDescription>
        </DialogHeader>
        <div className='grid gap-3 sm:grid-cols-3'>
          <div className='bg-muted/40 rounded-md px-3 py-2'>
            <div className='text-muted-foreground text-xs'>{t('Requests')}</div>
            <div className='text-xl font-semibold tabular-nums'>
              {formatCompact(stats.requests)}
            </div>
          </div>
          <div className='bg-muted/40 rounded-md px-3 py-2'>
            <div className='text-muted-foreground text-xs'>{t('Tokens')}</div>
            <div className='text-xl font-semibold tabular-nums'>
              {formatCompact(stats.tokens)}
            </div>
          </div>
          <div className='bg-muted/40 rounded-md px-3 py-2'>
            <div className='text-muted-foreground text-xs'>
              {t('Average Latency')}
            </div>
            <div className='text-xl font-semibold tabular-nums'>
              {averageLatency.toFixed(2)}s
            </div>
          </div>
        </div>
        <ChannelStatsChart stats={stats} pulse={pulse} large />
      </DialogContent>
    </Dialog>
  )
}

export function ChannelsModernView() {
  const { t } = useTranslation()
  const { setOpen, setCurrentRow, folderDialogOpen, setFolderDialogOpen } =
    useChannels()
  const [folders, setFolders] = useState<ChannelFolder[]>(readFolders)
  const [editingFolder, setEditingFolder] = useState<ChannelFolder>()
  const [expandedFolderId, setExpandedFolderId] = useState<string | null>(null)
  const [draggingId, setDraggingId] = useState<number | null>(null)
  const [range, setRange] = useState<DateRange>(() => getDateRange(7))
  const [rangeDraft, setRangeDraft] = useState<DateRange>(() => getDateRange(7))
  const [rangeDialogOpen, setRangeDialogOpen] = useState(false)

  const channelsQuery = useQuery({
    queryKey: [...channelsQueryKeys.all, 'modern-all'],
    queryFn: fetchAllChannels,
    refetchInterval: 60_000,
    refetchOnWindowFocus: false,
  })
  const channels = useMemo(() => channelsQuery.data ?? [], [channelsQuery.data])
  const channelIdsKey = channels.map((channel) => channel.id).join(',')

  const statsQuery = useQuery({
    queryKey: [
      ...channelsQueryKeys.all,
      'modern-stats',
      channelIdsKey,
      range.start.getTime(),
      range.end.getTime(),
      channels,
      range,
    ],
    enabled: channels.length > 0,
    queryFn: async () => {
      const entries = await mapWithConcurrency(channels, 4, async (channel) => {
        try {
          return [channel.id, await fetchChannelStats(channel, range)] as const
        } catch {
          return [channel.id, EMPTY_STATS] as const
        }
      })
      return Object.fromEntries(entries) as Record<number, ChannelStats>
    },
    refetchInterval: 60_000,
    refetchOnWindowFocus: false,
    placeholderData: (previousData) => previousData,
  })

  const pulse = statsQuery.dataUpdatedAt
  const statsByChannel = statsQuery.data ?? EMPTY_STATS_BY_CHANNEL
  const folderChannelIds = useMemo(
    () => new Set(folders.flatMap((folder) => folder.channelIds)),
    [folders]
  )
  const ungroupedChannels = channels.filter(
    (channel) => !folderChannelIds.has(channel.id)
  )
  const totalStats = mergeStats(
    channels.map((channel) => channel.id),
    statsByChannel
  )

  const updateFolders = useCallback(
    (updater: (folders: ChannelFolder[]) => ChannelFolder[]) => {
      setFolders((previous) => {
        const next = updater(previous)
        persistFolders(next)
        return next
      })
    },
    []
  )

  const openChannel = useCallback(
    (channel: Channel) => {
      setCurrentRow(channel)
      setOpen('update-channel')
    },
    [setCurrentRow, setOpen]
  )

  const saveFolder = (name: string, color: string) => {
    if (editingFolder) {
      updateFolders((previous) =>
        previous.map((folder) =>
          folder.id === editingFolder.id ? { ...folder, name, color } : folder
        )
      )
    } else {
      updateFolders((previous) => [
        ...previous,
        {
          id: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
          name,
          color,
          channelIds: [],
        },
      ])
    }
    setEditingFolder(undefined)
    setFolderDialogOpen(false)
  }

  const moveChannel = (channelId: number, folderId: string) => {
    updateFolders((previous) =>
      previous.map((folder) => ({
        ...folder,
        channelIds:
          folder.id === folderId
            ? Array.from(new Set([...folder.channelIds, channelId]))
            : folder.channelIds.filter((id) => id !== channelId),
      }))
    )
    setDraggingId(null)
  }

  const chooseRange = (days: number) => {
    const next = getDateRange(days)
    setRange(next)
    setRangeDraft(next)
    setRangeDialogOpen(false)
  }

  const applyCustomRange = () => {
    const duration = rangeDraft.end.getTime() - rangeDraft.start.getTime()
    if (duration <= 0) {
      toast.error(t('End time must be after start time'))
      return
    }
    if (duration > 90 * 86400000) {
      toast.error(t('Statistics can cover up to 90 days'))
      return
    }
    setRange({ ...rangeDraft, days: null })
    setRangeDialogOpen(false)
  }

  const rangeLabel = range.days ? `${range.days}d` : t('Custom range')

  return (
    <div className='space-y-4'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <div className='flex min-w-0 items-center gap-2'>
          <div className='text-muted-foreground text-sm'>
            {channelsQuery.isLoading
              ? t('Loading channels...')
              : t('{{count}} channels', { count: channels.length })}
          </div>
          {statsQuery.isFetching && (
            <RefreshCw className='text-muted-foreground size-3.5 animate-spin' />
          )}
        </div>
        <div className='flex flex-wrap items-center gap-2'>
          <div className='flex items-center gap-1 rounded-md border p-0.5'>
            {[1, 2, 3].map((days) => (
              <Button
                key={days}
                size='sm'
                variant={range.days === days ? 'secondary' : 'ghost'}
                className='h-7 px-2 text-xs'
                onClick={() => chooseRange(days)}
              >
                {days}d
              </Button>
            ))}
          </div>
          <Button
            size='sm'
            variant='outline'
            onClick={() => {
              setRangeDraft(range)
              setRangeDialogOpen(true)
            }}
          >
            <CalendarDays className='size-4' />
            <span>{rangeLabel}</span>
          </Button>
          <Button
            size='sm'
            variant='outline'
            onClick={() => statsQuery.refetch()}
            disabled={statsQuery.isFetching}
            title={t('Refresh statistics')}
          >
            <RefreshCw
              className={cn('size-4', statsQuery.isFetching && 'animate-spin')}
            />
            <span className='max-sm:hidden'>{t('Refresh')}</span>
          </Button>
        </div>
      </div>

      <div className='grid gap-2 sm:grid-cols-3'>
        <div className='bg-muted/30 rounded-lg border px-3 py-2'>
          <div className='text-muted-foreground text-xs'>
            {t('Total Requests')}
          </div>
          <div className='text-lg font-semibold tabular-nums'>
            {formatCompact(totalStats.requests)}
          </div>
        </div>
        <div className='bg-muted/30 rounded-lg border px-3 py-2'>
          <div className='text-muted-foreground text-xs'>
            {t('Total Tokens')}
          </div>
          <div className='text-lg font-semibold tabular-nums'>
            {formatCompact(totalStats.tokens)}
          </div>
        </div>
        <div className='bg-muted/30 rounded-lg border px-3 py-2'>
          <div className='text-muted-foreground text-xs'>
            {t('Average Latency')}
          </div>
          <div className='text-lg font-semibold tabular-nums'>
            {totalStats.latencySamples > 0
              ? `${(totalStats.latencySum / totalStats.latencySamples).toFixed(2)}s`
              : '0s'}
          </div>
        </div>
      </div>

      {channelsQuery.isLoading ? (
        <div className='grid gap-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4'>
          {Array.from({ length: 4 }, (_, index) => (
            <Skeleton
              key={index}
              className='aspect-square min-h-[320px] rounded-xl'
            />
          ))}
        </div>
      ) : (
        <>
          {folders.length > 0 && (
            <div className='grid gap-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4'>
              {folders.map((folder) => {
                const folderChannels = channels.filter((channel) =>
                  folder.channelIds.includes(channel.id)
                )
                return (
                  <FolderCard
                    key={folder.id}
                    folder={folder}
                    channels={folderChannels}
                    stats={mergeStats(folder.channelIds, statsByChannel)}
                    pulse={pulse}
                    onEdit={() => {
                      setEditingFolder(folder)
                      setFolderDialogOpen(true)
                    }}
                    onDelete={() => {
                      updateFolders((previous) =>
                        previous.filter((item) => item.id !== folder.id)
                      )
                    }}
                    onOpenChannel={openChannel}
                    onDropChannel={(channelId) =>
                      moveChannel(channelId, folder.id)
                    }
                    onExpandChart={() => setExpandedFolderId(folder.id)}
                    onDragStart={setDraggingId}
                    onDragEnd={() => setDraggingId(null)}
                  />
                )
              })}
            </div>
          )}

          <section className='space-y-3'>
            <div className='flex items-center justify-between'>
              <div className='flex items-center gap-2'>
                <h3 className='text-sm font-semibold'>
                  {t('Unfiled Channels')}
                </h3>
                <Badge variant='outline'>{ungroupedChannels.length}</Badge>
              </div>
              {draggingId !== null && (
                <span className='text-muted-foreground text-xs'>
                  {t('Drop onto a folder to organize')}
                </span>
              )}
            </div>
            {ungroupedChannels.length > 0 ? (
              <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4'>
                {ungroupedChannels.map((channel) => (
                  <ChannelTile
                    key={channel.id}
                    channel={channel}
                    onOpen={openChannel}
                    onDragStart={setDraggingId}
                    onDragEnd={() => setDraggingId(null)}
                  />
                ))}
              </div>
            ) : (
              <div className='text-muted-foreground rounded-lg border border-dashed px-4 py-8 text-center text-sm'>
                {channels.length === 0
                  ? t('No channels found')
                  : t('All channels are organized in folders')}
              </div>
            )}
          </section>
        </>
      )}

      <FolderDialog
        key={`${editingFolder?.id || 'new-folder'}-${folderDialogOpen}`}
        open={folderDialogOpen}
        folder={editingFolder}
        onOpenChange={(open) => {
          setFolderDialogOpen(open)
          if (!open) setEditingFolder(undefined)
        }}
        onSave={saveFolder}
      />

      <Dialog open={rangeDialogOpen} onOpenChange={setRangeDialogOpen}>
        <DialogContent className='sm:max-w-lg'>
          <DialogHeader>
            <DialogTitle>{t('Statistics Range')}</DialogTitle>
            <DialogDescription>
              {t('Choose a period for folder request and token statistics.')}
            </DialogDescription>
          </DialogHeader>
          <div className='grid gap-4 py-2'>
            <div className='flex flex-wrap gap-2'>
              {[7, 14, 30, 90].map((days) => (
                <Button
                  key={days}
                  type='button'
                  size='sm'
                  variant='outline'
                  onClick={() => chooseRange(days)}
                >
                  {days}d
                </Button>
              ))}
            </div>
            <div className='grid gap-2 sm:grid-cols-2'>
              <div className='grid gap-2'>
                <Label>{t('Start Time')}</Label>
                <DateTimePicker
                  value={rangeDraft.start}
                  onChange={(date) =>
                    date &&
                    setRangeDraft((previous) => ({
                      ...previous,
                      start: date,
                      days: null,
                    }))
                  }
                />
              </div>
              <div className='grid gap-2'>
                <Label>{t('End Time')}</Label>
                <DateTimePicker
                  value={rangeDraft.end}
                  onChange={(date) =>
                    date &&
                    setRangeDraft((previous) => ({
                      ...previous,
                      end: date,
                      days: null,
                    }))
                  }
                />
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant='outline' onClick={() => setRangeDialogOpen(false)}>
              {t('Cancel')}
            </Button>
            <Button onClick={applyCustomRange}>{t('Apply')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {expandedFolderId && (
        <ChartDialog
          open
          onOpenChange={(open) => !open && setExpandedFolderId(null)}
          folderName={
            folders.find((folder) => folder.id === expandedFolderId)?.name ||
            t('Folder')
          }
          stats={mergeStats(
            folders.find((folder) => folder.id === expandedFolderId)
              ?.channelIds || [],
            statsByChannel
          )}
          pulse={pulse}
        />
      )}
    </div>
  )
}
