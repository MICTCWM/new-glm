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
import { useState } from 'react'
import { Plus, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { toast } from 'sonner'
import { useRedemptions } from './redemptions-provider'
import { deleteInvalidRedemptions, deleteUnusedRedemptions } from '../api'

export function RedemptionsPrimaryButtons() {
  const { t } = useTranslation()
  const { setOpen, triggerRefresh } = useRedemptions()
  const [showDeleteInvalidConfirm, setShowDeleteInvalidConfirm] = useState(false)
  const [showDeleteUnusedConfirm, setShowDeleteUnusedConfirm] = useState(false)
  const [isDeleting, setIsDeleting] = useState(false)

  const handleDeleteInvalid = async () => {
    setIsDeleting(true)
    try {
      const result = await deleteInvalidRedemptions()
      if (result.success) {
        const count = result.data || 0
        toast.success(
          t('Successfully deleted {{count}} invalid redemption codes', {
            count,
          })
        )
        triggerRefresh()
        setShowDeleteInvalidConfirm(false)
      } else {
        toast.error(result.message || t('Failed to delete invalid redemption codes'))
      }
    } catch {
      toast.error(t('An unexpected error occurred'))
    } finally {
      setIsDeleting(false)
    }
  }

  const handleDeleteUnused = async () => {
    setIsDeleting(true)
    try {
      const result = await deleteUnusedRedemptions()
      if (result.success) {
        const count = result.data || 0
        toast.success(
          t('Successfully deleted {{count}} unused redemption codes', {
            count,
          })
        )
        triggerRefresh()
        setShowDeleteUnusedConfirm(false)
      } else {
        toast.error(result.message || t('Failed to delete redemption code'))
      }
    } catch {
      toast.error(t('An unexpected error occurred'))
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
        handleConfirm={handleDeleteUnused}
        isLoading={isDeleting}
        className='max-w-md'
        title={t('Delete Unused Redemption Codes?')}
        desc={
          <>
            {t('This will delete all')} <strong>{t('unused')}</strong>{' '}
            {t('redemption codes.')}
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
        title={t('Delete Invalid Redemption Codes?')}
        desc={
          <>
            {t('This will delete all')} <strong>{t('used')}</strong>,{' '}
            <strong>{t('disabled')}</strong>
            {t(', and')} <strong>{t('expired')}</strong>{' '}
            {t('redemption codes.')}
            <br />
            {t('This action cannot be undone.')}
          </>
        }
        confirmText={t('Delete Invalid')}
      />
    </>
  )
}
