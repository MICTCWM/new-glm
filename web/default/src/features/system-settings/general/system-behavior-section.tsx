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
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
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
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { ChannelMultiSelect } from '@/features/vendors/components/channel-multi-select'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'

const behaviorSchema = z.object({
  OverloadProtectionRPM: z.coerce.number().int().min(0).max(100000),
  OverloadProtectionChannelIds: z.array(z.number().int().positive()),
  LimitedInputTokenChannelIds: z.array(z.number().int().positive()),
  ReassuranceChannelIds: z.array(z.number().int().positive()),
  DailyUsageLimit: z.coerce.number().int().min(0).max(2147483647),
  RenewPotentialPassScore: z.coerce.number().min(0).max(100),
  LowQuotaAlertPercent: z.coerce.number().min(0).max(100),
  ShortExpiryDays: z.coerce.number().min(1).max(365),
  ConsumeStatPeriodDays: z.coerce.number().min(1).max(365),
  RequestMaxDuration: z.coerce.number().min(0).max(86400),
  DefaultCollapseSidebar: z.boolean(),
  DemoSiteEnabled: z.boolean(),
  SelfUseModeEnabled: z.boolean(),
  RegionBlockEnabled: z.boolean(),
  ModelNoImageModels: z.string(),
})

type BehaviorFormValues = z.infer<typeof behaviorSchema>

type SystemBehaviorSectionProps = {
  defaultValues: BehaviorFormValues
}

export function SystemBehaviorSection({
  defaultValues,
}: SystemBehaviorSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const form = useForm({
    resolver: zodResolver(behaviorSchema),
    defaultValues,
  })

  useResetForm(form, defaultValues)

  const onSubmit = async (data: BehaviorFormValues) => {
    const updates = Object.entries(data).filter(
      ([key, value]) => value !== defaultValues[key as keyof BehaviorFormValues]
    )

    for (const [key, value] of updates) {
      await updateOption.mutateAsync({
        key,
        value: Array.isArray(value) ? JSON.stringify(value) : value,
      })
    }
  }

  return (
    <SettingsSection
      title={t('System Behavior')}
      description={t('Configure system-wide behavior and defaults')}
    >
      <Form {...form}>
        <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-6'>
          <FormField
            control={form.control}
            name='OverloadProtectionRPM'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Overload Protection RPM')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min='0'
                    max='100000'
                    value={field.value as number}
                    onChange={(e) => field.onChange(e.target.valueAsNumber)}
                    name={field.name}
                    onBlur={field.onBlur}
                    ref={field.ref}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Shared RPM threshold for selected channels (0 = disabled, default 30)'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='OverloadProtectionChannelIds'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Overload Protection Channels')}</FormLabel>
                <FormControl>
                  <ChannelMultiSelect
                    value={field.value.map(String)}
                    onChange={(values) =>
                      field.onChange(
                        Array.from(
                          new Set(
                            values
                              .map((value) => Number(value))
                              .filter(
                                (value) => Number.isInteger(value) && value > 0
                              )
                          )
                        )
                      )
                    }
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Only selected channels share this RPM budget. Requests above the threshold are routed to fallback channels; all other channels are unaffected.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='ReassuranceChannelIds'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('支持安抚性语言的渠道')}</FormLabel>
                <FormControl>
                  <ChannelMultiSelect
                    value={field.value.map(String)}
                    onChange={(values) =>
                      field.onChange(
                        Array.from(
                          new Set(
                            values
                              .map((value) => Number(value))
                              .filter(
                                (value) => Number.isInteger(value) && value > 0
                              )
                          )
                        )
                      )
                    }
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    '仅所选渠道在排队等待时会显示安抚性语言与硬推理提示；其他渠道保持静默，不显示任何排队安抚内容。'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='LimitedInputTokenChannelIds'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Input Token Limit Channels')}</FormLabel>
                <FormControl>
                  <ChannelMultiSelect
                    value={field.value.map(String)}
                    onChange={(values) =>
                      field.onChange(
                        Array.from(
                          new Set(
                            values
                              .map((value) => Number(value))
                              .filter(
                                (value) => Number.isInteger(value) && value > 0
                              )
                          )
                        )
                      )
                    }
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Only selected channels are capped at 360000 input tokens. Requests exceeding this are rejected with a context-too-long error; all other channels are unaffected.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='DailyUsageLimit'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Daily Usage Limit')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min='0'
                    max='2147483647'
                    value={field.value as number}
                    onChange={(e) => field.onChange(e.target.valueAsNumber)}
                    name={field.name}
                    onBlur={field.onBlur}
                    ref={field.ref}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    "When today's consume-log quota reaches this value (internal quota units), new normal requests use fallback channels (0 = disabled)"
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='RenewPotentialPassScore'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Renew Potential Pass Score')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min='0'
                    max='100'
                    value={field.value as number}
                    onChange={(e) => field.onChange(e.target.valueAsNumber)}
                    name={field.name}
                    onBlur={field.onBlur}
                    ref={field.ref}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Minimum score for a user to be considered a high-renewal-potential user (0-100)'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='LowQuotaAlertPercent'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Low Quota Alert Percent')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min='0'
                    max='100'
                    value={field.value as number}
                    onChange={(e) => field.onChange(e.target.valueAsNumber)}
                    name={field.name}
                    onBlur={field.onBlur}
                    ref={field.ref}
                  />
                </FormControl>
                <FormDescription>
                  {t('Percentage threshold for low quota warning (0-100)')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='ShortExpiryDays'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Short Expiry Days')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min='1'
                    max='365'
                    value={field.value as number}
                    onChange={(e) => field.onChange(e.target.valueAsNumber)}
                    name={field.name}
                    onBlur={field.onBlur}
                    ref={field.ref}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Days threshold for short-term subscription expiry (1-365)'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='ConsumeStatPeriodDays'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Consume Stat Period Days')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min='1'
                    max='365'
                    value={field.value as number}
                    onChange={(e) => field.onChange(e.target.valueAsNumber)}
                    name={field.name}
                    onBlur={field.onBlur}
                    ref={field.ref}
                  />
                </FormControl>
                <FormDescription>
                  {t('Period in days for consumption statistics (1-365)')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='RequestMaxDuration'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Request Max Duration (seconds)')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min='0'
                    max='86400'
                    value={field.value as number}
                    onChange={(e) => field.onChange(e.target.valueAsNumber)}
                    name={field.name}
                    onBlur={field.onBlur}
                    ref={field.ref}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Maximum request duration in seconds (0 = no limit, default 900)'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='DefaultCollapseSidebar'
            render={({ field }) => (
              <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                <div className='space-y-0.5'>
                  <FormLabel className='text-base'>
                    {t('Default Collapse Sidebar')}
                  </FormLabel>
                  <FormDescription>
                    {t('Sidebar collapsed by default for new users')}
                  </FormDescription>
                </div>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='DemoSiteEnabled'
            render={({ field }) => (
              <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                <div className='space-y-0.5'>
                  <FormLabel className='text-base'>
                    {t('Demo Site Mode')}
                  </FormLabel>
                  <FormDescription>
                    {t('Enable demo mode with limited functionality')}
                  </FormDescription>
                </div>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='SelfUseModeEnabled'
            render={({ field }) => (
              <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                <div className='space-y-0.5'>
                  <FormLabel className='text-base'>
                    {t('Self-Use Mode')}
                  </FormLabel>
                  <FormDescription>
                    {t('Optimize system for self-hosted single-user usage')}
                  </FormDescription>
                </div>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='RegionBlockEnabled'
            render={({ field }) => (
              <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                <div className='space-y-0.5'>
                  <FormLabel className='text-base'>
                    {t('Region Block Enabled')}
                  </FormLabel>
                  <FormDescription>
                    {t('Block access from users outside the service area')}
                  </FormDescription>
                </div>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='ModelNoImageModels'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Models Not Supporting Images')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={6}
                    placeholder={t('one model per line')}
                    {...field}
                    onChange={(event) => field.onChange(event.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'By default all models support images. Requests containing images sent to a model listed here are rejected. Enter one model name per line.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <Button type='submit' disabled={updateOption.isPending}>
            {updateOption.isPending ? t('Saving...') : t('Save Changes')}
          </Button>
        </form>
      </Form>
    </SettingsSection>
  )
}
