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
import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { SettingsSection } from '../../components/settings-section'
import { getGptGroupSettings, updateGptGroupSettings } from '../api'
import type { GptGroupSettings } from '../types'
import { GptGroupRatioVisualEditor } from './gpt-group-ratio-visual-editor'

export function GptGroupsSection() {
  const { t } = useTranslation()
  const [settings, setSettings] = useState<GptGroupSettings>({
    groupRatio: {},
    userUsableGroups: {},
  })
  const [isLoading, setIsLoading] = useState(true)
  const [isSaving, setIsSaving] = useState(false)
  const [hasDuplicateNames, setHasDuplicateNames] = useState(false)

  useEffect(() => {
    let cancelled = false
    async function load() {
      setIsLoading(true)
      try {
        const result = await getGptGroupSettings()
        if (cancelled) return
        setSettings(result)
      } catch {
        // errors are already handled by the axios interceptor
      } finally {
        if (!cancelled) setIsLoading(false)
      }
    }
    load()
    return () => {
      cancelled = true
    }
  }, [])

  const handleChange = useCallback((next: GptGroupSettings) => {
    setSettings(next)
  }, [])

  const handleDuplicateNamesChange = useCallback((names: string[]) => {
    setHasDuplicateNames(names.length > 0)
  }, [])

  const handleSave = async () => {
    setIsSaving(true)
    try {
      await updateGptGroupSettings(
        settings.groupRatio,
        settings.userUsableGroups
      )
      toast.success(t('GPT group settings updated successfully'))
    } catch {
      // errors are already handled by the axios interceptor
    } finally {
      setIsSaving(false)
    }
  }

  if (isLoading) {
    return (
      <SettingsSection
        title={t('GPT Group Settings')}
        description={t('Configure dedicated groups for GPT mode users')}
      >
        <div className='space-y-2'>
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className='h-12 w-full' />
          ))}
        </div>
      </SettingsSection>
    )
  }

  return (
    <SettingsSection
      title={t('GPT Group Settings')}
      description={t('Configure dedicated groups for GPT mode users')}
    >
      <GptGroupRatioVisualEditor
        groupRatio={settings.groupRatio}
        userUsableGroups={settings.userUsableGroups}
        onChange={handleChange}
        onDuplicateNamesChange={handleDuplicateNamesChange}
      />
      <div className='pt-4'>
        <Button onClick={handleSave} disabled={isSaving || hasDuplicateNames}>
          {isSaving ? t('Loading') : t('Save')}
        </Button>
      </div>
    </SettingsSection>
  )
}
