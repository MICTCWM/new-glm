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
import { forwardRef } from 'react'
import { Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import type { UserWalletData } from '../types'

interface GptQuotaCardProps {
  user: UserWalletData | null
  onRecharge: () => void
}

export const GptQuotaCard = forwardRef<HTMLDivElement, GptQuotaCardProps>(
  function GptQuotaCard({ onRecharge }, ref) {
    const { t } = useTranslation()

    return (
      <Card ref={ref} className='bg-muted/20 py-0'>
        <CardContent className='flex items-center justify-end gap-3 p-3 sm:gap-4 sm:p-4'>
          <div className='flex min-w-0 items-center gap-2.5'>
            <div className='bg-background flex size-8 shrink-0 items-center justify-center rounded-lg border'>
              <Sparkles className='text-muted-foreground size-4' />
            </div>
          </div>
          <Button
            onClick={onRecharge}
            size='sm'
            className='h-9 shrink-0 px-3'
          >
            {t('Recharge')}
          </Button>
        </CardContent>
      </Card>
    )
  }
)