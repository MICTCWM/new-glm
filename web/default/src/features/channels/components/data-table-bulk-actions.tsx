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
import {
  CalendarClock,
  Power,
  PowerOff,
  RefreshCcw,
  RotateCcw,
  Tag,
  Trash2,
} from 'lucide-react'
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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
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
  handleBatchSetResetRule,
  handleBatchSetTag,
} from '../lib'
import { buildRuleConfig } from '../lib/reset-rule-utils'
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
  const [showResetRuleDialog, setShowResetRuleDialog] = useState(false)
  const [tagValue, setTagValue] = useState('')
  const [maxCallCount, setMaxCallCount] = useState('0')
  const [resetMinute, setResetMinute] = useState('0')
  const [resetHours, setResetHours] = useState<number[]>([])
  const [ruleType, setRuleType] = useState<string>('daily')
  const [ruleHour, setRuleHour] = useState('0')
  const [ruleMinute, setRuleMinute] = useState('0')
  const [ruleWeekday, setRuleWeekday] = useState('0')
  const [ruleDayOfMonth, setRuleDayOfMonth] = useState('1')
  const [ruleIntervalSeconds, setRuleIntervalSeconds] = useState('3600')
  const [ruleSpecificTime, setRuleSpecificTime] = useState('')
  const [resetValue, setResetValue] = useState('0')
  const [ruleRemark, setRuleRemark] = useState('')

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

  const handleSetResetRule = () => {
    const values: Record<string, unknown> = {
      hour: Number(ruleHour),
      minute: Number(ruleMinute),
      weekday: Number(ruleWeekday),
      day_of_month: Number(ruleDayOfMonth),
      interval_seconds: Number(ruleIntervalSeconds),
      specific_time: ruleSpecificTime
        ? new Date(ruleSpecificTime).getTime()
        : 0,
    }
    const ruleConfig = buildRuleConfig(ruleType, values)
    handleBatchSetResetRule(
      selectedIds,
      {
        rule_type: ruleType,
        rule_config: ruleConfig,
        reset_value: Math.max(0, Number(resetValue) || 0),
        enabled: true,
        remark: ruleRemark || undefined,
      },
      queryClient,
      () => {
        setShowResetRuleDialog(false)
        setRuleType('daily')
        setRuleHour('0')
        setRuleMinute('0')
        setRuleWeekday('0')
        setRuleDayOfMonth('1')
        setRuleIntervalSeconds('3600')
        setRuleSpecificTime('')
        setResetValue('0')
        setRuleRemark('')
        handleClearSelection()
      }
    )
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
                onClick={() => setShowResetRuleDialog(true)}
                className='size-8'
                aria-label={t('Set reset rule for selected channels')}
                title={t('Set reset rule for selected channels')}
              />
            }
          >
            <CalendarClock />
            <span className='sr-only'>
              {t('Set reset rule for selected channels')}
            </span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('Set reset rule for selected channels')}</p>
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

      {/* Batch Set Reset Rule Dialog */}
      <Dialog open={showResetRuleDialog} onOpenChange={setShowResetRuleDialog}>
        <DialogContent className='sm:max-w-2xl'>
          <DialogHeader>
            <DialogTitle>{t('Batch Set Reset Rule')}</DialogTitle>
            <DialogDescription>
              {t('Set reset rule for')} {selectedIds.length}{' '}
              {t('selected channel(s).')}
            </DialogDescription>
          </DialogHeader>

          <div className='grid gap-4 py-4'>
            <div className='grid gap-2'>
              <Label htmlFor='batch-rule-type'>{t('Rule Type')}</Label>
              <Select
                value={ruleType}
                onValueChange={(v) => v !== null && setRuleType(v)}
              >
                <SelectTrigger id='batch-rule-type'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='daily'>{t('Daily')}</SelectItem>
                  <SelectItem value='weekly'>{t('Weekly')}</SelectItem>
                  <SelectItem value='monthly'>{t('Monthly')}</SelectItem>
                  <SelectItem value='custom_interval'>
                    {t('Custom Interval')}
                  </SelectItem>
                  <SelectItem value='specific_time'>
                    {t('Specific Time')}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>

            {ruleType === 'daily' && (
              <div className='grid grid-cols-2 gap-4'>
                <div className='grid gap-2'>
                  <Label htmlFor='batch-rule-hour'>{t('Hour')}</Label>
                  <Input
                    id='batch-rule-hour'
                    type='number'
                    min={0}
                    max={23}
                    value={ruleHour}
                    onChange={(e) => setRuleHour(e.target.value)}
                  />
                </div>
                <div className='grid gap-2'>
                  <Label htmlFor='batch-rule-minute'>{t('Minute')}</Label>
                  <Input
                    id='batch-rule-minute'
                    type='number'
                    min={0}
                    max={59}
                    value={ruleMinute}
                    onChange={(e) => setRuleMinute(e.target.value)}
                  />
                </div>
              </div>
            )}

            {ruleType === 'weekly' && (
              <div className='grid grid-cols-3 gap-4'>
                <div className='grid gap-2'>
                  <Label htmlFor='batch-rule-weekday'>{t('Weekday')}</Label>
                  <Select
                    value={ruleWeekday}
                    onValueChange={(v) => v !== null && setRuleWeekday(v)}
                  >
                    <SelectTrigger id='batch-rule-weekday'>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {[0, 1, 2, 3, 4, 5, 6].map((d) => (
                        <SelectItem key={d} value={String(d)}>
                          {t('weekday_' + d)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className='grid gap-2'>
                  <Label htmlFor='batch-rule-hour-w'>{t('Hour')}</Label>
                  <Input
                    id='batch-rule-hour-w'
                    type='number'
                    min={0}
                    max={23}
                    value={ruleHour}
                    onChange={(e) => setRuleHour(e.target.value)}
                  />
                </div>
                <div className='grid gap-2'>
                  <Label htmlFor='batch-rule-minute-w'>{t('Minute')}</Label>
                  <Input
                    id='batch-rule-minute-w'
                    type='number'
                    min={0}
                    max={59}
                    value={ruleMinute}
                    onChange={(e) => setRuleMinute(e.target.value)}
                  />
                </div>
              </div>
            )}

            {ruleType === 'monthly' && (
              <div className='grid grid-cols-3 gap-4'>
                <div className='grid gap-2'>
                  <Label htmlFor='batch-rule-dom'>{t('Day of Month')}</Label>
                  <Input
                    id='batch-rule-dom'
                    type='number'
                    min={1}
                    max={31}
                    value={ruleDayOfMonth}
                    onChange={(e) => setRuleDayOfMonth(e.target.value)}
                  />
                </div>
                <div className='grid gap-2'>
                  <Label htmlFor='batch-rule-hour-m'>{t('Hour')}</Label>
                  <Input
                    id='batch-rule-hour-m'
                    type='number'
                    min={0}
                    max={23}
                    value={ruleHour}
                    onChange={(e) => setRuleHour(e.target.value)}
                  />
                </div>
                <div className='grid gap-2'>
                  <Label htmlFor='batch-rule-minute-m'>{t('Minute')}</Label>
                  <Input
                    id='batch-rule-minute-m'
                    type='number'
                    min={0}
                    max={59}
                    value={ruleMinute}
                    onChange={(e) => setRuleMinute(e.target.value)}
                  />
                </div>
              </div>
            )}

            {ruleType === 'custom_interval' && (
              <div className='grid gap-2'>
                <Label htmlFor='batch-rule-interval'>
                  {t('Interval (seconds)')}
                </Label>
                <Input
                  id='batch-rule-interval'
                  type='number'
                  min={60}
                  value={ruleIntervalSeconds}
                  onChange={(e) => setRuleIntervalSeconds(e.target.value)}
                />
              </div>
            )}

            {ruleType === 'specific_time' && (
              <div className='grid gap-2'>
                <Label htmlFor='batch-rule-specific'>{t('Specific Time')}</Label>
                <Input
                  id='batch-rule-specific'
                  type='datetime-local'
                  value={ruleSpecificTime}
                  onChange={(e) => setRuleSpecificTime(e.target.value)}
                />
              </div>
            )}

            <div className='grid gap-2'>
              <Label htmlFor='batch-reset-value'>{t('Reset Value')}</Label>
              <Input
                id='batch-reset-value'
                type='number'
                min={0}
                value={resetValue}
                onChange={(e) => setResetValue(e.target.value)}
                placeholder='0'
              />
              <p className='text-muted-foreground text-sm'>
                {t('0 means keep unchanged')}
              </p>
            </div>

            <div className='grid gap-2'>
              <Label htmlFor='batch-rule-remark'>{t('Remark')}</Label>
              <Input
                id='batch-rule-remark'
                value={ruleRemark}
                onChange={(e) => setRuleRemark(e.target.value)}
                placeholder={t('Optional')}
              />
            </div>
          </div>

          <DialogFooter>
            <Button
              variant='outline'
              onClick={() => setShowResetRuleDialog(false)}
            >
              {t('Cancel')}
            </Button>
            <Button onClick={handleSetResetRule}>{t('Set Reset Rule')}</Button>
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
