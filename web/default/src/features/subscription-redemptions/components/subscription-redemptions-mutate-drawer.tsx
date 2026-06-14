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
import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { addTimeToDate } from '@/lib/time'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { DateTimePicker } from '@/components/datetime-picker'
import {
  createSubscriptionRedemption,
  updateSubscriptionRedemption,
  getSubscriptionRedemption,
} from '../api'
import { SUCCESS_MESSAGES } from '../constants'
import {
  getSubscriptionRedemptionFormSchema,
  type SubscriptionRedemptionFormValues,
  SUBSCRIPTION_REDEMPTION_FORM_DEFAULT_VALUES,
  transformFormDataToPayload,
  transformRedemptionToFormDefaults,
} from '../lib'
import { type SubscriptionRedemption } from '../types'
import { useSubscriptionRedemptions } from './subscription-redemptions-provider'

// Import subscription plans API
import { getAdminPlans } from '@/features/subscriptions/api'

type SubscriptionRedemptionsMutateDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: SubscriptionRedemption
}

export function SubscriptionRedemptionsMutateDrawer({
  open,
  onOpenChange,
  currentRow,
}: SubscriptionRedemptionsMutateDrawerProps) {
  const { t } = useTranslation()
  const isUpdate = !!currentRow
  const { triggerRefresh } = useSubscriptionRedemptions()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [plans, setPlans] = useState<
    Array<{ id: number; title: string; price_amount: number; currency: string }>
  >([])

  const form = useForm<SubscriptionRedemptionFormValues>({
    resolver: zodResolver(getSubscriptionRedemptionFormSchema(t)),
    defaultValues: SUBSCRIPTION_REDEMPTION_FORM_DEFAULT_VALUES,
  })

  // Load subscription plans
  useEffect(() => {
    if (open) {
      getAdminPlans().then((result) => {
        if (result.success && result.data) {
          const enabledPlans = result.data
            .filter((p) => p.plan.enabled)
            .map((p) => ({
              id: p.plan.id,
              title: p.plan.title,
              price_amount: p.plan.price_amount,
              currency: p.plan.currency,
            }))
          setPlans(enabledPlans)
        }
      })
    }
  }, [open])

  // Load existing data when updating
  useEffect(() => {
    if (open && isUpdate && currentRow) {
      // For update, fetch fresh data
      getSubscriptionRedemption(currentRow.id).then((result) => {
        if (result.success && result.data) {
          form.reset(transformRedemptionToFormDefaults(result.data))
        }
      })
    } else if (open && !isUpdate) {
      // For create, reset to defaults
      form.reset(SUBSCRIPTION_REDEMPTION_FORM_DEFAULT_VALUES)
    }
  }, [open, isUpdate, currentRow, form])

  const onSubmit = async (data: SubscriptionRedemptionFormValues) => {
    setIsSubmitting(true)
    try {
      const basePayload = transformFormDataToPayload(data)

      if (isUpdate && currentRow) {
        const result = await updateSubscriptionRedemption({
          ...basePayload,
          id: currentRow.id,
        })
        if (result.success) {
          toast.success(t(SUCCESS_MESSAGES.SUBSCRIPTION_REDEMPTION_UPDATED))
          onOpenChange(false)
          triggerRefresh()
        } else {
          toast.error(result.message || t('Failed to update subscription redemption code'))
        }
      } else {
        // Create mode
        const result = await createSubscriptionRedemption(basePayload)
        if (result.success) {
          const count = result.data?.length || 0
          toast.success(
            count > 1
              ? t('Successfully created {{count}} subscription redemption codes', {
                  count,
                })
              : t(SUCCESS_MESSAGES.SUBSCRIPTION_REDEMPTION_CREATED)
          )
          onOpenChange(false)
          triggerRefresh()
        } else {
          toast.error(result.message || t('Failed to create subscription redemption code'))
        }
      }
    } catch (error) {
      toast.error(t('An error occurred'))
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleSetExpiry = (months: number, days: number, hours: number) => {
    const newDate = addTimeToDate(months, days, hours)
    form.setValue('expired_time', newDate)
  }

  return (
    <Sheet
      open={open}
      onOpenChange={(v) => {
        onOpenChange(v)
        if (!v) {
          form.reset()
        }
      }}
    >
      <SheetContent className='flex h-dvh w-full flex-col gap-0 overflow-hidden p-0 sm:max-w-[600px]'>
        <SheetHeader className='border-b px-4 py-3 text-start sm:px-6 sm:py-4'>
          <SheetTitle>
            {isUpdate
              ? t('Update Subscription Redemption Code')
              : t('Create Subscription Redemption Code')}
          </SheetTitle>
          <SheetDescription>
            {isUpdate
              ? t('Update the subscription redemption code by providing necessary info.')
              : t('Add new subscription redemption code(s) by providing necessary info.')}{' '}
            {t('Click save when you&apos;re done.')}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form
            id='subscription-redemption-form'
            onSubmit={form.handleSubmit(onSubmit)}
            className='flex-1 space-y-4 overflow-y-auto px-3 py-3 pb-4 sm:space-y-6 sm:px-4'
          >
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Name')}</FormLabel>
                  <FormControl>
                    <Input {...field} placeholder={t('Enter a name')} />
                  </FormControl>
                  <FormDescription>
                    {t('Name for this redemption code (1-20 characters)')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='plan_id'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Subscription Plan')}</FormLabel>
                  <Select
                    value={field.value ? String(field.value) : ''}
                    onValueChange={(value) => field.onChange(Number(value))}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder={t('Select a subscription plan')} />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {plans.map((plan) => (
                        <SelectItem key={plan.id} value={String(plan.id)}>
                          {plan.title} ({plan.price_amount} {plan.currency})
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormDescription>
                    {t('Select the subscription plan for this redemption code')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='expired_time'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Expiration Time')}</FormLabel>
                  <div className='space-y-2'>
                    <FormControl>
                      <DateTimePicker
                        value={field.value}
                        onChange={field.onChange}
                        placeholder={t('Never expires')}
                      />
                    </FormControl>
                    <div className='grid grid-cols-4 gap-1.5 sm:flex sm:gap-2'>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={() => handleSetExpiry(0, 0, 0)}
                      >
                        {t('Never')}
                      </Button>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={() => handleSetExpiry(1, 0, 0)}
                      >
                        {t('1M')}
                      </Button>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={() => handleSetExpiry(0, 7, 0)}
                      >
                        {t('1W')}
                      </Button>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={() => handleSetExpiry(0, 1, 0)}
                      >
                        {t('1 Day')}
                      </Button>
                    </div>
                  </div>
                  <FormDescription>
                    {t('Leave empty for never expires')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            {!isUpdate && (
              <FormField
                control={form.control}
                name='count'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Quantity')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        type='number'
                        min='1'
                        max='100'
                        placeholder={t('Number of codes to create')}
                        onChange={(e) =>
                          field.onChange(parseInt(e.target.value, 10) || 1)
                        }
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Create multiple redemption codes at once (1-100)')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            )}
          </form>
        </Form>
        <SheetFooter className='border-t px-4 py-3 sm:px-6 sm:py-4'>
          <SheetClose asChild>
            <Button variant='outline'>{t('Cancel')}</Button>
          </SheetClose>
          <Button
            type='submit'
            form='subscription-redemption-form'
            disabled={isSubmitting}
          >
            {isSubmitting ? t('Saving...') : t('Save')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}