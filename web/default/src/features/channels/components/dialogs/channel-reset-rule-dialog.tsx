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
import { useCallback, useEffect, useMemo, useState } from 'react'
import { Loader2, Plus, Pencil, Trash2, ArrowLeft } from 'lucide-react'
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { DateTimePicker } from '@/components/datetime-picker'
import {
  createChannelResetRule,
  deleteChannelResetRule,
  getChannelResetRules,
  updateChannelResetRule,
  type ChannelResetRule,
} from '../../api'
import {
  RULE_TYPES,
  WEEKDAYS,
  buildRuleConfig,
  parseRuleConfig,
  renderRuleConfigSummary,
} from '../../lib/reset-rule-utils'

interface ChannelResetRuleDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  channelId: number | null
}

interface FormValues {
  rule_type: string
  hour: number
  minute: number
  weekday: number
  day_of_month: number
  interval_seconds: number
  specific_time: number | null
  reset_value: number
  enabled: boolean
  remark: string
}

const DEFAULT_FORM_VALUES: FormValues = {
  rule_type: 'daily',
  hour: 3,
  minute: 0,
  weekday: 1,
  day_of_month: 1,
  interval_seconds: 3600,
  specific_time: null,
  reset_value: 0,
  enabled: true,
  remark: '',
}

function formatNextResetTime(ts: number): string {
  if (!ts || ts <= 0) return '-'
  return new Date(ts * 1000).toLocaleString()
}

export function ChannelResetRuleDialog({
  open,
  onOpenChange,
  channelId,
}: ChannelResetRuleDialogProps) {
  const { t } = useTranslation()

  const [loading, setLoading] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [rules, setRules] = useState<ChannelResetRule[]>([])

  // Form state
  const [showForm, setShowForm] = useState(false)
  const [editingRule, setEditingRule] = useState<ChannelResetRule | null>(null)
  const [formValues, setFormValues] = useState<FormValues>(DEFAULT_FORM_VALUES)

  // Delete confirmation
  const [deleteTarget, setDeleteTarget] = useState<ChannelResetRule | null>(null)

  const loadRules = useCallback(async () => {
    if (!channelId) return
    setLoading(true)
    try {
      const res = await getChannelResetRules(channelId)
      if (res.success) {
        setRules(Array.isArray(res.data) ? res.data : [])
      } else {
        toast.error(res.message || t('Failed to load rules'))
      }
    } catch (error: unknown) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to load rules')
      )
    } finally {
      setLoading(false)
    }
  }, [channelId, t])

  useEffect(() => {
    if (open && channelId) {
      loadRules()
    }
    if (!open) {
      setRules([])
      setShowForm(false)
      setEditingRule(null)
      setFormValues(DEFAULT_FORM_VALUES)
      setDeleteTarget(null)
    }
  }, [open, channelId, loadRules])

  const openAddForm = () => {
    setEditingRule(null)
    setFormValues(DEFAULT_FORM_VALUES)
    setShowForm(true)
  }

  const openEditForm = (rule: ChannelResetRule) => {
    setEditingRule(rule)
    const parsed = parseRuleConfig(rule.rule_type, rule.rule_config)
    setFormValues({
      rule_type: rule.rule_type || 'daily',
      hour: Number(parsed.hour ?? 3),
      minute: Number(parsed.minute ?? 0),
      weekday: Number(parsed.weekday ?? 1),
      day_of_month: Number(parsed.day_of_month ?? 1),
      interval_seconds: Number(parsed.interval_seconds ?? 3600),
      specific_time: parsed.specific_time ? Number(parsed.specific_time) : null,
      reset_value: rule.reset_value ?? 0,
      enabled: rule.enabled,
      remark: rule.remark || '',
    })
    setShowForm(true)
  }

  const closeForm = () => {
    setShowForm(false)
    setEditingRule(null)
    setFormValues(DEFAULT_FORM_VALUES)
  }

  const handleSubmit = async () => {
    if (!channelId) return
    const ruleConfig = buildRuleConfig(
      formValues.rule_type,
      formValues as unknown as Record<string, unknown>
    )
    const payload = {
      channel_id: channelId,
      rule_type: formValues.rule_type,
      rule_config: ruleConfig,
      reset_value: formValues.reset_value,
      enabled: editingRule ? editingRule.enabled : formValues.enabled,
      remark: formValues.remark || '',
    }

    if (formValues.rule_type === 'specific_time' && !formValues.specific_time) {
      toast.error(t('Specific Time') + ' ' + t('Required'))
      return
    }

    setSubmitting(true)
    try {
      let res
      if (editingRule) {
        res = await updateChannelResetRule({ id: editingRule.id, ...payload })
      } else {
        res = await createChannelResetRule(payload)
      }
      if (res.success) {
        toast.success(
          editingRule
            ? t('Rule updated successfully')
            : t('Rule created successfully')
        )
        closeForm()
        await loadRules()
      } else {
        toast.error(res.message || t('Failed to save rule'))
      }
    } catch (error: unknown) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to save rule')
      )
    } finally {
      setSubmitting(false)
    }
  }

  const handleDelete = async (ruleId: number) => {
    try {
      const res = await deleteChannelResetRule(ruleId)
      if (res.success) {
        toast.success(t('Rule deleted successfully'))
        await loadRules()
      } else {
        toast.error(res.message || t('Failed to delete rule'))
      }
    } catch (error: unknown) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to delete rule')
      )
    }
  }

  const handleToggleEnabled = async (rule: ChannelResetRule) => {
    if (!channelId) return
    try {
      const res = await updateChannelResetRule({
        id: rule.id,
        channel_id: channelId,
        rule_type: rule.rule_type,
        rule_config: rule.rule_config,
        reset_value: rule.reset_value,
        enabled: !rule.enabled,
        remark: rule.remark || '',
      })
      if (res.success) {
        toast.success(t('Rule updated successfully'))
        await loadRules()
      } else {
        toast.error(res.message || t('Failed to save rule'))
      }
    } catch (error: unknown) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to save rule')
      )
    }
  }

  const ruleTypeLabel = (type: string): string => {
    switch (type) {
      case 'daily':
        return t('Daily')
      case 'weekly':
        return t('Weekly')
      case 'monthly':
        return t('Monthly')
      case 'custom_interval':
        return t('Custom Interval')
      case 'specific_time':
        return t('Specific Time')
      default:
        return type
    }
  }

  const weekdayLabel = (d: number): string => {
    const map = [
      t('Sunday'),
      t('Monday'),
      t('Tuesday'),
      t('Wednesday'),
      t('Thursday'),
      t('Friday'),
      t('Saturday'),
    ]
    return map[d] ?? t('Sunday')
  }

  const ruleTypeOptions = useMemo(
    () =>
      RULE_TYPES.map((type) => ({
        value: type,
        label: ruleTypeLabel(type),
      })),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [t]
  )

  const weekdayOptions = useMemo(
    () =>
      WEEKDAYS.map((d) => ({
        value: String(d),
        label: weekdayLabel(d),
      })),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [t]
  )

  const updateFormValue = <K extends keyof FormValues>(
    key: K,
    value: FormValues[K]
  ) => {
    setFormValues((prev) => ({ ...prev, [key]: value }))
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[90vh] max-w-3xl overflow-y-auto'>
        <DialogHeader>
          <DialogTitle>{t('Reset Rule Management')}</DialogTitle>
          <DialogDescription>
            {channelId
              ? `${t('Channel ID')}: ${channelId}`
              : t('Reset Rule Management')}
          </DialogDescription>
        </DialogHeader>

        {showForm ? (
          // ===== Form View =====
          <div className='space-y-4 py-2'>
            <div className='flex items-center gap-2'>
              <Button
                variant='ghost'
                size='sm'
                onClick={closeForm}
                disabled={submitting}
              >
                <ArrowLeft className='mr-1 size-4' />
                {t('Back')}
              </Button>
              <span className='text-muted-foreground text-sm'>
                {editingRule ? t('Edit Rule') : t('Add Rule')}
              </span>
            </div>

            {/* Rule Type */}
            <div className='space-y-2'>
              <Label>{t('Rule Type')}</Label>
              <Select
                value={formValues.rule_type}
                onValueChange={(v) => v && updateFormValue('rule_type', v)}
                disabled={!!editingRule || submitting}
              >
                <SelectTrigger className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {ruleTypeOptions.map((opt) => (
                    <SelectItem key={opt.value} value={opt.value}>
                      {opt.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {/* Dynamic config fields based on rule type */}
            {formValues.rule_type === 'daily' && (
              <div className='grid grid-cols-2 gap-3'>
                <div className='space-y-2'>
                  <Label>{t('Hour')}</Label>
                  <Select
                    value={String(formValues.hour)}
                    onValueChange={(v) =>
                      v && updateFormValue('hour', Number(v))
                    }
                    disabled={submitting}
                  >
                    <SelectTrigger className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {Array.from({ length: 24 }, (_, i) => i).map((h) => (
                        <SelectItem key={h} value={String(h)}>
                          {String(h).padStart(2, '0')}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className='space-y-2'>
                  <Label>{t('Minute')}</Label>
                  <Select
                    value={String(formValues.minute)}
                    onValueChange={(v) =>
                      v && updateFormValue('minute', Number(v))
                    }
                    disabled={submitting}
                  >
                    <SelectTrigger className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {Array.from({ length: 60 }, (_, i) => i).map((m) => (
                        <SelectItem key={m} value={String(m)}>
                          {String(m).padStart(2, '0')}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>
            )}

            {formValues.rule_type === 'weekly' && (
              <div className='space-y-3'>
                <div className='space-y-2'>
                  <Label>{t('Weekday')}</Label>
                  <Select
                    value={String(formValues.weekday)}
                    onValueChange={(v) =>
                      v && updateFormValue('weekday', Number(v))
                    }
                    disabled={submitting}
                  >
                    <SelectTrigger className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {weekdayOptions.map((opt) => (
                        <SelectItem key={opt.value} value={opt.value}>
                          {opt.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className='grid grid-cols-2 gap-3'>
                  <div className='space-y-2'>
                    <Label>{t('Hour')}</Label>
                    <Select
                      value={String(formValues.hour)}
                      onValueChange={(v) =>
                        v && updateFormValue('hour', Number(v))
                      }
                      disabled={submitting}
                    >
                      <SelectTrigger className='w-full'>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {Array.from({ length: 24 }, (_, i) => i).map((h) => (
                          <SelectItem key={h} value={String(h)}>
                            {String(h).padStart(2, '0')}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className='space-y-2'>
                    <Label>{t('Minute')}</Label>
                    <Select
                      value={String(formValues.minute)}
                      onValueChange={(v) =>
                        v && updateFormValue('minute', Number(v))
                      }
                      disabled={submitting}
                    >
                      <SelectTrigger className='w-full'>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {Array.from({ length: 60 }, (_, i) => i).map((m) => (
                          <SelectItem key={m} value={String(m)}>
                            {String(m).padStart(2, '0')}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                </div>
              </div>
            )}

            {formValues.rule_type === 'monthly' && (
              <div className='grid grid-cols-3 gap-3'>
                <div className='space-y-2'>
                  <Label>{t('Day of Month')}</Label>
                  <Select
                    value={String(formValues.day_of_month)}
                    onValueChange={(v) =>
                      v && updateFormValue('day_of_month', Number(v))
                    }
                    disabled={submitting}
                  >
                    <SelectTrigger className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {Array.from({ length: 31 }, (_, i) => i + 1).map((d) => (
                        <SelectItem key={d} value={String(d)}>
                          {String(d)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className='space-y-2'>
                  <Label>{t('Hour')}</Label>
                  <Select
                    value={String(formValues.hour)}
                    onValueChange={(v) =>
                      v && updateFormValue('hour', Number(v))
                    }
                    disabled={submitting}
                  >
                    <SelectTrigger className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {Array.from({ length: 24 }, (_, i) => i).map((h) => (
                        <SelectItem key={h} value={String(h)}>
                          {String(h).padStart(2, '0')}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className='space-y-2'>
                  <Label>{t('Minute')}</Label>
                  <Select
                    value={String(formValues.minute)}
                    onValueChange={(v) =>
                      v && updateFormValue('minute', Number(v))
                    }
                    disabled={submitting}
                  >
                    <SelectTrigger className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {Array.from({ length: 60 }, (_, i) => i).map((m) => (
                        <SelectItem key={m} value={String(m)}>
                          {String(m).padStart(2, '0')}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>
            )}

            {formValues.rule_type === 'custom_interval' && (
              <div className='space-y-2'>
                <Label>{t('Interval (seconds)')}</Label>
                <Input
                  type='number'
                  min={60}
                  value={formValues.interval_seconds}
                  onChange={(e) =>
                    updateFormValue(
                      'interval_seconds',
                      Number(e.target.value) || 0
                    )
                  }
                  disabled={submitting}
                />
              </div>
            )}

            {formValues.rule_type === 'specific_time' && (
              <div className='space-y-2'>
                <Label>{t('Specific Time')}</Label>
                <DateTimePicker
                  value={
                    formValues.specific_time
                      ? new Date(formValues.specific_time)
                      : undefined
                  }
                  onChange={(date) =>
                    updateFormValue(
                      'specific_time',
                      date ? date.getTime() : null
                    )
                  }
                  className='w-full'
                />
              </div>
            )}

            {/* Reset Value */}
            <div className='space-y-2'>
              <Label>{t('Reset Value')}</Label>
              <Input
                type='number'
                min={-1}
                value={formValues.reset_value}
                onChange={(e) =>
                  updateFormValue('reset_value', Number(e.target.value) || 0)
                }
                disabled={submitting}
                placeholder={t('0 means keep unchanged')}
              />
              <p className='text-muted-foreground text-xs'>
                {t('0 means keep unchanged')}
              </p>
            </div>

            {/* Remark */}
            <div className='space-y-2'>
              <Label>{t('Remark')}</Label>
              <Input
                value={formValues.remark}
                onChange={(e) => updateFormValue('remark', e.target.value)}
                disabled={submitting}
                placeholder={t('Optional')}
              />
            </div>

            {/* Enabled (only for new rule; edit uses toggle in list) */}
            {!editingRule && (
              <div className='flex items-center justify-between'>
                <Label htmlFor='rule-enabled'>{t('Enabled')}</Label>
                <Switch
                  id='rule-enabled'
                  checked={formValues.enabled}
                  onCheckedChange={(checked) =>
                    updateFormValue('enabled', !!checked)
                  }
                  disabled={submitting}
                />
              </div>
            )}

            <DialogFooter>
              <Button
                variant='outline'
                onClick={closeForm}
                disabled={submitting}
              >
                {t('Cancel')}
              </Button>
              <Button onClick={handleSubmit} disabled={submitting}>
                {submitting && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
                {submitting ? t('Saving...') : t('Save')}
              </Button>
            </DialogFooter>
          </div>
        ) : (
          // ===== List View =====
          <>
            <div className='flex items-center justify-between py-2'>
              <span className='text-muted-foreground text-sm'>
                {t('{{count}} rule(s)', { count: rules.length })}
              </span>
              <Button
                size='sm'
                onClick={openAddForm}
                disabled={!channelId || loading}
              >
                <Plus className='mr-1 size-4' />
                {t('Add Rule')}
              </Button>
            </div>

            {loading ? (
              <div className='flex items-center justify-center py-12'>
                <Loader2 className='text-muted-foreground h-8 w-8 animate-spin' />
              </div>
            ) : rules.length === 0 ? (
              <div className='text-muted-foreground flex items-center justify-center py-12 text-sm'>
                {t('No reset rules')}
              </div>
            ) : (
              <div className='overflow-x-auto'>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('Rule Type')}</TableHead>
                      <TableHead>{t('Config')}</TableHead>
                      <TableHead>{t('Next Reset Time')}</TableHead>
                      <TableHead>{t('Reset Value')}</TableHead>
                      <TableHead>{t('Enabled')}</TableHead>
                      <TableHead>{t('Remark')}</TableHead>
                      <TableHead className='text-right'>
                        {t('Actions')}
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {rules.map((rule) => (
                      <TableRow key={rule.id}>
                        <TableCell>{ruleTypeLabel(rule.rule_type)}</TableCell>
                        <TableCell className='text-sm'>
                          {renderRuleConfigSummary(
                            rule.rule_type,
                            rule.rule_config
                          )}
                        </TableCell>
                        <TableCell className='text-sm'>
                          {formatNextResetTime(rule.next_reset_time)}
                        </TableCell>
                        <TableCell className='text-sm'>
                          {rule.reset_value > 0
                            ? rule.reset_value
                            : t('0 means keep unchanged')}
                        </TableCell>
                        <TableCell>
                          <Switch
                            checked={rule.enabled}
                            onCheckedChange={() => handleToggleEnabled(rule)}
                          />
                        </TableCell>
                        <TableCell className='text-sm'>
                          {rule.remark || '-'}
                        </TableCell>
                        <TableCell className='text-right'>
                          <div className='flex justify-end gap-1'>
                            <Button
                              variant='ghost'
                              size='icon-sm'
                              onClick={() => openEditForm(rule)}
                              aria-label={t('Edit Rule')}
                            >
                              <Pencil className='size-4' />
                            </Button>
                            <Button
                              variant='ghost'
                              size='icon-sm'
                              onClick={() => setDeleteTarget(rule)}
                              aria-label={t('Delete Rule')}
                              className='text-destructive hover:text-destructive'
                            >
                              <Trash2 className='size-4' />
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}

            <DialogFooter>
              <Button variant='outline' onClick={() => onOpenChange(false)}>
                {t('Close')}
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(v) => !v && setDeleteTarget(null)}
        title={t('Delete Rule')}
        desc={t('Confirm delete this rule?')}
        confirmText={t('Delete')}
        destructive
        handleConfirm={() => {
          if (deleteTarget) {
            handleDelete(deleteTarget.id)
          }
          setDeleteTarget(null)
        }}
      />
    </Dialog>
  )
}
