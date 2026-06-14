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
import { useTranslation } from 'react-i18next'
import { SectionPageLayout } from '@/components/layout'
import { SubscriptionRedemptionsDialogs } from './components/subscription-redemptions-dialogs'
import { SubscriptionRedemptionsPrimaryButtons } from './components/subscription-redemptions-primary-buttons'
import { SubscriptionRedemptionsProvider } from './components/subscription-redemptions-provider'
import { SubscriptionRedemptionsTable } from './components/subscription-redemptions-table'

export function SubscriptionRedemptions() {
  const { t } = useTranslation()
  return (
    <SubscriptionRedemptionsProvider>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('Subscription Redemption Codes')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Description>
          {t('Manage subscription redemption codes for subscription activation')}
        </SectionPageLayout.Description>
        <SectionPageLayout.Actions>
          <SubscriptionRedemptionsPrimaryButtons />
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <SubscriptionRedemptionsTable />
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <SubscriptionRedemptionsDialogs />
    </SubscriptionRedemptionsProvider>
  )
}