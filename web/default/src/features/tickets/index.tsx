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
import { Plus, List } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { SectionPageLayout } from '@/components/layout'
import { useAuthStore } from '@/stores/auth-store'
import { TicketForm } from './components/ticket-form'
import { TicketList } from './components/ticket-list'

export function Tickets() {
  const { t } = useTranslation()
  const { auth } = useAuthStore()
  const isAdmin = auth.user && auth.user.role >= 10
  const [showForm, setShowForm] = useState(!isAdmin)
  const [refreshKey, setRefreshKey] = useState(0)

  const handleSuccess = () => {
    setRefreshKey((k) => k + 1)
    if (isAdmin) {
      setShowForm(false)
    }
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Tickets')}</SectionPageLayout.Title>
      <SectionPageLayout.Description>
        {t('Submit and manage support tickets')}
      </SectionPageLayout.Description>
      <SectionPageLayout.Actions>
        <div className='flex items-center gap-2'>
          <Button
            variant={showForm ? 'default' : 'outline'}
            size='sm'
            onClick={() => setShowForm(true)}
          >
            <Plus className='h-4 w-4 mr-1' />
            {t('New Ticket')}
          </Button>
          <Button
            variant={!showForm ? 'default' : 'outline'}
            size='sm'
            onClick={() => setShowForm(false)}
          >
            <List className='h-4 w-4 mr-1' />
            {isAdmin ? t('All Tickets') : t('My Tickets')}
          </Button>
        </div>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        {showForm ? (
          <TicketForm onSuccess={handleSuccess} />
        ) : (
          <TicketList key={refreshKey} isAdmin={isAdmin} />
        )}
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
