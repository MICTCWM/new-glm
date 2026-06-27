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
import { useQueryClient } from '@tanstack/react-query'
import { type Table } from '@tanstack/react-table'
import { Power, PowerOff, RefreshCcw, RotateCcw, Tag, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
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
import { Label } from '@/components/ui/label'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { DataTableBulkActions as BulkActionsToolbar } from '@/components/data-table'
import {
  handleBatchDelete,
  handleBatchDisable,
  handleBatchEnable,
  handleBatchResetQuota,
  handleBatchSetQuotaConfig,
  handleBatchSetTag,
} from '../lib'
import type { Channel } from '../types'

interface DataTableBulkActionsProps<TData> {
  table: Table<TData>
}

const RESET_HOUR_OPTIONS = Array.from({ length: 24 }, (_, hour) => hour)

export function DataTableBulkActions<TData>({
  table,
}: DataTableBulkActionsProps<TData>) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [showTagDialog, setShowTagDialog] = useState(false)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const [showQuotaDialog, setShowQuotaDialog] = useState(false)
  const [showResetConfirm, setShowResetConfirm] = useState(false)
  const [tagValue, setTagValue] = useState('')
  const [maxCallCount, setMaxCallCount] = useState('0')
  const [resetMinute, setResetMinute] = useState('0')
  const [resetHours, setResetHours] = useState<number[]>([])

  const selectedRows = table.getFilteredSelectedRowModel().rows
  const selectedIds = selectedRows.reduce<number[]>((ids, row) => {
    const id = (row.original as Channel).id

    if (typeof id === 'number') {
      ids.push(id)
    }

    return ids
  }, [])

  const handleClearSelection = () => {
    table.resetRowSelection()
  }

  const handleEnableAll = () => {
    handleBatchEnable(selectedIds, queryClient, handleClearSelection)
  }

  const handleDisableAll = () => {
    handleBatchDisable(selectedIds, queryClient, handleClearSelection)
  }

  const handleDeleteAll = () => {
    handleBatchDelete(selectedIds, queryClient, () => {
      setShowDeleteConfirm(false)
      handleClearSelection()
    })
  }

  const handleSetTag = () => {
    handleBatchSetTag(selectedIds, tagValue || null, queryClient, () => {
      setShowTagDialog(false)
      setTagValue('')
      handleClearSelection()
    })
  }

  const handleSetQuota = () => {
    handleBatchSetQuotaConfig(
      selectedIds,
      {
        max_call_count: Math.max(0, Number(maxCallCount) || 0),
        reset_hours: [...resetHours].sort((a, b) => a - b),
        reset_minute: Math.min(59, Math.max(0, Number(resetMinute) || 0)),
      },
      queryClient,
      () => {
        setShowQuotaDialog(false)
        setMaxCallCount('0')
        setResetMinute('0')
        setResetHours([])
        handleClearSelection()
      }
    )
  }

  const handleResetQuotaConfirm = () => {
    handleBatchResetQuota(selectedIds, queryClient, () => {
      setShowResetConfirm(false)
      handleClearSelection()
    })
  }

  return (
    <>
      <BulkActionsToolbar table={table} entityName='channel'>
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='outline'
                size='icon'
                onClick={handleEnableAll}
                className='size-8'
                aria-label={t('Enable selected channels')}
                title={t('Enable selected channels')}
              />
            }
          >
            <Power />
            <span className='sr-only'>{t('Enable selected channels')}</span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('Enable selected channels')}</p>
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='outline'
                size='icon'
                onClick={handleDisableAll}
                className='size-8'
                aria-label={t('Disable selected channels')}
                title={t('Disable selected channels')}
              />
            }
          >
            <PowerOff />
            <span className='sr-only'>{t('Disable selected channels')}</span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('Disable selected channels')}</p>
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='outline'
                size='icon'
                onClick={() => setShowQuotaDialog(true)}
                className='size-8'
                aria-label={t('Set quota for selected channels')}
                title={t('Set quota for selected channels')}
              />
            }
          >
            <RotateCcw />
            <span className='sr-only'>
              {t('Set quota for selected channels')}
            </span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('Set quota for selected channels')}</p>
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='outline'
                size='icon'
                onClick={() => setShowResetConfirm(true)}
                className='size-8'
                aria-label={t('Reset quota for selected channels')}
                title={t('Reset quota for selected channels')}
              />
            }
          >
            <RefreshCcw />
            <span className='sr-only'>
              {t('Reset quota for selected channels')}
            </span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('Reset quota for selected channels')}</p>
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='outline'
                size='icon'
                onClick={() => setShowTagDialog(true)}
                className='size-8'
                aria-label={t('Set tag for selected channels')}
                title={t('Set tag for selected channels')}
              />
            }
          >
            <Tag />
            <span className='sr-only'>
              {t('Set tag for selected channels')}
            </span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('Set tag for selected channels')}</p>
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='destructive'
                size='icon'
                onClick={() => setShowDeleteConfirm(true)}
                className='size-8'
                aria-label={t('Delete selected channels')}
                title={t('Delete selected channels')}
              />
            }
          >
            <Trash2 />
            <span className='sr-only'>{t('Delete selected channels')}</span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('Delete selected channels')}</p>
          </TooltipContent>
        </Tooltip>
      </BulkActionsToolbar>

      {/* Set Tag Dialog */}
      <Dialog open={showTagDialog} onOpenChange={setShowTagDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Set Tag')}</DialogTitle>
            <DialogDescription>
              {t('Set a tag for')} {selectedIds.length}{' '}
              {t('selected channel(s). Leave empty to remove tag.')}
            </DialogDescription>
          </DialogHeader>

          <div className='grid gap-4 py-4'>
            <div className='grid gap-2'>
              <Label htmlFor='tag'>{t('Tag')}</Label>
              <Input
                id='tag'
                placeholder={t('Enter tag name (optional)')}
                value={tagValue}
                onChange={(e) => setTagValue(e.target.value)}
              />
            </div>
          </div>

          <DialogFooter>
            <Button
              variant='outline'
              onClick={() => {
                setShowTagDialog(false)
                setTagValue('')
              }}
            >
              {t('Cancel')}
            </Button>
            <Button onClick={handleSetTag}>{t('Set Tag')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={showQuotaDialog} onOpenChange={setShowQuotaDialog}>
        <DialogContent className='sm:max-w-2xl'>
          <DialogHeader>
            <DialogTitle>{t('Batch Quota Settings')}</DialogTitle>
            <DialogDescription>
              {t('Set quota rules for')} {selectedIds.length}{' '}
              {t('selected channel(s).')}
            </DialogDescription>
          </DialogHeader>

          <div className='grid gap-4 py-4'>
            <div className='grid gap-2'>
              <Label htmlFor='batch-max-call-count'>
                {t('Max successful requests')}
              </Label>
              <Input
                id='batch-max-call-count'
                type='number'
                min={0}
                value={maxCallCount}
                onChange={(e) => setMaxCallCount(e.target.value)}
                placeholder='0'
              />
              <p className='text-muted-foreground text-sm'>
                {t(
                  'The channel will be auto-disabled after successful requests reach this value. 0 means unlimited.'
                )}
              </p>
            </div>

            <div className='grid gap-2'>
              <Label htmlFor='batch-reset-minute'>{t('Reset minute')}</Label>
              <Input
                id='batch-reset-minute'
                type='number'
                min={0}
                max={59}
                value={resetMinute}
                onChange={(e) => setResetMinute(e.target.value)}
                placeholder='0'
              />
            </div>

            <div className='grid gap-2'>
              <Label>{t('Reset time (hourly)')}</Label>
              <p className='text-muted-foreground text-sm'>
                {t(
                  'Select the daily hours to clear used request quota, using the server timezone.'
                )}
              </p>
              <div className='grid grid-cols-4 gap-2 sm:grid-cols-6'>
                {RESET_HOUR_OPTIONS.map((hour) => {
                  const checked = resetHours.includes(hour)
                  return (
                    <label
                      key={hour}
                      className='border-input hover:bg-muted/50 flex cursor-pointer items-center gap-2 rounded-md border px-2 py-1.5 text-xs'
                    >
                      <Checkbox
                        checked={checked}
                        onCheckedChange={(value) => {
                          setResetHours((current) =>
                            value
                              ? Array.from(new Set([...current, hour]))
                              : current.filter((item) => item !== hour)
                          )
                        }}
                      />
                      <span>{String(hour).padStart(2, '0')}:00</span>
                    </label>
                  )
                })}
              </div>
            </div>
          </div>

          <DialogFooter>
            <Button
              variant='outline'
              onClick={() => {
                setShowQuotaDialog(false)
                setMaxCallCount('0')
                setResetMinute('0')
                setResetHours([])
              }}
            >
              {t('Cancel')}
            </Button>
            <Button onClick={handleSetQuota}>{t('Save changes')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Reset Quota Confirmation Dialog */}
      <Dialog open={showResetConfirm} onOpenChange={setShowResetConfirm}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Reset Channel Quota')}</DialogTitle>
            <DialogDescription>
              {t('Confirm reset used quota for')} {selectedIds.length}{' '}
              {t(
                'selected channel(s)? Total quota remains unchanged, used count will be cleared.'
              )}
            </DialogDescription>
          </DialogHeader>

          <DialogFooter>
            <Button
              variant='outline'
              onClick={() => setShowResetConfirm(false)}
            >
              {t('Cancel')}
            </Button>
            <Button onClick={handleResetQuotaConfirm}>{t('Reset')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <Dialog open={showDeleteConfirm} onOpenChange={setShowDeleteConfirm}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Delete Channels?')}</DialogTitle>
            <DialogDescription>
              {t('Are you sure you want to delete')} {selectedIds.length}{' '}
              {t('channel(s)? This action cannot be undone.')}
            </DialogDescription>
          </DialogHeader>

          <DialogFooter>
            <Button
              variant='outline'
              onClick={() => setShowDeleteConfirm(false)}
            >
              {t('Cancel')}
            </Button>
            <Button variant='destructive' onClick={handleDeleteAll}>
              {t('Delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
