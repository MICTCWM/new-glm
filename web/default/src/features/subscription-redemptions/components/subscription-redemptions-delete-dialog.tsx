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
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { deleteSubscriptionRedemption } from '../api'
import { SUCCESS_MESSAGES } from '../constants'
import { useSubscriptionRedemptions } from './subscription-redemptions-provider'

interface SubscriptionRedemptionsDeleteDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow: {
    id: number
    name: string
    key: string
  } | null
}

export function SubscriptionRedemptionsDeleteDialog({
  open,
  onOpenChange,
  currentRow,
}: SubscriptionRedemptionsDeleteDialogProps) {
  const { t } = useTranslation()
  const { triggerRefresh } = useSubscriptionRedemptions()

  const handleDelete = async () => {
    if (!currentRow) return

    try {
      const result = await deleteSubscriptionRedemption(currentRow.id)
      if (result.success) {
        toast.success(t(SUCCESS_MESSAGES.SUBSCRIPTION_REDEMPTION_DELETED))
        onOpenChange(false)
        triggerRefresh()
      } else {
        toast.error(result.message || t('Failed to delete subscription redemption code'))
      }
    } catch (error) {
      toast.error(t('An error occurred'))
    }
  }

  return (
    <ConfirmDialog
      destructive
      open={open}
      onOpenChange={onOpenChange}
      title={t('Delete Subscription Redemption Code')}
      description={
        currentRow
          ? t(
              'Are you sure you want to delete subscription redemption code "{{name}}"? This action cannot be undone.',
              { name: currentRow.name }
            )
          : ''
      }
      onConfirm={handleDelete}
      confirmText={t('Delete')}
    />
  )
}