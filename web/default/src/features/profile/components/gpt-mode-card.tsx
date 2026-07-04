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
import { useMemo, useState } from 'react'
import { Loader2, Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { StatusBadge } from '@/components/status-badge'
import { DEFAULT_QUOTA_WARNING_THRESHOLD } from '../constants'
import { useProfile } from '../hooks'
import { parseUserSettings } from '../lib'
import type { UpdateUserSettingsRequest } from '../types'

// ============================================================================
// GPT Mode Card Component
// ============================================================================

const DISCLAIMER_TEXT =
  '该功能仅支持在除中国大陆之外地区使用，禁止在中国大陆地区使用。请遵守本国法律。如您人在中国大陆使用该功能，由您自己承担所产生的法律责任。本站已启用区域限制，如您使用其他方法跳过本限制，本站不承担由此产生的责任。'

interface GptModeCardProps {
  loading: boolean
}

export function GptModeCard({ loading: pageLoading }: GptModeCardProps) {
  const { t } = useTranslation()
  const { profile, updateSettings, updating } = useProfile()
  const [enableDialogOpen, setEnableDialogOpen] = useState(false)
  const [exitDialogOpen, setExitDialogOpen] = useState(false)
  const [acknowledged, setAcknowledged] = useState(false)

  const gptModeEnabled = useMemo(() => {
    const settings = parseUserSettings(profile?.setting)
    return settings.gpt_mode === true
  }, [profile?.setting])

  // The backend UpdateUserSetting endpoint validates notify_type and
  // quota_warning_threshold unconditionally and performs an overwrite-style
  // reconstruction of the whole UserSetting. To update gpt_mode safely we
  // must therefore re-send the existing settings alongside the new value.
  const baseSettings = useMemo<UpdateUserSettingsRequest>(() => {
    const s = parseUserSettings(profile?.setting)
    return {
      notify_type: s.notify_type || 'email',
      quota_warning_threshold:
        s.quota_warning_threshold ?? DEFAULT_QUOTA_WARNING_THRESHOLD,
      notification_email: s.notification_email ?? '',
      webhook_url: s.webhook_url ?? '',
      webhook_secret: s.webhook_secret ?? '',
      bark_url: s.bark_url ?? '',
      gotify_url: s.gotify_url ?? '',
      gotify_token: s.gotify_token ?? '',
      gotify_priority: s.gotify_priority ?? 5,
      accept_unset_model_ratio_model:
        s.accept_unset_model_ratio_model ?? false,
      record_ip_log: s.record_ip_log ?? false,
      upstream_model_update_notify_enabled:
        s.upstream_model_update_notify_enabled ?? false,
    }
  }, [profile?.setting])

  const handleEnableDialogOpenChange = (open: boolean) => {
    if (!open) {
      setAcknowledged(false)
    }
    setEnableDialogOpen(open)
  }

  const handleEnable = async () => {
    const success = await updateSettings({ ...baseSettings, gpt_mode: true })
    if (success) {
      setEnableDialogOpen(false)
      setAcknowledged(false)
    }
  }

  const handleExit = async () => {
    const success = await updateSettings({ ...baseSettings, gpt_mode: false })
    if (success) {
      setExitDialogOpen(false)
    }
  }

  if (pageLoading) {
    return (
      <Card className='gap-0 overflow-hidden py-0'>
        <CardHeader className='p-3 sm:p-5'>
          <Skeleton className='h-6 w-48' />
          <Skeleton className='mt-2 h-4 w-64' />
        </CardHeader>
        <CardContent className='p-3 sm:p-5'>
          <Skeleton className='h-20 w-full' />
        </CardContent>
      </Card>
    )
  }

  return (
    <>
      <Card className='gap-0 overflow-hidden py-0'>
        <CardHeader className='p-3 sm:p-5'>
          <CardTitle className='text-lg tracking-tight sm:text-xl'>
            {t('GPT Mode')}
          </CardTitle>
          <CardDescription className='text-xs sm:text-sm'>
            {t('Enable to use GPT series models')}
          </CardDescription>
        </CardHeader>

        <CardContent className='p-3 sm:p-5'>
          <div className='space-y-6'>
            <div className='flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between xl:flex-col 2xl:flex-row'>
              <div className='flex items-start gap-4'>
                <div className='bg-muted rounded-md p-2'>
                  <Sparkles className='h-5 w-5' />
                </div>
                <div className='space-y-1'>
                  <div className='flex flex-wrap items-center gap-2'>
                    <p className='font-medium'>{t('GPT Mode')}</p>
                    <StatusBadge
                      label={gptModeEnabled ? t('Enabled') : t('Disabled')}
                      variant={gptModeEnabled ? 'success' : 'neutral'}
                      showDot
                      copyable={false}
                    />
                  </div>
                  <p className='text-muted-foreground text-sm'>
                    {t('Enable to use GPT series models')}
                  </p>
                </div>
              </div>

              {!gptModeEnabled && (
                <Button
                  className='w-full sm:w-auto xl:w-full 2xl:w-auto'
                  onClick={() => setEnableDialogOpen(true)}
                  disabled={updating}
                >
                  {t('Enable GPT Mode')}
                </Button>
              )}
            </div>

            {gptModeEnabled && (
              <div className='flex flex-col gap-3 border-t pt-6 sm:flex-row xl:flex-col 2xl:flex-row'>
                <Button
                  variant='destructive'
                  className='flex-1'
                  onClick={() => setExitDialogOpen(true)}
                  disabled={updating}
                >
                  {t('Exit')}
                </Button>
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Enable Dialog with Disclaimer */}
      <Dialog
        open={enableDialogOpen}
        onOpenChange={handleEnableDialogOpenChange}
      >
        <DialogContent className='sm:max-w-lg'>
          <DialogHeader>
            <DialogTitle>{t('Disclaimer')}</DialogTitle>
          </DialogHeader>

          <div className='space-y-4 py-2'>
            <p className='text-muted-foreground text-sm leading-relaxed whitespace-pre-wrap'>
              {DISCLAIMER_TEXT}
            </p>
            <div className='flex items-start gap-3'>
              <Checkbox
                id='gpt-mode-ack'
                checked={acknowledged}
                onCheckedChange={(checked) => setAcknowledged(checked === true)}
                className='mt-0.5'
              />
              <Label
                htmlFor='gpt-mode-ack'
                className='text-sm leading-5 font-normal cursor-pointer'
              >
                {t('I have read and understood')}
              </Label>
            </div>
          </div>

          <DialogFooter>
            <Button
              variant='outline'
              onClick={() => handleEnableDialogOpenChange(false)}
              disabled={updating}
            >
              {t('Cancel')}
            </Button>
            <Button
              onClick={handleEnable}
              disabled={!acknowledged || updating}
            >
              {updating && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
              {t('Confirm Enable')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Exit Confirmation Dialog */}
      <Dialog open={exitDialogOpen} onOpenChange={setExitDialogOpen}>
        <DialogContent className='sm:max-w-md'>
          <DialogHeader>
            <DialogTitle>{t('Confirm Exit')}</DialogTitle>
            <DialogDescription>
              {t('Are you sure you want to exit GPT mode?')}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant='outline'
              onClick={() => setExitDialogOpen(false)}
              disabled={updating}
            >
              {t('Cancel')}
            </Button>
            <Button
              variant='destructive'
              onClick={handleExit}
              disabled={updating}
            >
              {updating && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
              {t('Confirm Exit')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
