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
import { useState, useEffect, useCallback, useMemo, useRef } from 'react'
import i18next from 'i18next'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { getSelf } from '@/lib/api'
import { formatQuota, parseQuotaFromDollars } from '@/lib/format'
import { useStatus } from '@/hooks/use-status'
import { useSystemConfig } from '@/hooks/use-system-config'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { SectionPageLayout } from '@/components/layout'
import { parseUserSettings } from '@/features/profile/lib/format'
import { AffiliateRewardsCard } from './components/affiliate-rewards-card'
import { BillingHistoryDialog } from './components/dialogs/billing-history-dialog'
import { CreemConfirmDialog } from './components/dialogs/creem-confirm-dialog'
import { GptRechargeDialog } from './components/dialogs/gpt-recharge-dialog'
import { PaymentConfirmDialog } from './components/dialogs/payment-confirm-dialog'
import { TransferDialog } from './components/dialogs/transfer-dialog'
import { GptQuotaCard } from './components/gpt-quota-card'
import { QuotaTransferAnimation } from './components/quota-transfer-animation'
import type { TransferDirection } from './components/quota-transfer-animation'
import { RechargeFormCard } from './components/recharge-form-card'
import { SubscriptionPlansCard } from './components/subscription-plans-card'
import { WalletStatsCard } from './components/wallet-stats-card'
import { transferGptQuota, transferGptQuotaBack } from './api'
import { DEFAULT_DISCOUNT_RATE } from './constants'
import {
  useTopupInfo,
  usePayment,
  useAffiliate,
  useRedemption,
  useCreemPayment,
  useWaffoPayment,
  useWaffoPancakePayment,
} from './hooks'
import {
  getDefaultPaymentType,
  getMinTopupAmount,
  isWaffoPancakePayment,
} from './lib'
import type {
  UserWalletData,
  PaymentMethod,
  PresetAmount,
  CreemProduct,
} from './types'

function formatGptQuota(value: number): string {
  return value.toLocaleString(undefined, { maximumFractionDigits: 4 })
}

interface WalletProps {
  initialShowHistory?: boolean
}

export function Wallet(props: WalletProps) {
  const { t } = useTranslation()
  const [user, setUser] = useState<UserWalletData | null>(null)
  const [userLoading, setUserLoading] = useState(true)
  const [topupAmount, setTopupAmount] = useState(0)
  const [selectedPreset, setSelectedPreset] = useState<number | null>(null)
  const [selectedPaymentMethod, setSelectedPaymentMethod] =
    useState<PaymentMethod>()
  const [paymentLoading, setPaymentLoading] = useState<string | null>(null)
  const [confirmDialogOpen, setConfirmDialogOpen] = useState(false)
  const [transferDialogOpen, setTransferDialogOpen] = useState(false)
  const [billingDialogOpen, setBillingDialogOpen] = useState(false)
  const [redemptionCode, setRedemptionCode] = useState('')
  const [creemDialogOpen, setCreemDialogOpen] = useState(false)
  const [selectedCreemProduct, setSelectedCreemProduct] =
    useState<CreemProduct | null>(null)
  const [showSubscriptionPanel, setShowSubscriptionPanel] = useState(true)
  const [gptRechargeOpen, setGptRechargeOpen] = useState(false)
  const [isTransferring, setIsTransferring] = useState(false)
  const [transferLoading, setTransferLoading] = useState(false)
  const [transferDirection, setTransferDirection] =
    useState<TransferDirection>('toGpt')
  const [transferAmount, setTransferAmount] = useState('')

  const baseCardRef = useRef<HTMLDivElement>(null)
  const gptCardRef = useRef<HTMLDivElement>(null)

  const { status } = useStatus()
  const { currency } = useSystemConfig()
  const { topupInfo, presetAmounts, loading: topupLoading } = useTopupInfo()

  // Calculate effective exchange rate - when display type is USD, use rate of 1
  const effectiveUsdExchangeRate = useMemo(() => {
    return currency?.quotaDisplayType === 'USD'
      ? 1
      : currency?.usdExchangeRate || 1
  }, [currency?.quotaDisplayType, currency?.usdExchangeRate])
  const {
    amount: paymentAmount,
    calculating,
    processing,
    calculatePaymentAmount,
    processPayment,
  } = usePayment()
  const {
    affiliateLink,
    loading: affiliateLoading,
    transferQuota,
    transferring,
  } = useAffiliate()
  const { redeeming, redeemCode } = useRedemption()
  const { processing: creemProcessing, processCreemPayment } = useCreemPayment()
  const { processWaffoPayment } = useWaffoPayment()
  const { processing: pancakeProcessing, processWaffoPancakePayment } =
    useWaffoPancakePayment()

  // Fetch and refresh user data
  const fetchUser = useCallback(async () => {
    try {
      setUserLoading(true)
      const response = await getSelf()
      if (response.success && response.data) {
        setUser(response.data as UserWalletData)
      }
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('Failed to fetch user data:', error)
    } finally {
      setUserLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchUser()
  }, [fetchUser])

  // Determine if GPT mode is enabled from user settings
  const gptModeEnabled = useMemo(() => {
    const settings = parseUserSettings(user?.setting)
    return settings.gpt_mode === true
  }, [user?.setting])

  useEffect(() => {
    if (props.initialShowHistory) {
      setBillingDialogOpen(true)
      window.history.replaceState({}, '', window.location.pathname)
    }
  }, [props.initialShowHistory])

  // Initialize topup amount when topup info is loaded
  useEffect(() => {
    if (topupInfo && topupAmount === 0) {
      const minTopup = getMinTopupAmount(topupInfo)
      setTopupAmount(minTopup)

      // Calculate initial payment amount with default payment type
      const defaultPaymentType = getDefaultPaymentType(topupInfo)
      calculatePaymentAmount(minTopup, defaultPaymentType)
    }
  }, [topupInfo, topupAmount, calculatePaymentAmount])

  // Get current payment type (selected or default)
  const getCurrentPaymentType = useCallback(() => {
    return selectedPaymentMethod?.type || getDefaultPaymentType(topupInfo)
  }, [selectedPaymentMethod, topupInfo])

  // Handle preset selection
  const handleSelectPreset = (preset: PresetAmount) => {
    setTopupAmount(preset.value)
    setSelectedPreset(preset.value)
    calculatePaymentAmount(preset.value, getCurrentPaymentType())
  }

  // Handle topup amount change
  const handleTopupAmountChange = (amount: number) => {
    setTopupAmount(amount)
    setSelectedPreset(null)
    calculatePaymentAmount(amount, getCurrentPaymentType())
  }

  // Handle payment method selection
  const handlePaymentMethodSelect = async (method: PaymentMethod) => {
    setSelectedPaymentMethod(method)
    setPaymentLoading(method.type)

    try {
      // Validate minimum topup
      const minTopup = getMinTopupAmount(topupInfo)
      if (topupAmount < minTopup) {
        return
      }

      // Calculate payment amount and show confirmation dialog
      await calculatePaymentAmount(topupAmount, method.type)
      setConfirmDialogOpen(true)
    } finally {
      setPaymentLoading(null)
    }
  }

  // Handle payment confirmation
  const handlePaymentConfirm = async () => {
    if (!selectedPaymentMethod) return

    const isPancake = isWaffoPancakePayment(selectedPaymentMethod.type)
    const success = isPancake
      ? await processWaffoPancakePayment(topupAmount)
      : await processPayment(topupAmount, selectedPaymentMethod.type)

    if (success) {
      setConfirmDialogOpen(false)
      await fetchUser()
    }
  }

  // Handle redemption
  const handleRedeem = async () => {
    if (!redemptionCode) return

    const success = await redeemCode(redemptionCode)
    if (success) {
      setRedemptionCode('')
      await fetchUser()
    }
  }

  // Handle transfer
  const handleTransfer = async (amount: number) => {
    const success = await transferQuota(amount)
    if (success) {
      await fetchUser()
    }
    return success
  }

  // Handle Creem product selection
  const handleCreemProductSelect = (product: CreemProduct) => {
    setSelectedCreemProduct(product)
    setCreemDialogOpen(true)
  }

  // Handle Creem payment confirmation
  const handleCreemConfirm = async () => {
    if (!selectedCreemProduct) return

    const success = await processCreemPayment(selectedCreemProduct.productId)
    if (success) {
      setCreemDialogOpen(false)
      setSelectedCreemProduct(null)
      await fetchUser()
    }
  }

  const handleWaffoMethodSelect = async (_method: unknown, index: number) => {
    const loadingKey = `waffo-${index}`
    setPaymentLoading(loadingKey)

    try {
      await processWaffoPayment(topupAmount, index)
    } finally {
      setPaymentLoading(null)
    }
  }

  // Get discount rate for current topup amount
  const getDiscountRate = useCallback(() => {
    return topupInfo?.discount?.[topupAmount] || DEFAULT_DISCOUNT_RATE
  }, [topupInfo, topupAmount])

  const handleSubscriptionAvailabilityChange = useCallback(
    (available: boolean) => {
      setShowSubscriptionPanel(available)
    },
    []
  )

  // Transfer base quota to GPT quota (uses USD input → internal quota)
  const handleTransferToGpt = async () => {
    const amountUsd = Number(transferAmount)
    if (!Number.isFinite(amountUsd) || amountUsd <= 0) return

    const baseQuota = parseQuotaFromDollars(amountUsd)
    const availableQuota = user?.quota ?? 0
    if (baseQuota > availableQuota) {
      toast.error(i18next.t('Insufficient base balance'))
      return
    }

    try {
      setTransferLoading(true)
      setTransferDirection('toGpt')
      const response = await transferGptQuota(Math.round(baseQuota))
      if (!response.success) {
        toast.error(response.message || i18next.t('Transfer failed'))
        return
      }
      await fetchUser()
      toast.success(i18next.t('Transfer successful'))
      setTransferAmount('')
      setIsTransferring(true)
      setTimeout(() => setIsTransferring(false), 1600)
    } catch (_error) {
      toast.error(i18next.t('Transfer failed'))
    } finally {
      setTransferLoading(false)
    }
  }

  // Transfer GPT quota back to base quota (uses GPT quota input directly)
  const handleTransferToBase = async () => {
    const gptQuotaAmount = Number(transferAmount)
    if (!Number.isFinite(gptQuotaAmount) || gptQuotaAmount <= 0) return

    const availableGpt = user?.gpt_quota ?? 0
    if (gptQuotaAmount > availableGpt) {
      toast.error(i18next.t('Insufficient GPT quota'))
      return
    }

    try {
      setTransferLoading(true)
      setTransferDirection('toBase')
      const response = await transferGptQuotaBack(gptQuotaAmount)
      if (!response.success) {
        toast.error(response.message || i18next.t('Transfer failed'))
        return
      }
      await fetchUser()
      toast.success(i18next.t('Transfer successful'))
      setTransferAmount('')
      setIsTransferring(true)
      setTimeout(() => setIsTransferring(false), 1600)
    } catch (_error) {
      toast.error(i18next.t('Transfer failed'))
    } finally {
      setTransferLoading(false)
    }
  }

  const transferBusy = transferLoading || isTransferring
  const transferAmountNum = Number(transferAmount)
  const canTransfer =
    !transferBusy &&
    Number.isFinite(transferAmountNum) &&
    transferAmountNum > 0

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Wallet')}</SectionPageLayout.Title>
        <SectionPageLayout.Description>
          {t('Manage your balance and payment methods')}
        </SectionPageLayout.Description>
        <SectionPageLayout.Content>
          <div className='mx-auto flex w-full max-w-7xl flex-col gap-4 sm:gap-5'>
            <WalletStatsCard ref={baseCardRef} user={user} loading={userLoading} />

            {gptModeEnabled && (
              <>
                <QuotaTransferAnimation
                  fromValue={user?.quota ?? 0}
                  toValue={user?.gpt_quota ?? 0}
                  fromLabel={t('Base Balance')}
                  toLabel={t('GPT Quota')}
                  fromColor='primary'
                  toColor='accent'
                  isTransferring={isTransferring}
                  transferDirection={transferDirection}
                  fromRef={baseCardRef}
                  toRef={gptCardRef}
                  formatFromValue={formatQuota}
                  formatToValue={formatGptQuota}
                />

                <div className='flex flex-col gap-2 rounded-lg border bg-muted/10 p-2 sm:flex-row sm:items-center sm:gap-2 sm:p-3'>
                  <Input
                    type='number'
                    min={0}
                    step={0.01}
                    value={transferAmount}
                    onChange={(e) => setTransferAmount(e.target.value)}
                    placeholder={t('Amount')}
                    disabled={transferBusy}
                    className='font-mono sm:flex-1'
                  />
                  <div className='flex gap-2'>
                    <Button
                      onClick={handleTransferToGpt}
                      disabled={!canTransfer}
                      size='sm'
                      className='flex-1 sm:flex-none'
                    >
                      {transferBusy && transferDirection === 'toGpt'
                        ? t('Transferring...')
                        : t('Base → GPT')}
                    </Button>
                    <Button
                      onClick={handleTransferToBase}
                      disabled={!canTransfer}
                      size='sm'
                      variant='secondary'
                      className='flex-1 sm:flex-none'
                    >
                      {transferBusy && transferDirection === 'toBase'
                        ? t('Transferring...')
                        : t('GPT → Base')}
                    </Button>
                  </div>
                </div>

                <GptQuotaCard
                  ref={gptCardRef}
                  user={user}
                  onRecharge={() => setGptRechargeOpen(true)}
                />
              </>
            )}

            <div
              className={
                showSubscriptionPanel
                  ? 'grid gap-4 xl:grid-cols-[minmax(0,1.05fr)_minmax(360px,0.95fr)] xl:items-start'
                  : 'grid gap-4'
              }
            >
              <div id='wallet-add-funds' className='scroll-mt-4'>
                <RechargeFormCard
                  topupInfo={topupInfo}
                  presetAmounts={presetAmounts}
                  selectedPreset={selectedPreset}
                  onSelectPreset={handleSelectPreset}
                  topupAmount={topupAmount}
                  onTopupAmountChange={handleTopupAmountChange}
                  paymentAmount={paymentAmount}
                  calculating={calculating}
                  onPaymentMethodSelect={handlePaymentMethodSelect}
                  paymentLoading={paymentLoading}
                  redemptionCode={redemptionCode}
                  onRedemptionCodeChange={setRedemptionCode}
                  onRedeem={handleRedeem}
                  redeeming={redeeming}
                  topupLink={topupInfo?.topup_link}
                  loading={topupLoading}
                  priceRatio={(status?.price as number) || 1}
                  usdExchangeRate={effectiveUsdExchangeRate}
                  onOpenBilling={() => setBillingDialogOpen(true)}
                  creemProducts={topupInfo?.creem_products}
                  enableCreemTopup={topupInfo?.enable_creem_topup}
                  onCreemProductSelect={handleCreemProductSelect}
                  enableWaffoTopup={topupInfo?.enable_waffo_topup}
                  waffoPayMethods={topupInfo?.waffo_pay_methods}
                  waffoMinTopup={topupInfo?.waffo_min_topup}
                  onWaffoMethodSelect={handleWaffoMethodSelect}
                  enableWaffoPancakeTopup={
                    topupInfo?.enable_waffo_pancake_topup
                  }
                />
              </div>

              <SubscriptionPlansCard
                topupInfo={topupInfo}
                onAvailabilityChange={handleSubscriptionAvailabilityChange}
              />
            </div>

            <AffiliateRewardsCard
              user={user}
              affiliateLink={affiliateLink}
              onTransfer={() => setTransferDialogOpen(true)}
              complianceConfirmed={
                topupInfo?.payment_compliance_confirmed !== false
              }
              loading={affiliateLoading}
            />
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <PaymentConfirmDialog
        open={confirmDialogOpen}
        onOpenChange={setConfirmDialogOpen}
        onConfirm={handlePaymentConfirm}
        topupAmount={topupAmount}
        paymentAmount={paymentAmount}
        paymentMethod={selectedPaymentMethod}
        calculating={calculating}
        processing={processing || pancakeProcessing}
        discountRate={getDiscountRate()}
        usdExchangeRate={effectiveUsdExchangeRate}
      />

      <TransferDialog
        open={transferDialogOpen}
        onOpenChange={setTransferDialogOpen}
        onConfirm={handleTransfer}
        availableQuota={user?.aff_quota ?? 0}
        transferring={transferring}
      />

      <BillingHistoryDialog
        open={billingDialogOpen}
        onOpenChange={setBillingDialogOpen}
      />

      <CreemConfirmDialog
        open={creemDialogOpen}
        onOpenChange={setCreemDialogOpen}
        onConfirm={handleCreemConfirm}
        product={selectedCreemProduct}
        processing={creemProcessing}
      />

      <GptRechargeDialog
        open={gptRechargeOpen}
        onOpenChange={setGptRechargeOpen}
        onSuccess={fetchUser}
        availableBaseQuota={user?.quota ?? 0}
      />
    </>
  )
}
