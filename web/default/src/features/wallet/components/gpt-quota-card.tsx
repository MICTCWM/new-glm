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
import { Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import type { UserWalletData } from '../types'

interface GptQuotaCardProps {
  user: UserWalletData | null
  onRecharge: () => void
}

function formatGptQuota(value: number): string {
  return value.toLocaleString(undefined, { maximumFractionDigits: 4 })
}

export function GptQuotaCard({ user, onRecharge }: GptQuotaCardProps) {
  const { t } = useTranslation()
  const gptQuota = user?.gpt_quota ?? 0

  return (
    <Card className='bg-muted/20 py-0'>
      <CardContent className='flex flex-col gap-3 p-3 sm:flex-row sm:items-center sm:justify-between sm:gap-4 sm:p-4'>
        <div className='flex min-w-0 items-center gap-2.5'>
          <div className='bg-background flex size-8 shrink-0 items-center justify-center rounded-lg border'>
            <Sparkles className='text-muted-foreground size-4' />
          </div>
          <div className='min-w-0'>
            <h3 className='truncate text-sm font-semibold'>
              {t('GPT Quota')}
            </h3>
            <p className='text-muted-foreground line-clamp-1 text-xs'>
              {t('Exclusive balance for GPT models')}
            </p>
          </div>
        </div>

        <div className='flex items-center gap-3'>
          <div className='text-right'>
            <div className='text-muted-foreground truncate text-[10px] font-medium tracking-wider uppercase'>
              {t('Current Balance')}
            </div>
            <div className='mt-0.5 truncate font-mono text-base font-bold tabular-nums sm:text-lg'>
              {formatGptQuota(gptQuota)}
            </div>
          </div>
          <Button onClick={onRecharge} size='sm' className='h-9 shrink-0 px-3'>
            {t('Recharge')}
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}
