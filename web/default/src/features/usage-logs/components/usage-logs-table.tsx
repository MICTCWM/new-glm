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
import { useEffect, useMemo, useRef } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import {
  type ColumnDef,
  flexRender,
  getCoreRowModel,
  getFacetedRowModel,
  getFacetedUniqueValues,
  getFilteredRowModel,
  getPaginationRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { motion } from 'motion/react'
import { useMediaQuery } from '@/hooks'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { MOTION_TRANSITION } from '@/lib/motion'
import { useIsAdmin } from '@/hooks/use-admin'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { TableCell } from '@/components/ui/table'
import { DataTablePage } from '@/components/data-table'
import { DEFAULT_LOGS_DATA, LOG_TYPE_ENUM } from '../constants'
import { useColumnsByCategory } from '../lib/columns'
import { fetchLogsByCategory } from '../lib/utils'
import type { LogCategory } from '../types'
import { CommonLogsFilterBar } from './common-logs-filter-bar'
import { TaskLogsFilterBar } from './task-logs-filter-bar'

const route = getRouteApi('/_authenticated/usage-logs/$section')

const logTypeRowTint: Record<number, string> = {
  [LOG_TYPE_ENUM.ERROR]: 'bg-rose-50/40 dark:bg-rose-950/20',
  [LOG_TYPE_ENUM.REFUND]: 'bg-blue-50/30 dark:bg-blue-950/15',
}

interface UsageLogsTableProps {
  logCategory: LogCategory
}

export function UsageLogsTable({ logCategory }: UsageLogsTableProps) {
  const { t } = useTranslation()
  const isAdmin = useIsAdmin()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const searchParams = route.useSearch()

  const {
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: route.useSearch(),
    navigate: route.useNavigate(),
    pagination: { defaultPage: 1, defaultPageSize: isMobile ? 20 : 100 },
    globalFilter: { enabled: false },
    columnFilters: [
      { columnId: 'created_at', searchKey: 'type', type: 'array' as const },
      { columnId: 'model_name', searchKey: 'model', type: 'string' as const },
      { columnId: 'token_name', searchKey: 'token', type: 'string' as const },
      { columnId: 'group', searchKey: 'group', type: 'string' as const },
      ...(isAdmin
        ? [
            {
              columnId: 'channel',
              searchKey: 'channel',
              type: 'string' as const,
            },
            {
              columnId: 'username',
              searchKey: 'username',
              type: 'string' as const,
            },
          ]
        : []),
    ],
  })

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'logs',
      logCategory,
      isAdmin,
      pagination.pageIndex + 1,
      pagination.pageSize,
      columnFilters,
      searchParams,
      t,
    ],
    queryFn: async () => {
      const result = await fetchLogsByCategory({
        logCategory,
        isAdmin,
        page: pagination.pageIndex + 1,
        pageSize: pagination.pageSize,
        searchParams,
        columnFilters,
      })

      if (!result?.success) {
        toast.error(result?.message || t('Failed to load logs'))
        return DEFAULT_LOGS_DATA
      }

      return result.data || DEFAULT_LOGS_DATA
    },
    placeholderData: (previousData, previousQuery) => {
      if (previousQuery?.queryKey[1] === logCategory) {
        return previousData
      }
      return undefined
    },
    // Auto-refresh every 1s so new logs stream in without a manual search.
    // TanStack Query v5 defaults to refetchIntervalInBackground: false, so
    // background tabs won't waste requests. Explicitly set it here for clarity.
    refetchInterval: 1000,
    refetchIntervalInBackground: false,
    // A polling response only updates existing rows. Don't trigger an extra
    // refetch on tab focus and keep the current data while the request is in
    // flight (handled by placeholderData above).
    refetchOnWindowFocus: false,
  })

  const logs = data?.items || []
  const columns = useColumnsByCategory(logCategory, isAdmin)
  const isLoadingData = isLoading || (isFetching && !data)

  const table = useReactTable({
    data: logs as Record<string, unknown>[],
    columns: columns as ColumnDef<Record<string, unknown>>[],
    state: {
      columnFilters,
      pagination,
    },
    enableRowSelection: false,
    onPaginationChange,
    onColumnFiltersChange,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getFacetedRowModel: getFacetedRowModel(),
    getFacetedUniqueValues: getFacetedUniqueValues(),
    manualPagination: true,
    pageCount: Math.ceil((data?.total || 0) / pagination.pageSize),
  })

  const pageCount = table.getPageCount()
  useEffect(() => {
    ensurePageInRange(pageCount)
  }, [pageCount, ensurePageInRange])

  const isCommon = logCategory === 'common'

  // ---- Auto-refresh support: scroll-following + new-row animation ----
  const scrollContainerRef = useRef<HTMLDivElement>(null)
  // Tracks whether the user was reading near the top of the scroll
  // container *before* the latest refresh. Updated by a passive scroll
  // listener so the value is always current when new data arrives.
  // Initialised to `false` so auto-scroll only kicks in after the user
  // actively scrolls to the top (latest logs appear at the top by default).
  const wasAtTopRef = useRef(false)
  // Distinguishes the very first successful data paint from subsequent
  // polling refreshes — we don't want to auto-scroll on the initial load.
  const hasInitialDataRef = useRef(false)

  useEffect(() => {
    const el = scrollContainerRef.current
    if (!el) return
    const handleScroll = () => {
      wasAtTopRef.current = el.scrollTop < 100
    }
    el.addEventListener('scroll', handleScroll, { passive: true })
    return () => el.removeEventListener('scroll', handleScroll)
  }, [])

  // When refreshed data arrives, if the user was near the top, follow the
  // new top so newly-arrived rows stay visible. Skip the first paint so
  // the initial view starts at the top (latest logs).
  useEffect(() => {
    if (!hasInitialDataRef.current) {
      if (data) hasInitialDataRef.current = true
      return
    }
    if (!wasAtTopRef.current) return
    const el = scrollContainerRef.current
    if (!el) return
    el.scrollTo({ top: 0, behavior: 'smooth' })
  }, [data])

  // Track previous row ids so we can detect newly-inserted rows on each
  // refresh and only animate those (avoid re-animating the whole list).
  const rows = table.getRowModel().rows
  const prevRowIdsRef = useRef<Set<string>>(new Set())
  const newRowIds = useMemo(() => {
    const prev = prevRowIdsRef.current
    return new Set(rows.filter((r) => !prev.has(r.id)).map((r) => r.id))
  }, [rows])
  useEffect(() => {
    prevRowIdsRef.current = new Set(rows.map((r) => r.id))
  }, [rows])

  // If a refresh inserts more than 10 new rows at once, skip per-row animation
  // to avoid jank (e.g. after a filter change or first load).
  const skipRowAnimation = newRowIds.size > 10
  const rowTransition = skipRowAnimation ? { duration: 0 } : MOTION_TRANSITION.fast

  return (
    <DataTablePage
      table={table}
      columns={columns as ColumnDef<Record<string, unknown>>[]}
      isLoading={isLoadingData}
      emptyTitle={t('No Logs Found')}
      emptyDescription={t(
        'No usage logs available. Logs will appear here once API calls are made.'
      )}
      skeletonKeyPrefix='usage-log-skeleton'
      tableClassName='max-h-[calc(100dvh-13rem)] overflow-auto sm:max-h-[calc(100dvh-14rem)]'
      tableHeaderClassName='bg-muted/30 sticky top-0 z-10'
      scrollContainerRef={scrollContainerRef}
      toolbar={
        isCommon ? (
          <CommonLogsFilterBar table={table} />
        ) : (
          <TaskLogsFilterBar table={table} logCategory={logCategory} />
        )
      }
      renderRow={(row) => {
        const logType = (row.original as Record<string, unknown>).type as
          | number
          | undefined
        const tintClass =
          isCommon && logType != null ? (logTypeRowTint[logType] ?? '') : ''
        const isNewRow = newRowIds.has(row.id)

        return (
          <motion.tr
            key={row.id}
            data-slot='table-row'
            data-state={row.getIsSelected() && 'selected'}
            initial={
              isNewRow && !skipRowAnimation ? { opacity: 0, y: -24 } : false
            }
            animate={{ opacity: 1, y: 0 }}
            transition={
              isNewRow && !skipRowAnimation
                ? {
                    type: 'spring',
                    stiffness: 300,
                    damping: 30,
                    opacity: { duration: 0.3 },
                  }
                : rowTransition
            }
            className={cn(
              'hover:bg-muted/50 has-aria-expanded:bg-muted/50 data-[state=selected]:bg-muted border-b transition-colors',
              tintClass
            )}
          >
            {row.getVisibleCells().map((cell) => (
              <TableCell key={cell.id} className={isCommon ? 'py-2' : 'py-3.5'}>
                {flexRender(cell.column.columnDef.cell, cell.getContext())}
              </TableCell>
            ))}
          </motion.tr>
        )
      }}
    />
  )
}
