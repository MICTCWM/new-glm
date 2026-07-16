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
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Main } from '@/components/layout'
import { Input } from '@/components/ui/input'
import { Search } from 'lucide-react'
import { getAllUserBalances, type UserBalance, type UserSubscriptionBrief } from './api'

const QUOTA_PER_UNIT = 500000

function formatUSD(value: number): string {
  const usd = value / QUOTA_PER_UNIT
  const str = usd.toFixed(5)
  return str
}

function formatDaysLeft(endTime: number): string {
  if (!endTime) return ''
  const now = Math.floor(Date.now() / 1000)
  const secondsLeft = endTime - now
  if (secondsLeft <= 0) return '0d'
  const days = Math.floor(secondsLeft / 86400)
  if (days >= 1) return `${days}d`
  const hours = Math.floor(secondsLeft / 3600)
  return `${hours}h`
}

function SubscriptionTag({ sub }: { sub: UserSubscriptionBrief }) {
  const remaining = sub.amount_total - sub.amount_used
  const remainingUSD = formatUSD(remaining)
  const totalUSD = formatUSD(sub.amount_total)
  const daysLeft = formatDaysLeft(sub.end_time)

  return (
    <div className="flex items-center gap-1.5 rounded-md bg-green-50 dark:bg-green-500/10 px-2 py-1 text-xs">
      <span className="truncate font-medium text-green-700 dark:text-green-400">
        {sub.plan_title || `Plan #${sub.plan_id}`}
      </span>
      <span className="shrink-0 tabular-nums text-green-600 dark:text-green-500">
        {remainingUSD}/{totalUSD}
      </span>
      <span className="shrink-0 tabular-nums text-green-600/70 dark:text-green-500/70">
        ·{daysLeft}
      </span>
    </div>
  )
}

function BalanceCard({ user, maxQuota, maxGptQuota }: {
  user: UserBalance
  maxQuota: number
  maxGptQuota: number
}) {
  const { t } = useTranslation()
  const quotaUSD = user.quota / QUOTA_PER_UNIT
  const gptUSD = user.gpt_quota / QUOTA_PER_UNIT
  const maxQuotaUSD = maxQuota / QUOTA_PER_UNIT
  const maxGptUSD = maxGptQuota / QUOTA_PER_UNIT
  const quotaPercent = maxQuotaUSD > 0 ? Math.min((quotaUSD / maxQuotaUSD) * 100, 100) : 0
  const gptPercent = maxGptUSD > 0 ? Math.min((gptUSD / maxGptUSD) * 100, 100) : 0
  const quotaStr = quotaUSD.toFixed(5)
  const gptStr = gptUSD.toFixed(5)

  return (
    <div className="group relative rounded-xl border bg-card p-4 text-card-foreground shadow-sm transition-all duration-200 hover:z-10 hover:scale-105 hover:border-primary/30 hover:shadow-lg">
      <div className="mb-3 flex items-center justify-between gap-1">
        <span className="truncate text-sm font-medium">
          {user.display_name || user.username}
        </span>
        <span className="shrink-0 text-xs text-muted-foreground">
          #{user.id}
        </span>
      </div>
      <div className="space-y-2">
        <div>
          <div className="mb-0.5 flex items-center justify-between gap-2">
            <span className="text-xs text-muted-foreground">{t('Quota')}</span>
            <span className="text-xs tabular-nums" title={`$${quotaStr}`}>
              ${quotaStr.length > 10 ? quotaStr.slice(0, 9) + '…' : quotaStr}
            </span>
          </div>
          <div className="h-2 overflow-hidden rounded-full bg-muted">
            <div
              className="h-full rounded-full bg-blue-500 transition-all"
              style={{ width: `${quotaPercent}%` }}
            />
          </div>
        </div>
        <div>
          <div className="mb-0.5 flex items-center justify-between gap-2">
            <span className="text-xs text-muted-foreground">{t('GPT Quota')}</span>
            <span className="text-xs tabular-nums" title={`$${gptStr}`}>
              ${gptStr.length > 10 ? gptStr.slice(0, 9) + '…' : gptStr}
            </span>
          </div>
          <div className="h-2 overflow-hidden rounded-full bg-muted">
            <div
              className="h-full rounded-full bg-purple-500 transition-all"
              style={{ width: `${gptPercent}%` }}
            />
          </div>
        </div>
      </div>
      {user.subscriptions && user.subscriptions.length > 0 && (
        <div className="mt-3 space-y-1.5">
          {user.subscriptions.map((sub, idx) => (
            <SubscriptionTag key={idx} sub={sub} />
          ))}
        </div>
      )}
    </div>
  )
}

export function Balances() {
  const { t } = useTranslation()
  const [search, setSearch] = useState('')

  const { data, isLoading } = useQuery({
    queryKey: ['user-balances'],
    queryFn: getAllUserBalances,
  })

  const filtered = (data ?? []).filter(u => {
    if (!search.trim()) return true
    const q = search.trim().toLowerCase()
    return (
      u.username.toLowerCase().includes(q) ||
      u.display_name?.toLowerCase().includes(q) ||
      String(u.id).includes(q)
    )
  })

  const sorted = filtered.slice().sort((a, b) => {
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
        <div className="mb-4">
          <div className="relative max-w-md">
            <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              placeholder={t('Search users...')}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="pl-9"
            />
          </div>
        </div>
        {isLoading ? (
          <div className="flex h-full items-center justify-center">
            <p className="text-muted-foreground">{t('Loading...')}</p>
          </div>
        ) : sorted.length === 0 ? (
          <div className="flex h-full items-center justify-center">
            <p className="text-muted-foreground">{t('No data')}</p>
          </div>
        ) : (
          <div className="grid gap-3 grid-cols-[repeat(auto-fill,minmax(260px,1fr))]">
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
