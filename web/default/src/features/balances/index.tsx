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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Search } from 'lucide-react'
import { getAllUserBalances, type UserBalance, type UserSubscriptionBrief } from './api'

const QUOTA_PER_UNIT = 500000

type FilterMode = 'all' | 'with_subscriptions' | 'renew_potential' | 'regression'
type SortMode = 'quota_desc' | 'quota_asc' | 'gpt_desc' | 'gpt_asc' | 'sub_duration_desc' | 'sub_duration_asc' | 'sub_remaining_desc' | 'sub_remaining_asc'

function formatUSD(value: number): string {
  const usd = value / QUOTA_PER_UNIT
  return usd.toFixed(5)
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

function getMaxSubscriptionEndTime(subs: UserSubscriptionBrief[] | undefined): number {
  if (!subs || subs.length === 0) return 0
  let max = 0
  for (const s of subs) {
    if (s.end_time > max) max = s.end_time
  }
  return max
}

function getMinSubscriptionEndTime(subs: UserSubscriptionBrief[] | undefined): number {
  if (!subs || subs.length === 0) return 0
  let min = Infinity
  for (const s of subs) {
    if (s.end_time > 0 && s.end_time < min) min = s.end_time
  }
  return min === Infinity ? 0 : min
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
      {user.renew_level && user.renew_level !== 'none' && (
        <div className={`absolute left-0 top-0 z-10 rounded-tl-xl rounded-br-xl px-2 py-0.5 text-xs font-medium text-white ${
          user.renew_level === 'high' ? 'bg-orange-500' :
          user.renew_level === 'medium' ? 'bg-blue-500' :
          'bg-gray-400'
        }`}>
          {user.renew_level === 'high' ? t('High') : user.renew_level === 'medium' ? t('Medium') : t('Low')}
        </div>
      )}
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
      <div className="mt-3 space-y-1 border-t pt-2 text-xs">
        <div className="flex items-center justify-between">
          <span className="text-muted-foreground">{t('Renew Score')}</span>
          <span className="font-medium tabular-nums" title={`${user.renew_score}`}>
            {user.renew_score > 99999999 ? '99999999…' : user.renew_score}
          </span>
        </div>
        <div className="flex items-center justify-between">
          <span className="text-muted-foreground">{t('Daily Consume')}</span>
          <span className="tabular-nums" title={`$${formatUSD(user.daily_consume)}`}>
            ${formatUSD(user.daily_consume).length > 8 ? formatUSD(user.daily_consume).slice(0, 7) + '…' : formatUSD(user.daily_consume)}
          </span>
        </div>
        <div className="flex items-center justify-between">
          <span className="text-muted-foreground">{t('Remaining Ratio')}</span>
          <span className="tabular-nums">
            {(user.quota_remaining_ratio * 100).toFixed(1)}%
          </span>
        </div>
        {user.regression_level && (
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground">{t('Regression')}</span>
            <span className={`rounded px-1.5 py-0.5 text-xs font-medium ${
              user.regression_level === 'high' ? 'bg-orange-100 text-orange-700 dark:bg-orange-500/20 dark:text-orange-400' :
              user.regression_level === 'medium' ? 'bg-blue-100 text-blue-700 dark:bg-blue-500/20 dark:text-blue-400' :
              'bg-gray-100 text-gray-600 dark:bg-gray-500/20 dark:text-gray-400'
            }`}>
              {user.regression_level === 'high' ? t('High') : user.regression_level === 'medium' ? t('Medium') : t('Low')}
            </span>
          </div>
        )}
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
  const [filter, setFilter] = useState<FilterMode>('all')
  const [sort, setSort] = useState<SortMode>('quota_desc')

  const { data, isLoading } = useQuery({
    queryKey: ['user-balances'],
    queryFn: getAllUserBalances,
  })

  const now = Math.floor(Date.now() / 1000)

  const processed = (data ?? []).filter(u => {
    // 搜索过滤
    if (search.trim()) {
      const q = search.trim().toLowerCase()
      if (!u.username.toLowerCase().includes(q) &&
          !u.display_name?.toLowerCase().includes(q) &&
          !String(u.id).includes(q)) {
        return false
      }
    }
    // 筛选过滤
    if (filter === 'with_subscriptions') {
      if (!u.subscriptions || u.subscriptions.length === 0) return false
    }
    if (filter === 'renew_potential') {
      if (u.renew_level === 'none' || u.renew_level === '') return false
    }
    if (filter === 'regression') {
      if (!u.regression_level) return false
    }
    return true
  })

  const sorted = processed.slice().sort((a, b) => {
    const aHasBalance = a.quota > 0 || a.gpt_quota > 0
    const bHasBalance = b.quota > 0 || b.gpt_quota > 0
    const aHasSub = (a.subscriptions?.length ?? 0) > 0
    const bHasSub = (b.subscriptions?.length ?? 0) > 0

    switch (sort) {
      case 'quota_desc': {
        if (aHasBalance !== bHasBalance) return aHasBalance ? -1 : 1
        return b.quota - a.quota
      }
      case 'quota_asc': {
        return a.quota - b.quota
      }
      case 'gpt_desc': {
        if (aHasBalance !== bHasBalance) return aHasBalance ? -1 : 1
        return b.gpt_quota - a.gpt_quota
      }
      case 'gpt_asc': {
        return a.gpt_quota - b.gpt_quota
      }
      case 'sub_duration_desc': {
        if (aHasSub !== bHasSub) return aHasSub ? -1 : 1
        return getMaxSubscriptionEndTime(b.subscriptions) - getMaxSubscriptionEndTime(a.subscriptions)
      }
      case 'sub_duration_asc': {
        const aEnd = getMinSubscriptionEndTime(a.subscriptions)
        const bEnd = getMinSubscriptionEndTime(b.subscriptions)
        return aEnd - bEnd
      }
      case 'sub_remaining_desc': {
        if (aHasSub !== bHasSub) return aHasSub ? -1 : 1
        const aRemaining = getMaxSubscriptionEndTime(a.subscriptions) - now
        const bRemaining = getMaxSubscriptionEndTime(b.subscriptions) - now
        return bRemaining - aRemaining
      }
      case 'sub_remaining_asc': {
        const aRemaining = getMinSubscriptionEndTime(a.subscriptions) - now
        const bRemaining = getMinSubscriptionEndTime(b.subscriptions) - now
        return aRemaining - bRemaining
      }
      default:
        return 0
    }
  })

  const maxQuota = Math.max(...(data ?? []).map(u => u.quota), 1)
  const maxGptQuota = Math.max(...(data ?? []).map(u => u.gpt_quota), 1)

  return (
    <Main>
      <div className="min-h-0 flex-1 overflow-auto px-4 py-4">
        <div className="mb-4 flex flex-wrap items-center gap-3">
          <div className="relative min-w-[200px] flex-1 max-w-md">
            <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              placeholder={t('Search users...')}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="pl-9"
            />
          </div>
          <Select value={filter} onValueChange={(v) => setFilter(v as FilterMode)}>
            <SelectTrigger className="w-[140px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t('All')}</SelectItem>
              <SelectItem value="with_subscriptions">{t('With Subscriptions')}</SelectItem>
              <SelectItem value="renew_potential">{t('Renew Potential')}</SelectItem>
              <SelectItem value="regression">{t('Regression Potential')}</SelectItem>
            </SelectContent>
          </Select>
          <Select value={sort} onValueChange={(v) => setSort(v as SortMode)}>
            <SelectTrigger className="w-[180px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="quota_desc">{t('Quota (High to Low)')}</SelectItem>
              <SelectItem value="quota_asc">{t('Quota (Low to High)')}</SelectItem>
              <SelectItem value="gpt_desc">{t('GPT Quota (High to Low)')}</SelectItem>
              <SelectItem value="gpt_asc">{t('GPT Quota (Low to High)')}</SelectItem>
              <SelectItem value="sub_duration_desc">{t('Subscription Duration (Long to Short)')}</SelectItem>
              <SelectItem value="sub_duration_asc">{t('Subscription Duration (Short to Long)')}</SelectItem>
              <SelectItem value="sub_remaining_desc">{t('Subscription Remaining (Long to Short)')}</SelectItem>
              <SelectItem value="sub_remaining_asc">{t('Subscription Remaining (Short to Long)')}</SelectItem>
            </SelectContent>
          </Select>
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
