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
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { postponeUserSubscriptions } from '../api'
import { useUsers } from './users-provider'

export function UsersPostponeSubscriptionDialog() {
  const { t } = useTranslation()
  const { open, setOpen, selectedUsers, triggerRefresh, setRowSelection } =
    useUsers()
  const [days, setDays] = useState('')
  const [loading, setLoading] = useState(false)

  const selectedCount = selectedUsers.length
  const daysValue = parseInt(days, 10)
  const isValid = daysValue > 0 && selectedCount > 0

  const handleConfirm = async () => {
    if (!isValid) return
    setLoading(true)
    try {
      const userIds = selectedUsers.map((u) => u.id)
      const result = await postponeUserSubscriptions(userIds, daysValue)
      if (result.success) {
        const results = result.data?.results ?? {}
        const affectedCount = Object.values(results).filter(
          (v) => v > 0
        ).length
        if (affectedCount === 0) {
          toast.error(t('No users selected or no subscriptions found'))
        } else {
          toast.success(t('Postpone subscriptions success'))
        }
        setDays('')
        setOpen(null)
        setRowSelection({})
        triggerRefresh()
      } else {
        toast.error(
          result.message || t('Failed to postpone subscriptions')
        )
      }
    } catch (e: unknown) {
      toast.error(
        e instanceof Error
          ? e.message
          : t('Failed to postpone subscriptions')
      )
    } finally {
      setLoading(false)
    }
  }

  const handleCancel = () => {
    setDays('')
    setOpen(null)
  }

  return (
    <Dialog
      open={open === 'postpone-subscription'}
      onOpenChange={(isOpen) => !isOpen && setOpen(null)}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('Postpone Subscriptions')}</DialogTitle>
          <DialogDescription>
            {t('Select users with subscriptions to postpone')}
          </DialogDescription>
        </DialogHeader>
        <div className='space-y-4'>
          <div className='text-muted-foreground text-sm'>
            {t('Will postpone subscriptions for {{count}} users', {
              count: selectedCount,
            })}
          </div>
          <div className='space-y-2'>
            <Label>{t('Postpone Days')}</Label>
            <Input
              type='number'
              min={1}
              step={1}
              placeholder={t('Enter days to postpone')}
              value={days}
              onChange={(e) => setDays(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !loading) handleConfirm()
              }}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant='outline' onClick={handleCancel} disabled={loading}>
            {t('Cancel')}
          </Button>
          <Button onClick={handleConfirm} disabled={loading || !isValid}>
            {loading ? t('Processing...') : t('Postpone')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
