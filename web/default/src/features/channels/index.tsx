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
import { LayoutGrid, Table2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { SectionPageLayout } from '@/components/layout'
import { ChannelsDialogs } from './components/channels-dialogs'
import { ChannelsModernView } from './components/channels-modern-view'
import { ChannelsPrimaryButtons } from './components/channels-primary-buttons'
import { ChannelsProvider, useChannels } from './components/channels-provider'
import { ChannelsTable } from './components/channels-table'

export function Channels() {
  const { t } = useTranslation()
  return (
    <ChannelsProvider>
      <ChannelsPageContent t={t} />

      <ChannelsDialogs />
    </ChannelsProvider>
  )
}

function ChannelsPageContent({
  t,
}: {
  t: ReturnType<typeof useTranslation>['t']
}) {
  const { viewMode, setViewMode } = useChannels()

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        <div className='flex min-w-0 items-center gap-2 sm:gap-3'>
          <span className='truncate'>{t('Channels')}</span>
          <Button
            type='button'
            size='sm'
            variant={viewMode === 'modern' ? 'secondary' : 'outline'}
            onClick={() =>
              setViewMode(viewMode === 'modern' ? 'legacy' : 'modern')
            }
            className='shrink-0 gap-1.5 text-xs font-medium'
          >
            {viewMode === 'modern' ? (
              <Table2 className='size-3.5' />
            ) : (
              <LayoutGrid className='size-3.5' />
            )}
            <span className='max-sm:hidden'>
              {viewMode === 'modern'
                ? t('Switch to Legacy Interface')
                : t('Switch to New Interface')}
            </span>
            <span className='sm:hidden'>
              {viewMode === 'modern' ? t('Legacy') : t('New')}
            </span>
          </Button>
        </div>
      </SectionPageLayout.Title>
      <SectionPageLayout.Description>
        {t('Manage API channels and provider configurations')}
      </SectionPageLayout.Description>
      <SectionPageLayout.Actions>
        <ChannelsPrimaryButtons />
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        {viewMode === 'modern' ? <ChannelsModernView /> : <ChannelsTable />}
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
