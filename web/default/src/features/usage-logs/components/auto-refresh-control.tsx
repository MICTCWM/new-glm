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
import { useTranslation } from 'react-i18next'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { useUsageLogsContext } from './usage-logs-provider'

// 自动刷新间隔可选项（毫秒）
const REFRESH_INTERVAL_OPTIONS: { value: string; label: string }[] = [
  { value: '1000', label: '1s' },
  { value: '5000', label: '5s' },
  { value: '10000', label: '10s' },
  { value: '20000', label: '20s' },
]

// 自动刷新开关 + 间隔选择器，放在使用日志 toolbar 的 preActions 位置
export function AutoRefreshControl() {
  const { t } = useTranslation()
  const {
    autoRefreshEnabled,
    setAutoRefreshEnabled,
    autoRefreshInterval,
    setAutoRefreshInterval,
  } = useUsageLogsContext()

  return (
    <div className='flex items-center gap-2'>
      <label className='flex cursor-pointer items-center gap-1.5'>
        <Switch
          checked={autoRefreshEnabled}
          onCheckedChange={setAutoRefreshEnabled}
          aria-label={t('Auto Refresh')}
        />
        <span className='text-muted-foreground text-xs whitespace-nowrap'>
          {t('Auto Refresh')}
        </span>
      </label>
      <Select
        items={REFRESH_INTERVAL_OPTIONS}
        value={String(autoRefreshInterval)}
        onValueChange={(value) => {
          if (value !== null) {
            setAutoRefreshInterval(Number(value))
          }
        }}
        disabled={!autoRefreshEnabled}
      >
        <SelectTrigger
          size='sm'
          className='w-[68px]'
          aria-label={t('Refresh Interval')}
        >
          <SelectValue placeholder={t('Refresh Interval')} />
        </SelectTrigger>
        <SelectContent alignItemWithTrigger={false}>
          <SelectGroup>
            {REFRESH_INTERVAL_OPTIONS.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>
    </div>
  )
}
