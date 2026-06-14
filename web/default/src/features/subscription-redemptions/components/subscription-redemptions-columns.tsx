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
import { type ColumnDef } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { formatTimestampToDate } from '@/lib/format'
import { Checkbox } from '@/components/ui/checkbox'
import { DataTableColumnHeader } from '@/components/data-table'
import { MaskedValueDisplay } from '@/components/masked-value-display'
import { StatusBadge } from '@/components/status-badge'
import {
  SUBSCRIPTION_REDEMPTION_STATUS,
  SUBSCRIPTION_REDEMPTION_STATUS_LABELS,
} from '../constants'
import { isSubscriptionRedemptionExpired } from '../lib'
import { type SubscriptionRedemption } from '../types'
import { DataTableRowActions } from './data-table-row-actions'

export function useSubscriptionRedemptionsColumns(): ColumnDef<SubscriptionRedemption>[] {
  const { t } = useTranslation()
  return [
    {
      id: 'select',
      meta: { label: t('Select') },
      header: ({ table }) => (
        <Checkbox
          checked={table.getIsAllPageRowsSelected()}
          indeterminate={table.getIsSomePageRowsSelected()}
          onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
          aria-label={t('Select all')}
          className='translate-y-[2px]'
        />
      ),
      cell: ({ row }) => (
        <Checkbox
          checked={row.getIsSelected()}
          onCheckedChange={(value) => row.toggleSelected(!!value)}
          aria-label={t('Select row')}
          className='translate-y-[2px]'
        />
      ),
      enableSorting: false,
      enableHiding: false,
    },
    {
      accessorKey: 'id',
      meta: { label: t('ID'), mobileHidden: true },
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('ID')} />
      ),
      cell: ({ row }) => {
        return <div className='w-[60px]'>{row.getValue('id')}</div>
      },
    },
    {
      accessorKey: 'name',
      meta: { label: t('Name'), mobileTitle: true },
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Name')} />
      ),
      cell: ({ row }) => {
        return (
          <div className='max-w-[150px] truncate font-medium'>
            {row.getValue('name')}
          </div>
        )
      },
    },
    {
      accessorKey: 'status',
      meta: { label: t('Status'), mobileBadge: true },
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Status')} />
      ),
      cell: ({ row }) => {
        const redemption = row.original
        const statusValue = row.getValue('status') as number

        // Check if expired
        if (isSubscriptionRedemptionExpired(redemption.expired_time, statusValue)) {
          return (
            <StatusBadge
              label={t('Expired')}
              variant='warning'
              showDot={true}
              copyable={false}
            />
          )
        }

        const statusLabel = SUBSCRIPTION_REDEMPTION_STATUS_LABELS[statusValue]

        if (!statusLabel) {
          return null
        }

        const variantMap: Record<number, 'success' | 'secondary' | 'warning' | 'neutral'> = {
          [SUBSCRIPTION_REDEMPTION_STATUS.ENABLED]: 'success',
          [SUBSCRIPTION_REDEMPTION_STATUS.DISABLED]: 'warning',
          [SUBSCRIPTION_REDEMPTION_STATUS.USED]: 'secondary',
        }

        return (
          <StatusBadge
            label={t(statusLabel)}
            variant={variantMap[statusValue] || 'neutral'}
            showDot={statusValue === SUBSCRIPTION_REDEMPTION_STATUS.ENABLED}
            copyable={false}
          />
        )
      },
    },
    {
      id: 'code',
      accessorKey: 'key',
      meta: { label: t('Code') },
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Code')} />
      ),
      cell: function CodeCell({ row }) {
        const redemption = row.original
        const key = redemption.key
        const maskedKey = `${key.slice(0, 8)}${'*'.repeat(16)}${key.slice(-8)}`

        return (
          <MaskedValueDisplay
            label={t('Full Code')}
            fullValue={key}
            maskedValue={maskedKey}
            copyTooltip={t('Copy code')}
            copyAriaLabel={t('Copy subscription redemption code')}
          />
        )
      },
      enableSorting: false,
    },
    {
      accessorKey: 'plan_id',
      meta: { label: t('Plan ID') },
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Plan ID')} />
      ),
      cell: ({ row }) => {
        return <div className='w-[80px]'>{row.getValue('plan_id')}</div>
      },
    },
    {
      accessorKey: 'created_time',
      meta: { label: t('Created'), mobileHidden: true },
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Created')} />
      ),
      cell: ({ row }) => {
        return (
          <div className='min-w-[140px] font-mono text-sm'>
            {formatTimestampToDate(row.getValue('created_time'))}
          </div>
        )
      },
    },
    {
      accessorKey: 'expired_time',
      meta: { label: t('Expires'), mobileHidden: true },
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Expires')} />
      ),
      cell: ({ row }) => {
        const expiredTime = row.getValue('expired_time') as number
        if (expiredTime === 0) {
          return (
            <StatusBadge
              label={t('Never Expires')}
              variant='neutral'
              copyable={false}
            />
          )
        }
        return (
          <div className='min-w-[140px] font-mono text-sm'>
            {formatTimestampToDate(expiredTime)}
          </div>
        )
      },
    },
    {
      id: 'actions',
      meta: { label: t('Actions') },
      cell: ({ row }) => <DataTableRowActions row={row} />,
    },
  ]
}