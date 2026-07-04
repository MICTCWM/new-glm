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
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Skeleton } from '@/components/ui/skeleton'
import { SettingsSection } from '../../components/settings-section'
import { getAllChannels, getGptChannels, setGptChannels } from '../api'
import type { GptChannelInfo } from '../types'

export function GptChannelsSection() {
  const { t } = useTranslation()
  const [channels, setChannels] = useState<GptChannelInfo[]>([])
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set())
  const [isLoading, setIsLoading] = useState(true)
  const [isSaving, setIsSaving] = useState(false)

  useEffect(() => {
    let cancelled = false
    async function load() {
      setIsLoading(true)
      try {
        const [allRes, gptRes] = await Promise.all([
          getAllChannels(),
          getGptChannels(),
        ])
        if (cancelled) return
        const allChannels =
          ((allRes.data?.data as { items?: GptChannelInfo[] })?.items ??
            []) as GptChannelInfo[]
        const gptChannels =
          (gptRes.data?.data as GptChannelInfo[] | undefined) ?? []
        setChannels(allChannels)
        setSelectedIds(new Set(gptChannels.map((c) => c.id)))
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

  const handleToggle = (id: number, checked: boolean) => {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (checked) {
        next.add(id)
      } else {
        next.delete(id)
      }
      return next
    })
  }

  const handleSave = async () => {
    setIsSaving(true)
    try {
      await setGptChannels(Array.from(selectedIds), true)
      toast.success(t('Saved successfully'))
    } catch {
      // errors are already handled by the axios interceptor
    } finally {
      setIsSaving(false)
    }
  }

  if (isLoading) {
    return (
      <SettingsSection
        title={t('GPT Channels')}
        description={t(
          'GPT channel management, only users with GPT mode enabled can use these channels'
        )}
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
      title={t('GPT Channels')}
      description={t(
        'GPT channel management, only users with GPT mode enabled can use these channels'
      )}
    >
      {channels.length === 0 ? (
        <div className='text-muted-foreground py-8 text-center text-sm'>
          {t('No channels available')}
        </div>
      ) : (
        <div className='space-y-2'>
          <div className='text-muted-foreground flex items-center gap-3 border-b pb-2 text-sm font-medium'>
            <div className='w-6' />
            <div className='flex-1'>{t('Channel Name')}</div>
            <div className='w-20'>{t('Type')}</div>
            <div className='w-20'>{t('Status')}</div>
          </div>
          {channels.map((channel) => (
            <div
              key={channel.id}
              className='flex items-center gap-3 rounded-lg border p-2'
            >
              <Checkbox
                checked={selectedIds.has(channel.id)}
                onCheckedChange={(checked) =>
                  handleToggle(channel.id, checked === true)
                }
              />
              <div className='flex-1 truncate'>{channel.name}</div>
              <div className='text-muted-foreground w-20 text-sm'>
                {channel.type}
              </div>
              <div className='w-20'>
                {channel.status === 1 ? (
                  <Badge variant='secondary'>{t('Enabled')}</Badge>
                ) : (
                  <Badge variant='destructive'>{t('Disabled')}</Badge>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
      <div className='pt-4'>
        <Button onClick={handleSave} disabled={isSaving}>
          {isSaving ? t('Loading') : t('Save')}
        </Button>
      </div>
    </SettingsSection>
  )
}
