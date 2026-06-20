/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  flexRender,
  getCoreRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  useReactTable,
  type ColumnDef,
  type RowSelectionState,
} from '@tanstack/react-table'
import { Search } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useQuery } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { DataTablePagination } from '@/components/data-table/pagination'
import { searchUsers } from '@/features/users/api'
import type { User } from '@/features/users/types'

type SpecialUserPickerDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  selectedUserIds: number[]
  onConfirm: (ids: number[]) => void
}

function getUserDisplayName(user: User): string {
  const displayName = user.display_name?.trim()
  const username = user.username?.trim()
  const primaryName = displayName || username || `#${user.id}`
  const secondaryName =
    username && displayName && username !== displayName
      ? ` / ${username}`
      : ''
  return `${primaryName}${secondaryName} (#${user.id})`
}

export function SpecialUserPickerDialog({
  open,
  onOpenChange,
  selectedUserIds,
  onConfirm,
}: SpecialUserPickerDialogProps) {
  const { t } = useTranslation()
  const [search, setSearch] = useState('')
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({})

  const { data: usersData, isFetching } = useQuery({
    queryKey: ['special_user_picker', search],
    queryFn: () =>
      searchUsers({
        keyword: search,
        p: 1,
        page_size: 100,
      }),
    enabled: open,
  })

  const users = useMemo(() => usersData?.data?.items || [], [usersData])

  useEffect(() => {
    if (!open) return
    const newSelection: RowSelectionState = {}
    const availableIds = new Set(users.map((u) => u.id))
    selectedUserIds.forEach((id) => {
      if (availableIds.has(id)) {
        newSelection[id.toString()] = true
      }
    })
    setRowSelection(newSelection)
  }, [selectedUserIds, users, open])

  const handleConfirm = useCallback(() => {
    const selectedIds = Object.keys(rowSelection)
      .filter((key) => rowSelection[key])
      .map(Number)
      .filter((n) => Number.isInteger(n) && n > 0)

    const idsNotInCurrentPage = selectedUserIds.filter(
      (id) => !users.some((u) => u.id === id)
    )
    const finalIds = [
      ...idsNotInCurrentPage,
      ...selectedIds.filter(
        (id) => !idsNotInCurrentPage.includes(id) && users.some((u) => u.id === id)
      ),
    ]

    onConfirm(finalIds)
    onOpenChange(false)
  }, [rowSelection, selectedUserIds, users, onConfirm, onOpenChange])

  const columns = useMemo<ColumnDef<User>[]>(
    () => [
      {
        id: 'select',
        header: ({ table }) => (
          <Checkbox
            checked={table.getIsAllPageRowsSelected()}
            indeterminate={table.getIsSomePageRowsSelected()}
            onCheckedChange={(value) =>
              table.toggleAllPageRowsSelected(!!value)
            }
            aria-label={t('Select all')}
          />
        ),
        cell: ({ row }) => (
          <Checkbox
            checked={row.getIsSelected()}
            onCheckedChange={(value) => row.toggleSelected(!!value)}
            aria-label={t('Select row')}
          />
        ),
        enableSorting: false,
        enableHiding: false,
      },
      {
        id: 'name',
        header: t('User'),
        cell: ({ row }) => {
          const user = row.original
          return (
            <span className='font-medium'>{getUserDisplayName(user)}</span>
          )
        },
      },
      {
        accessorKey: 'group',
        header: t('Group'),
      },
      {
        accessorKey: 'email',
        header: t('Email'),
        cell: ({ row }) => {
          const email = row.getValue('email') as string | undefined
          return (
            <span className='text-muted-foreground text-xs'>
              {email || '-'}
            </span>
          )
        },
      },
    ],
    [t]
  )

  const table = useReactTable({
    data: users,
    columns,
    state: {
      rowSelection,
    },
    getRowId: (row) => row.id.toString(),
    enableRowSelection: true,
    onRowSelectionChange: setRowSelection,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    initialState: {
      pagination: {
        pageSize: 10,
      },
    },
  })

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='flex max-h-[85vh] max-w-[calc(100%-2rem)] flex-col sm:max-w-[700px]'>
        <DialogHeader>
          <DialogTitle>{t('Select Users')}</DialogTitle>
          <DialogDescription>
            {t(
              'Select users who can call models provided by this channel.'
            )}
          </DialogDescription>
        </DialogHeader>

        <div className='flex flex-1 flex-col gap-3 overflow-hidden'>
          <div className='flex items-center gap-2'>
            <div className='relative flex-1'>
              <Search className='text-muted-foreground absolute top-1/2 left-2 h-4 w-4 -translate-y-1/2' />
              <Input
                placeholder={t('Search users...')}
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className='ps-8'
              />
            </div>
          </div>

          <div className='flex-1 overflow-auto rounded-md border'>
            <Table>
              <TableHeader>
                {table.getHeaderGroups().map((headerGroup) => (
                  <TableRow key={headerGroup.id}>
                    {headerGroup.headers.map((header) => (
                      <TableHead key={header.id}>
                        {header.isPlaceholder
                          ? null
                          : flexRender(
                              header.column.columnDef.header,
                              header.getContext()
                            )}
                      </TableHead>
                    ))}
                  </TableRow>
                ))}
              </TableHeader>
              <TableBody>
                {isFetching ? (
                  <TableRow>
                    <TableCell
                      colSpan={columns.length}
                      className='h-24 text-center'
                    >
                      {t('Loading...')}
                    </TableCell>
                  </TableRow>
                ) : table.getRowModel().rows?.length ? (
                  table.getRowModel().rows.map((row) => (
                    <TableRow
                      key={row.id}
                      data-state={row.getIsSelected() && 'selected'}
                    >
                      {row.getVisibleCells().map((cell) => (
                        <TableCell key={cell.id}>
                          {flexRender(
                            cell.column.columnDef.cell,
                            cell.getContext()
                          )}
                        </TableCell>
                      ))}
                    </TableRow>
                  ))
                ) : (
                  <TableRow>
                    <TableCell
                      colSpan={columns.length}
                      className='h-24 text-center'
                    >
                      {t('No users found')}
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>

          <DataTablePagination table={table} />
        </div>

        <DialogFooter>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button onClick={handleConfirm}>{t('Confirm')}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
