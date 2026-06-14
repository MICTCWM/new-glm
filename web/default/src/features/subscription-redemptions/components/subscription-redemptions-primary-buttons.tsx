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
import { useState } from 'react'
import { Plus, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { toast } from 'sonner'
import { useSubscriptionRedemptions } from './subscription-redemptions-provider'
import {
  deleteInvalidSubscriptionRedemptions,
  deleteUnusedSubscriptionRedemptions,
} from '../api'

export function SubscriptionRedemptionsPrimaryButtons() {
  const { t } = useTranslation()
  const { setOpen, triggerRefresh } = useSubscriptionRedemptions()
  const [showDeleteInvalidConfirm, setShowDeleteInvalidConfirm] = useState(false)
  const [showDeleteUnusedConfirm, setShowDeleteUnusedConfirm] = useState(false)
  const [isDeleting, setIsDeleting] = useState(false)

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
      <div className='flex gap-2'>
        <Button size='sm' onClick={() => setOpen('create')}>
          <Plus className='h-4 w-4' />
          {t('Create Code')}
        </Button>
        <Button
          size='sm'
          variant='outline'
          onClick={() => setShowDeleteUnusedConfirm(true)}
        >
          <Trash2 className='h-4 w-4' />
          {t('Delete Unused')}
        </Button>
        <Button
          size='sm'
          variant='destructive'
          onClick={() => setShowDeleteInvalidConfirm(true)}
        >
          <Trash2 className='h-4 w-4' />
          {t('Delete Invalid')}
        </Button>
      </div>

      <ConfirmDialog
        destructive
        open={showDeleteUnusedConfirm}
        onOpenChange={setShowDeleteUnusedConfirm}
        title={t('Delete Unused Subscription Redemption Codes')}
        description={t(
          'This will delete all unused (enabled) subscription redemption codes. This action cannot be undone.'
        )}
        onConfirm={handleDeleteUnused}
        loading={isDeleting}
      />

      <ConfirmDialog
        destructive
        open={showDeleteInvalidConfirm}
        onOpenChange={setShowDeleteInvalidConfirm}
        title={t('Delete Invalid Subscription Redemption Codes')}
        description={t(
          'This will delete all used, disabled, and expired subscription redemption codes. This action cannot be undone.'
        )}
        onConfirm={handleDeleteInvalid}
        loading={isDeleting}
      />
    </>
  )
}