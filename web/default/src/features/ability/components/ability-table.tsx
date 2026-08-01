/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Software License as published by
the Free Software Foundation, either version 3 of the License, or (at your
option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Software License for more details.

You should have received a copy of the GNU Affero General Software License
along with this program. If not, see <https://www.gnu.org/licenses/>.
*/
import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { type ColumnDef, getCoreRowModel, useReactTable } from '@tanstack/react-table'
import { getRouteApi } from '@tanstack/react-router'
import { DataTablePage } from '@/components/data-table'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { getAbilities } from '../api'
import type { Ability } from '../types'

const route = getRouteApi('/_authenticated/system-settings/ability/')

export function AbilityTable() {
  const navigate = route.useNavigate()
  const search = route.useSearch()

  const { globalFilter, onGlobalFilterChange, pagination, onPaginationChange } =
    useTableUrlState({
      search: search as Record<string, unknown>,
      navigate: navigate as never,
    })

  const { data, isLoading } = useQuery({
    queryKey: ['abilities', 'list', { filter: globalFilter, ...pagination }],
    queryFn: () =>
      getAbilities({
        keyword: globalFilter || undefined,
        p: pagination.pageIndex + 1,
        page_size: pagination.pageSize,
      }),
    placeholderData: (previousData) => previousData,
  })

  const abilities = useMemo(() => data?.data?.items ?? [], [data])
  const totalCount = data?.data?.total ?? 0

  const columns = useMemo<ColumnDef<Ability>[]>(
    () => [
      {
        accessorKey: 'id',
        header: 'ID',
        cell: ({ row }) => <span className="text-muted-foreground">{row.original.id}</span>,
      },
      {
        accessorKey: 'group',
        header: 'Group',
        cell: ({ row }) => <span className="font-mono text-xs">{row.original.group}</span>,
      },
      {
        accessorKey: 'model',
        header: 'Model',
        cell: ({ row }) => <span className="font-mono text-xs">{row.original.model}</span>,
      },
      {
        accessorKey: 'channel_id',
        header: 'Channel ID',
        cell: ({ row }) => <span>{row.original.channel_id}</span>,
      },
      {
        accessorKey: 'channel_name',
        header: 'Channel',
        cell: ({ row }) => <span>{row.original.channel_name || '-'}</span>,
      },
      {
        accessorKey: 'channel_type',
        header: 'Type',
        cell: ({ row }) => <span>{row.original.channel_type}</span>,
      },
      {
        accessorKey: 'priority',
        header: 'Priority',
        cell: ({ row }) => <span>{row.original.priority}</span>,
      },
      {
        accessorKey: 'weight',
        header: 'Weight',
        cell: ({ row }) => <span>{row.original.weight}</span>,
      },
      {
        accessorKey: 'enabled',
        header: 'Enabled',
        cell: ({ row }) =>
          row.original.enabled ? (
            <span className="text-green-600">Yes</span>
          ) : (
            <span className="text-muted-foreground">No</span>
          ),
      },
      {
        accessorKey: 'tag',
        header: 'Tag',
        cell: ({ row }) =>
          row.original.tag ? (
            <span className="rounded bg-muted px-1.5 py-0.5 text-xs">{row.original.tag}</span>
          ) : (
            <span className="text-muted-foreground">-</span>
          ),
      },
    ],
    []
  )

  const table = useReactTable({
    data: abilities,
    columns,
    pageCount: Math.ceil(totalCount / pagination.pageSize),
    state: {
      pagination,
      globalFilter,
    },
    onPaginationChange,
    onGlobalFilterChange,
    manualPagination: true,
    getCoreRowModel: getCoreRowModel(),
  })

  return (
    <DataTablePage<Ability>
      table={table}
      columns={columns}
      isLoading={isLoading}
      emptyTitle="No abilities"
      emptyDescription="No abilities match the current filters."
    />
  )
}
