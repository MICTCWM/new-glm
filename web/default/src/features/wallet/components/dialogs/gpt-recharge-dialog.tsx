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
import { useEffect, useState } from 'react'
import { Loader2, Lock } from 'lucide-react'
import i18next from 'i18next'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from '@/components/ui/tabs'
import { transferGptQuota } from '../../api'
import {
  BASE_TO_GPT_RATIO,
  GPT_TO_BASE_RATIO,
} from '../../constants'
import { formatQuota, parseQuotaFromDollars, quotaUnitsToDollars } from '@/lib/format'

interface GptRechargeDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSuccess: () => void | Promise<void>
  availableBaseQuota: number
}

type RechargeMode = 'gpt' | 'base'

function formatGptQuota(value: number): string {
  return value.toLocaleString(undefined, { maximumFractionDigits: 4 })
}

export function GptRechargeDialog({
  open,
  onOpenChange,
  onSuccess,
  availableBaseQuota,
}: GptRechargeDialogProps) {
  const { t } = useTranslation()
  const [mode, setMode] = useState<RechargeMode>('gpt')
  const [gptInput, setGptInput] = useState('')
  const [baseInput, setBaseInput] = useState('')
  const [submitting, setSubmitting] = useState(false)

  // Reset state when dialog opens
  useEffect(() => {
    if (open) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setMode('gpt')
      setGptInput('')
      setBaseInput('')
    }
  }, [open])

  // 用户输入的是美金，需要转成内部额度
  const gptAmount = Number(gptInput) || 0
  const baseAmountUsd = Number(baseInput) || 0 // 用户输入的美金金额
  const baseAmountInput = parseQuotaFromDollars(baseAmountUsd) // 转成内部额度

  // 计算转换后的内部额度和 GPT 额度
  const effectiveBaseQuota =
    mode === 'gpt' ? gptAmount * GPT_TO_BASE_RATIO : baseAmountInput
  const effectiveGptQuota =
    mode === 'gpt' ? gptAmount : baseAmountInput * BASE_TO_GPT_RATIO

  const insufficientBalance = effectiveBaseQuota > availableBaseQuota
  const invalidAmount = effectiveBaseQuota <= 0
  const canSubmit =
    !invalidAmount && !insufficientBalance && !submitting && Number.isFinite(effectiveBaseQuota)

  const handleSubmit = async () => {
    if (!canSubmit) return
    try {
      setSubmitting(true)
      const response = await transferGptQuota(Math.round(effectiveBaseQuota))
      if (response.success) {
        toast.success(response.message || i18next.t('Recharge successful'))
        onOpenChange(false)
        await onSuccess()
      } else {
        toast.error(response.message || i18next.t('Recharge failed'))
      }
    } catch (_error) {
      toast.error(i18next.t('Recharge failed'))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-md'>
        <DialogHeader>
          <DialogTitle className='text-xl font-semibold'>
            {t('Recharge GPT Quota')}
          </DialogTitle>
          <DialogDescription>
            {t('Convert base balance to GPT-exclusive balance')}
          </DialogDescription>
        </DialogHeader>

        <Tabs defaultValue='quota' className='w-full'>
          <TabsList className='grid w-full grid-cols-2'>
            <TabsTrigger value='subscription'>
              <Lock className='size-3.5' />
              {t('Subscription Recharge')}
            </TabsTrigger>
            <TabsTrigger value='quota'>{t('Quota Recharge')}</TabsTrigger>
          </TabsList>

          <TabsContent value='subscription' className='mt-4'>
            <div className='text-muted-foreground rounded-lg border border-dashed p-6 text-center text-sm'>
              {t('Coming soon')}
            </div>
          </TabsContent>

          <TabsContent value='quota' className='mt-4'>
            <div className='space-y-4'>
              <div className='bg-muted/40 flex items-center justify-between rounded-lg px-3 py-2'>
                <span className='text-muted-foreground text-xs font-medium tracking-wider uppercase'>
                  {t('Available Base Balance')}
                </span>
                <span className='font-mono text-sm font-semibold tabular-nums'>
                  {formatQuota(availableBaseQuota)}
                </span>
              </div>

              <RadioGroup
                value={mode}
                onValueChange={(v) => setMode(v as RechargeMode)}
                className='grid gap-2'
              >
                <div className='flex items-start gap-3 rounded-lg border p-3'>
                  <RadioGroupItem
                    value='gpt'
                    id='recharge-mode-gpt'
                    className='mt-0.5'
                  />
                  <div className='flex-1 space-y-2'>
                    <Label
                      htmlFor='recharge-mode-gpt'
                      className='cursor-pointer text-sm font-medium'
                    >
                      {t('Recharge by GPT Amount')}
                    </Label>
                    <Input
                      type='number'
                      min={0}
                      step={0.1}
                      value={gptInput}
                      onChange={(e) => setGptInput(e.target.value)}
                      placeholder={t('GPT Amount')}
                      disabled={mode !== 'gpt'}
                      className='font-mono'
                    />
                  </div>
                </div>

                <div className='flex items-start gap-3 rounded-lg border p-3'>
                  <RadioGroupItem
                    value='base'
                    id='recharge-mode-base'
                    className='mt-0.5'
                  />
                  <div className='flex-1 space-y-2'>
                    <Label
                      htmlFor='recharge-mode-base'
                      className='cursor-pointer text-sm font-medium'
                    >
                      {t('Recharge by Base Balance')}
                    </Label>
                    <Input
                      type='number'
                      min={0}
                      step={0.01}
                      value={baseInput}
                      onChange={(e) => setBaseInput(e.target.value)}
                      placeholder={t('Base Balance (USD)')}
                      disabled={mode !== 'base'}
                      className='font-mono'
                    />
                  </div>
                </div>
              </RadioGroup>

              <div className='space-y-2 rounded-lg border p-3'>
                <div className='flex items-center justify-between text-sm'>
                  <span className='text-muted-foreground'>
                    {t('Will deduct base balance')}
                  </span>
                  <span
                    className={`font-mono font-semibold tabular-nums ${
                      insufficientBalance ? 'text-destructive' : ''
                    }`}
                  >
                    {formatQuota(effectiveBaseQuota)}
                  </span>
                </div>
                <div className='flex items-center justify-between text-sm'>
                  <span className='text-muted-foreground'>
                    {t('Will gain GPT quota')}
                  </span>
                  <span className='font-mono font-semibold text-green-600 tabular-nums'>
                    {formatGptQuota(effectiveGptQuota)}
                  </span>
                </div>
                {insufficientBalance && !invalidAmount && (
                  <p className='text-destructive text-xs font-medium'>
                    {t('Insufficient base balance')}
                  </p>
                )}
              </div>
            </div>
          </TabsContent>
        </Tabs>

        <DialogFooter className='grid grid-cols-2 gap-2 sm:flex'>
          <Button
            variant='outline'
            onClick={() => onOpenChange(false)}
            disabled={submitting}
          >
            {t('Cancel')}
          </Button>
          <Button onClick={handleSubmit} disabled={!canSubmit}>
            {submitting && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
            {t('Confirm Recharge')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
