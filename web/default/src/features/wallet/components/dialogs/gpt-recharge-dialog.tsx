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
import { Loader2, Lock, Unlock } from 'lucide-react'
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
import { transferGptQuota, transferGptQuotaBack } from '../../api'
import {
  BASE_TO_GPT_RATIO,
  GPT_TO_BASE_RATIO,
} from '../../constants'
import { formatQuota, parseQuotaFromDollars, quotaUnitsToDollars } from '@/lib/format'
import { getSelfSubscriptions } from '@/features/subscriptions/api'
import { getPublicPlans } from '@/features/subscriptions/api'
import type { UserSubscriptionRecord, SubscriptionPlan } from '@/features/subscriptions/types'

interface GptRechargeDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSuccess: () => void | Promise<void>
  availableBaseQuota: number
  availableGptQuota: number
  direction: 'toGpt' | 'toBase'
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
  availableGptQuota,
  direction,
}: GptRechargeDialogProps) {
  const { t } = useTranslation()
  const [mode, setMode] = useState<RechargeMode>('gpt')
  const [gptInput, setGptInput] = useState('')
  const [baseInput, setBaseInput] = useState('')
  const [submitting, setSubmitting] = useState(false)

  // 订阅充值相关状态
  const [subscriptions, setSubscriptions] = useState<UserSubscriptionRecord[]>([])
  const [subscriptionsLoading, setSubscriptionsLoading] = useState(false)
  const [plans, setPlans] = useState<SubscriptionPlan[]>([])
  const [selectedSubId, setSelectedSubId] = useState<number | null>(null)
  const [daysInput, setDaysInput] = useState('')
  const [subscriptionSubmitting, setSubscriptionSubmitting] = useState(false)

  const isToGpt = direction === 'toGpt'

  // Reset state when dialog opens
  useEffect(() => {
    if (open) {
      setMode('gpt')
      setGptInput('')
      setBaseInput('')
      setSelectedSubId(null)
      setDaysInput('')
      setSubscriptions([])
      setPlans([])
    }
  }, [open])

  // 当方向为 toGpt 且 dialog 打开时，加载订阅列表
  useEffect(() => {
    if (open && isToGpt) {
      setSubscriptionsLoading(true)
      Promise.all([
        getSelfSubscriptions(),
        getPublicPlans(),
      ])
        .then(([subRes, planRes]) => {
          if (subRes.success && subRes.data) {
            const activeSubs = (subRes.data.subscriptions || []).filter(
              (s: UserSubscriptionRecord) =>
                s.subscription.status === 'active'
            )
            setSubscriptions(activeSubs)
          }
          if (planRes.success && planRes.data) {
            const planList = (planRes.data as { plan: SubscriptionPlan }[]).map(
              (p: { plan: SubscriptionPlan }) => p.plan
            )
            setPlans(planList)
          }
        })
        .catch(() => {
          setSubscriptions([])
          setPlans([])
        })
        .finally(() => {
          setSubscriptionsLoading(false)
        })
    }
  }, [open, isToGpt])

  // 计算选中订阅的剩余天数
  const selectedSubscription = subscriptions.find(
    (s) => s.subscription.id === selectedSubId
  )
  const remainingDays = selectedSubscription
    ? Math.max(
        0,
        Math.ceil(
          (selectedSubscription.subscription.end_time * 1000 - Date.now()) /
            (1000 * 86400)
        )
      )
    : 0

  const daysAmount = Number(daysInput) || 0
  const daysInvalid = daysAmount <= 0 || !Number.isInteger(daysAmount)
  const daysExceedsRemaining = daysAmount > remainingDays

  // 订阅充值的 GPT 额度计算（前端预估）
  const estimatedGptQuotaFromSub = (() => {
    if (!selectedSubscription || daysInvalid) return 0
    const sub = selectedSubscription.subscription
    const plan = plans.find((p) => p.id === sub.plan_id)
    if (!plan || plan.price_amount <= 0) return 0
    // 计算总月数
    let totalMonths = 0
    switch (plan.duration_unit) {
      case 'year':
        totalMonths = plan.duration_value * 12
        break
      case 'month':
        totalMonths = plan.duration_value
        break
      case 'day':
        totalMonths = plan.duration_value / 30
        break
      case 'hour':
        totalMonths = plan.duration_value / (30 * 24)
        break
      case 'custom':
        totalMonths = (plan.custom_seconds || 0) / (30 * 24 * 3600)
        break
    }
    if (totalMonths <= 0) return 0
    const totalDays = totalMonths * 30
    const dailyPrice = plan.price_amount / totalDays
    const totalPrice = dailyPrice * daysAmount
    // 订阅 USD 价格直接等于 GPT 额度数值（1 USD 订阅 = 1 GPT 额度）
    return totalPrice
  })()

  const canSubmitSubscription =
    selectedSubId !== null &&
    !daysInvalid &&
    !daysExceedsRemaining &&
    !subscriptionSubmitting

  const handleSubscriptionSubmit = async () => {
    if (!canSubmitSubscription) return
    try {
      setSubscriptionSubmitting(true)
      const res = await (
        await import('../../api')
      ).subscriptionToGpt({
        subscription_id: selectedSubId!,
        days: daysAmount,
      })
      if (res.success) {
        toast.success(res.message || i18next.t('Recharge successful'))
        onOpenChange(false)
        await onSuccess()
      } else {
        toast.error(res.message || i18next.t('Recharge failed'))
      }
    } catch (_error) {
      toast.error(i18next.t('Recharge failed'))
    } finally {
      setSubscriptionSubmitting(false)
    }
  }

  // 格式化时间戳为日期字符串
  const formatEndTime = (timestamp: number): string => {
    const d = new Date(timestamp * 1000)
    return d.toLocaleDateString(undefined, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    })
  }

  // 用户输入的是美金，需要转成内部额度
  const gptAmount = Number(gptInput) || 0
  const baseAmountUsd = Number(baseInput) || 0 // 用户输入的美金金额
  const baseAmountInput = parseQuotaFromDollars(baseAmountUsd) // 转成内部额度

  // 计算换算后的内部额度与 GPT 额度（两个方向的换算公式一致，区别在于扣除/获得的语义）：
  // gpt 模式 → 输入 GPT 数量：base = gpt * GPT_TO_BASE_RATIO，gpt = gptAmount
  // base 模式 → 输入 USD：base = baseAmountInput，gpt = baseAmountInput * BASE_TO_GPT_RATIO
  // toGpt:  effectiveBaseQuota 为要扣除的基础余额，effectiveGptQuota 为获得的 GPT 额度
  // toBase: effectiveGptQuota 为要扣除的 GPT 额度，effectiveBaseQuota 为获得的基础余额
  const effectiveBaseQuota =
    mode === 'gpt' ? gptAmount * GPT_TO_BASE_RATIO : baseAmountInput
  const effectiveGptQuota =
    mode === 'gpt' ? gptAmount : baseAmountInput * BASE_TO_GPT_RATIO

  // 余额不足判断：toGpt 检查 base 余额；toBase 检查 GPT 余额
  const insufficientBalance = isToGpt
    ? effectiveBaseQuota > availableBaseQuota
    : effectiveGptQuota > availableGptQuota
  const invalidAmount = isToGpt
    ? effectiveBaseQuota <= 0
    : effectiveGptQuota <= 0
  const canSubmit =
    !invalidAmount &&
    !insufficientBalance &&
    !submitting &&
    Number.isFinite(effectiveBaseQuota) &&
    Number.isFinite(effectiveGptQuota)

  const handleSubmit = async () => {
    if (!canSubmit) return
    try {
      setSubmitting(true)
      const response = isToGpt
        ? await transferGptQuota(Math.round(effectiveBaseQuota))
        : await transferGptQuotaBack(effectiveGptQuota)
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

  const handleTransferAll = () => {
    // 边界检查：如果对应余额为0或负数，直接返回
    if (isToGpt && availableBaseQuota <= 0) {
      return
    }
    if (!isToGpt && availableGptQuota <= 0) {
      return
    }

    if (isToGpt) {
      // 基础转GPT：将所有基础余额转换为GPT
      if (mode === 'gpt') {
        // gpt模式：计算对应的GPT数量
        const gptAmount = availableBaseQuota / GPT_TO_BASE_RATIO
        setGptInput(gptAmount.toFixed(4))
      } else {
        // base模式：将基础余额转换为USD
        const usdAmount = quotaUnitsToDollars(availableBaseQuota)
        setBaseInput(usdAmount.toFixed(2))
      }
    } else {
      // GPT转基础：将所有GPT余额转换为基础余额
      if (mode === 'gpt') {
        // gpt模式：直接使用所有GPT余额
        setGptInput(availableGptQuota.toFixed(4))
      } else {
        // base模式：计算对应的基础余额USD
        const baseQuota = availableGptQuota * GPT_TO_BASE_RATIO
        const usdAmount = quotaUnitsToDollars(baseQuota)
        setBaseInput(usdAmount.toFixed(2))
      }
    }
  }

  /**
   * 全部转换：直接将所有余额一次性转换并提交
   * 绕过手动输入，自动计算全部金额后立即调用提交
   */
  const handleTransferAllAndSubmit = async () => {
    if (submitting) return
    
    // 边界检查
    if (isToGpt && availableBaseQuota <= 0) return
    if (!isToGpt && availableGptQuota <= 0) return

    try {
      setSubmitting(true)
      
      if (isToGpt) {
        // 基础转GPT：将所有基础余额转换为GPT
        const response = await transferGptQuota(Math.round(availableBaseQuota))
        if (response.success) {
          toast.success(response.message || i18next.t('Recharge successful'))
          onOpenChange(false)
          await onSuccess()
        } else {
          toast.error(response.message || i18next.t('Recharge failed'))
        }
      } else {
        // GPT转基础：将所有GPT余额转换为基础余额
        const response = await transferGptQuotaBack(availableGptQuota)
        if (response.success) {
          toast.success(response.message || i18next.t('Recharge successful'))
          onOpenChange(false)
          await onSuccess()
        } else {
          toast.error(response.message || i18next.t('Recharge failed'))
        }
      }
    } catch (_error) {
      toast.error(i18next.t('Recharge failed'))
    } finally {
      setSubmitting(false)
    }
  }

  const title = isToGpt
    ? t('Recharge GPT Quota')
    : t('Convert GPT to Base')
  const description = isToGpt
    ? t('Convert base balance to GPT-exclusive balance')
    : t('Convert GPT-exclusive balance to base balance')

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-md'>
        <DialogHeader>
          <DialogTitle className='text-xl font-semibold'>
            {title}
          </DialogTitle>
          <DialogDescription>
            {description}
          </DialogDescription>
        </DialogHeader>

        <Tabs defaultValue={isToGpt ? 'subscription' : 'quota'} className='w-full'>
          <TabsList className={`grid w-full ${isToGpt ? 'grid-cols-2' : 'grid-cols-1'}`}>
            {isToGpt && (
              <TabsTrigger value='subscription'>
                <Unlock className='size-3.5' />
                {t('Subscription Recharge')}
              </TabsTrigger>
            )}
            <TabsTrigger value='quota'>{t('Quota Recharge')}</TabsTrigger>
          </TabsList>

          {isToGpt && (
            <TabsContent value='subscription' className='mt-4'>
              <div className='space-y-4'>
                {subscriptionsLoading ? (
                  <div className='text-muted-foreground flex items-center justify-center gap-2 py-8 text-sm'>
                    <Loader2 className='h-4 w-4 animate-spin' />
                    {t('Loading subscriptions...')}
                  </div>
                ) : subscriptions.length === 0 ? (
                  <div className='text-muted-foreground rounded-lg border border-dashed p-6 text-center text-sm'>
                    {t('No active subscriptions')}
                  </div>
                ) : (
                  <>
                    <div className='grid gap-2'>
                      {subscriptions.map((s) => {
                        const sub = s.subscription
                        const isSelected = selectedSubId === sub.id
                        const subRemaining = Math.max(
                          0,
                          Math.ceil(
                            (sub.end_time * 1000 - Date.now()) / (1000 * 86400)
                          )
                        )
                        const planInfo = plans.find((p) => p.id === sub.plan_id)
                        return (
                          <button
                            key={sub.id}
                            type='button'
                            onClick={() => {
                              setSelectedSubId(isSelected ? null : sub.id)
                              setDaysInput('')
                            }}
                            className={`flex w-full items-center gap-3 rounded-lg border p-3 text-left transition-colors ${
                              isSelected
                                ? 'border-primary bg-primary/5'
                                : 'hover:bg-accent/50'
                            }`}
                          >
                            <div className='flex-1'>
                              <div className='text-sm font-medium'>
                                {planInfo?.title || `${t('Subscription')} #${sub.id}`}
                              </div>
                              <div className='text-muted-foreground text-xs'>
                                {t('Expires')}: {formatEndTime(sub.end_time)} · {t('Remaining')} {subRemaining} {t('days')}
                              </div>
                            </div>
                          </button>
                        )
                      })}
                    </div>

                    {selectedSubscription && (
                      <div className='space-y-3 rounded-lg border p-3'>
                        <div className='space-y-1'>
                          <Label htmlFor='sub-days'>{t('Days to reduce')}</Label>
                          <Input
                            id='sub-days'
                            type='number'
                            min={1}
                            max={remainingDays}
                            value={daysInput}
                            onChange={(e) => setDaysInput(e.target.value)}
                            placeholder={t('Enter number of days')}
                            className='font-mono'
                          />
                        </div>

                        {daysExceedsRemaining && !daysInvalid && (
                          <p className='text-destructive text-xs font-medium'>
                            {t('Reduction exceeds remaining valid days')}
                          </p>
                        )}

                        <div className='space-y-2 rounded-lg bg-muted/40 p-3'>
                          <div className='flex items-center justify-between text-sm'>
                            <span className='text-muted-foreground'>
                              {t('Estimated GPT quota gained')}
                            </span>
                            <span className='font-mono font-semibold text-green-600 tabular-nums'>
                              {formatGptQuota(estimatedGptQuotaFromSub)}
                            </span>
                          </div>
                        </div>

                        <Button
                          type='button'
                          size='lg'
                          className='w-full'
                          onClick={handleSubscriptionSubmit}
                          disabled={!canSubmitSubscription}
                        >
                          {subscriptionSubmitting && (
                            <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                          )}
                          {t('Confirm Recharge')}
                        </Button>
                      </div>
                    )}
                  </>
                )}
              </div>
            </TabsContent>
          )}

          <TabsContent value='quota' className='mt-4'>
            <div className='space-y-4'>
              <div className='bg-muted/40 flex items-center justify-between rounded-lg px-3 py-2'>
                <span className='text-muted-foreground text-xs font-medium tracking-wider uppercase'>
                  {isToGpt
                    ? t('Available Base Balance')
                    : t('Available GPT Balance')}
                </span>
                <span className='font-mono text-sm font-semibold tabular-nums'>
                  {isToGpt
                    ? formatQuota(availableBaseQuota)
                    : formatGptQuota(availableGptQuota)}
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
                    <div className='flex items-center justify-between'>
                      <Label
                        htmlFor='recharge-mode-gpt'
                        className='cursor-pointer text-sm font-medium'
                      >
                        {t('Recharge by GPT Amount')}
                      </Label>
                      <Button
                        type='button'
                        variant='ghost'
                        size='sm'
                        className='h-auto px-2 py-1 text-xs'
                        onClick={handleTransferAll}
                        disabled={mode !== 'gpt' || submitting || (isToGpt ? availableBaseQuota <= 0 : availableGptQuota <= 0)}
                      >
                        {t('Transfer All')}
                      </Button>
                    </div>
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
                    <div className='flex items-center justify-between'>
                      <Label
                        htmlFor='recharge-mode-base'
                        className='cursor-pointer text-sm font-medium'
                      >
                        {t('Recharge by Base Balance')}
                      </Label>
                      <Button
                        type='button'
                        variant='ghost'
                        size='sm'
                        className='h-auto px-2 py-1 text-xs'
                        onClick={handleTransferAll}
                        disabled={mode !== 'base' || submitting || (isToGpt ? availableBaseQuota <= 0 : availableGptQuota <= 0)}
                      >
                        {t('Transfer All')}
                      </Button>
                    </div>
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

              {/* 全部转换按钮：将所有余额一次性全部转换并提交 */}
              <Button
                type='button'
                variant='default'
                size='lg'
                className='w-full'
                onClick={handleTransferAllAndSubmit}
                disabled={submitting || (isToGpt ? availableBaseQuota <= 0 : availableGptQuota <= 0)}
              >
                {submitting && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
                {isToGpt ? t('Convert All to GPT') : t('Convert All to Base')}
              </Button>

              <div className='space-y-2 rounded-lg border p-3'>
                {isToGpt ? (
                  <>
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
                  </>
                ) : (
                  <>
                    <div className='flex items-center justify-between text-sm'>
                      <span className='text-muted-foreground'>
                        {t('Will deduct GPT quota')}
                      </span>
                      <span
                        className={`font-mono font-semibold tabular-nums ${
                          insufficientBalance ? 'text-destructive' : ''
                        }`}
                      >
                        {formatGptQuota(effectiveGptQuota)}
                      </span>
                    </div>
                    <div className='flex items-center justify-between text-sm'>
                      <span className='text-muted-foreground'>
                        {t('Will gain base balance')}
                      </span>
                      <span className='font-mono font-semibold text-green-600 tabular-nums'>
                        {formatQuota(effectiveBaseQuota)}
                      </span>
                    </div>
                    {insufficientBalance && !invalidAmount && (
                      <p className='text-destructive text-xs font-medium'>
                        {t('Insufficient GPT quota')}
                      </p>
                    )}
                  </>
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
