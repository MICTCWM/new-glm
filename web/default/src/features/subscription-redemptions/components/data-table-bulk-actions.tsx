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
import { useState, useMemo } from 'react'
import { type Table } from '@tanstack/react-table'
import { Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { CopyButton } from '@/components/copy-button'
import { DataTableBulkActions as BulkActionsToolbar } from '@/components/data-table'
import {
  deleteInvalidSubscriptionRedemptions,
  deleteUnusedSubscriptionRedemptions,
} from '../api'
import { type SubscriptionRedemption } from '../types'
import { useSubscriptionRedemptions } from './subscription-redemptions-provider'

type DataTableBulkActionsProps<TData> = {
  table: Table<TData>
}

export function DataTableBulkActions<TData>({
  table,
}: DataTableBulkActionsProps<TData>) {
  const { t } = useTranslation()
  const { triggerRefresh } = useSubscriptionRedemptions()
  const [showDeleteInvalidConfirm, setShowDeleteInvalidConfirm] = useState(false)
  const [showDeleteUnusedConfirm, setShowDeleteUnusedConfirm] = useState(false)
  const [isDeleting, setIsDeleting] = useState(false)
  const selectedRows = table.getFilteredSelectedRowModel().rows

  const contentToCopy = useMemo(() => {
    const selectedCodes = selectedRows.map((row) => {
      const redemption = row.original as SubscriptionRedemption
      return `${redemption.name}\t${redemption.key}`
    })
    return selectedCodes.join('\n')
  }, [selectedRows])

  const handleDeleteInvalid = async () => {
    setIsDeleting(true)
    try {
      const result = await deleteInvalidSubscriptionRedemptions()

      if (result.success) {
        const count = result.data || 0
        toast.success(
          t('Successfully deleted {{count}} invalid subscription redemption codes', {
            count,
          })
        )
        table.resetRowSelection()
        triggerRefresh()
        setShowDeleteInvalidConfirm(false)
      } else {
        toast.error(result.message || t('Failed to delete invalid codes'))
      }
    } catch (error) {
      toast.error(t('An error occurred'))
    } finally {
      setIsDeleting(false)
    }
  }

  const handleDeleteUnused = async () => {
    setIsDeleting(true)
    try {
      const result = await deleteUnusedSubscriptionRedemptions()

      if (result.success) {
        const count = result.data || 0
        toast.success(
          t('Successfully deleted {{count}} unused subscription redemption codes', {
            count,
          })
        )
        table.resetRowSelection()
        triggerRefresh()
        setShowDeleteUnusedConfirm(false)
      } else {
        toast.error(result.message || t('Failed to delete unused codes'))
      }
    } catch (error) {
      toast.error(t('An error occurred'))
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <>
      <BulkActionsToolbar table={table}>
        <Tooltip>
          <TooltipTrigger asChild>
            <CopyButton
              value={contentToCopy}
              className='h-8 w-8'
              successMessage={t('Selected codes copied')}
            />
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('Copy selected codes')}</p>
          </TooltipContent>
        </Tooltip>
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant='outline'
              size='sm'
              className='h-8'
              onClick={() => setShowDeleteUnusedConfirm(true)}
            >
              <Trash2 className='h-4 w-4' />
              {t('Delete Unused')}
            </Button>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('Delete unused codes')}</p>
          </TooltipContent>
        </Tooltip>
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant='destructive'
              size='sm'
              className='h-8'
              onClick={() => setShowDeleteInvalidConfirm(true)}
            >
              <Trash2 className='h-4 w-4' />
              {t('Delete Invalid')}
            </Button>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('Delete invalid codes')}</p>
          </TooltipContent>
        </Tooltip>
      </BulkActionsToolbar>

      <ConfirmDialog
        destructive
        open={showDeleteUnusedConfirm}
        onOpenChange={setShowDeleteUnusedConfirm}
        handleConfirm={handleDeleteUnused}
        isLoading={isDeleting}
        className='max-w-md'
        title={t('Delete Unused Subscription Redemption Codes?')}
        desc={
          <>
            {t('This will delete all')} <strong>{t('unused')}</strong>{' '}
            {t('subscription redemption codes.')}
            <br />
            {t('This action cannot be undone.')}
          </>
        }
        confirmText={t('Delete Unused')}
      />

      <ConfirmDialog
        destructive
        open={showDeleteInvalidConfirm}
        onOpenChange={setShowDeleteInvalidConfirm}
        handleConfirm={handleDeleteInvalid}
        isLoading={isDeleting}
        className='max-w-md'
        title={t('Delete Invalid Subscription Redemption Codes?')}
        desc={
          <>
            {t('This will delete all')} <strong>{t('used')}</strong>,{' '}
            <strong>{t('disabled')}</strong>
            {t(', and')} <strong>{t('expired')}</strong>{' '}
            {t('subscription redemption codes.')}
            <br />
            {t('This action cannot be undone.')}
          </>
        }
        confirmText={t('Delete Invalid')}
      />
    </>
  )
}