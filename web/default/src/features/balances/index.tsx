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
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Main } from '@/components/layout'
import { getAllUserBalances, type UserBalance } from './api'

function formatQuotaDisplay(value: number): string {
  const str = value.toString()
  if (str.length <= 8) return str
  return str.slice(0, 7) + '…'
}

function BalanceCard({ user, maxQuota, maxGptQuota }: {
  user: UserBalance
  maxQuota: number
  maxGptQuota: number
}) {
  const { t } = useTranslation()
  const quotaPercent = maxQuota > 0 ? Math.min((user.quota / maxQuota) * 100, 100) : 0
  const gptPercent = maxGptQuota > 0 ? Math.min((user.gpt_quota / maxGptQuota) * 100, 100) : 0
  const quotaStr = user.quota.toString()
  const gptStr = user.gpt_quota.toString()

  return (
    <div className="rounded-lg border bg-card p-3 text-card-foreground shadow-sm transition-colors hover:bg-accent/5">
      <div className="mb-2 flex items-center justify-between gap-1">
        <span className="truncate text-sm font-medium">
          {user.display_name || user.username}
        </span>
        <span className="shrink-0 text-xs text-muted-foreground">
          #{user.id}
        </span>
      </div>
      <div className="space-y-1.5">
        <div className="flex items-center gap-2">
          <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-muted">
            <div
              className="h-full rounded-full bg-blue-500"
              style={{ width: `${quotaPercent}%` }}
            />
          </div>
          <span
            className="w-20 shrink-0 text-right text-xs tabular-nums"
            title={quotaStr}
          >
            {formatQuotaDisplay(user.quota)}
          </span>
        </div>
        <div className="flex items-center gap-2">
          <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-muted">
            <div
              className="h-full rounded-full bg-purple-500"
              style={{ width: `${gptPercent}%` }}
            />
          </div>
          <span
            className="w-20 shrink-0 text-right text-xs tabular-nums"
            title={gptStr}
          >
            {formatQuotaDisplay(user.gpt_quota)}
          </span>
        </div>
      </div>
    </div>
  )
}

export function Balances() {
  const { t } = useTranslation()
  const { data, isLoading } = useQuery({
    queryKey: ['user-balances'],
    queryFn: getAllUserBalances,
  })

  const sorted = (data ?? []).slice().sort((a, b) => {
    const aHas = a.quota > 0 || a.gpt_quota > 0
    const bHas = b.quota > 0 || b.gpt_quota > 0
    if (aHas !== bHas) return aHas ? -1 : 1
    return (b.quota + b.used_quota) - (a.quota + a.used_quota)
  })

  const maxQuota = Math.max(...(data ?? []).map(u => u.quota), 1)
  const maxGptQuota = Math.max(...(data ?? []).map(u => u.gpt_quota), 1)

  return (
    <Main>
      <div className="min-h-0 flex-1 overflow-auto px-4 py-4">
        {isLoading ? (
          <div className="flex h-full items-center justify-center">
            <p className="text-muted-foreground">{t('Loading...')}</p>
          </div>
        ) : sorted.length === 0 ? (
          <div className="flex h-full items-center justify-center">
            <p className="text-muted-foreground">{t('No data')}</p>
          </div>
        ) : (
          <div className="grid gap-2 grid-cols-[repeat(auto-fill,minmax(200px,1fr))]">
            {sorted.map(user => (
              <BalanceCard
                key={user.id}
                user={user}
                maxQuota={maxQuota}
                maxGptQuota={maxGptQuota}
              />
            ))}
          </div>
        )}
      </div>
    </Main>
  )
}
