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
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { SettingsSection } from '../../components/settings-section'
import { getOptionValue, useSystemOptions } from '../../hooks/use-system-options'
import { useUpdateOption } from '../../hooks/use-update-option'

export function GptModeToggleSection() {
  const { t } = useTranslation()
  const { data, isLoading } = useSystemOptions()
  const updateOption = useUpdateOption()
  const [isConfirmOpen, setIsConfirmOpen] = useState(false)

  const options = data?.data ?? []
  const { GptModeEnabled } = getOptionValue(options, {
    GptModeEnabled: true,
  })
  const enabled = Boolean(GptModeEnabled)

  const handleCheckedChange = (checked: boolean) => {
    if (checked) {
      // 开启 GPT 模式：直接更新，无需确认
      updateOption.mutate({ key: 'GptModeEnabled', value: 'true' })
    } else {
      // 关闭 GPT 模式：弹出确认对话框
      setIsConfirmOpen(true)
    }
  }

  const handleConfirmDisable = () => {
    updateOption.mutate({ key: 'GptModeEnabled', value: 'false' })
    setIsConfirmOpen(false)
  }

  if (isLoading) {
    return (
      <SettingsSection
        title={t('GPT Mode')}
        description={t(
          'When disabled, all users in GPT mode will be forced to exit, and their GPT quota will be automatically converted to base quota.'
        )}
      >
        <Skeleton className='h-12 w-full' />
      </SettingsSection>
    )
  }

  return (
    <SettingsSection
      title={t('GPT Mode')}
      description={t(
        'When disabled, all users in GPT mode will be forced to exit, and their GPT quota will be automatically converted to base quota.'
      )}
    >
      <div className='flex items-center justify-between rounded-lg border p-4'>
        <div className='space-y-0.5 pr-4'>
          <div className='text-sm font-medium'>
            {t('Enable GPT mode for users')}
          </div>
          <div className='text-muted-foreground text-xs'>
            {t(
              'When disabled, all users in GPT mode will be forced to exit, and their GPT quota will be automatically converted to base quota.'
            )}
          </div>
        </div>
        <Switch
          checked={enabled}
          onCheckedChange={handleCheckedChange}
          disabled={updateOption.isPending}
        />
      </div>

      <AlertDialog open={isConfirmOpen} onOpenChange={setIsConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Disable GPT Mode')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('Are you sure you want to disable GPT mode?')}{' '}
              {t(
                'When disabled, all users in GPT mode will be forced to exit, and their GPT quota will be automatically converted to base quota.'
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              variant='destructive'
              onClick={handleConfirmDisable}
              disabled={updateOption.isPending}
            >
              {t('Disable GPT Mode')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </SettingsSection>
  )
}
