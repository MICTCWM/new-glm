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
import { type Row } from '@tanstack/react-table'
import {
  Trash2,
  Edit,
  Power,
  PowerOff,
  MoreHorizontal as DotsHorizontalIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { updateSubscriptionRedemptionStatus } from '../api'
import { SUBSCRIPTION_REDEMPTION_STATUS } from '../constants'
import { isSubscriptionRedemptionExpired } from '../lib'
import { subscriptionRedemptionSchema } from '../types'
import { useSubscriptionRedemptions } from './subscription-redemptions-provider'

interface DataTableRowActionsProps<TData> {
  row: Row<TData>
}

export function DataTableRowActions<TData>({
  row,
}: DataTableRowActionsProps<TData>) {
  const { t } = useTranslation()
  const redemption = subscriptionRedemptionSchema.parse(row.original)
  const { setOpen, setCurrentRow, triggerRefresh } = useSubscriptionRedemptions()
  const isEnabled = redemption.status === SUBSCRIPTION_REDEMPTION_STATUS.ENABLED
  const isUsed = redemption.status === SUBSCRIPTION_REDEMPTION_STATUS.USED
  const isExpired = isSubscriptionRedemptionExpired(
    redemption.expired_time,
    redemption.status
  )

  const handleToggleStatus = async () => {
    const newStatus = isEnabled
      ? SUBSCRIPTION_REDEMPTION_STATUS.DISABLED
      : SUBSCRIPTION_REDEMPTION_STATUS.ENABLED

    try {
      const result = await updateSubscriptionRedemptionStatus(redemption.id, newStatus)
      if (result.success) {
        const message = isEnabled
          ? t('Subscription redemption code disabled')
          : t('Subscription redemption code enabled')
        toast.success(message)
        triggerRefresh()
      } else {
        toast.error(result.message || t('Failed to update status'))
      }
    } catch (error) {
      toast.error(t('Failed to update status'))
    }
  }

  const canEdit = isEnabled && !isExpired
  const canToggle = !isUsed && !isExpired

  return (
    <DropdownMenu modal={false}>
      <DropdownMenuTrigger asChild>
        <Button
          variant='ghost'
          className='flex h-8 w-8 p-0 data-[state=open]:bg-muted'
        >
          <DotsHorizontalIcon className='h-4 w-4' />
          <span className='sr-only'>{t('Open menu')}</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end' className='w-[160px]'>
        {canEdit && (
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(redemption)
              setOpen('update')
            }}
          >
            <Edit className='mr-2 h-4 w-4' />
            {t('Edit')}
            <DropdownMenuShortcut>⌘+e</DropdownMenuShortcut>
          </DropdownMenuItem>
        )}
        {canToggle && (
          <DropdownMenuItem onClick={handleToggleStatus}>
            {isEnabled ? (
              <>
                <PowerOff className='mr-2 h-4 w-4' />
                {t('Disable')}
              </>
            ) : (
              <>
                <Power className='mr-2 h-4 w-4' />
                {t('Enable')}
              </>
            )}
          </DropdownMenuItem>
        )}
        {canEdit && canToggle && <DropdownMenuSeparator />}
        <DropdownMenuItem
          className='text-destructive'
          onClick={() => {
            setCurrentRow(redemption)
            setOpen('delete')
          }}
        >
          <Trash2 className='mr-2 h-4 w-4' />
          {t('Delete')}
          <DropdownMenuShortcut>⌘+d</DropdownMenuShortcut>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}