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
import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { getPerfMetricsSummary } from '@/features/performance-metrics/api'
import { DEFAULT_PRICING_PAGE_SIZE, DEFAULT_TOKEN_UNIT } from '../constants'
import { getAllModelMonitorSamples } from '../api'
import { isGptGroupModel } from '../lib/filters'
import type { ModelMonitorSample, PricingModel, TokenUnit } from '../types'
import { ModelCard } from './model-card'
import type { ModelPerfBadgeData } from './model-perf-badge'

export interface ModelCardGridProps {
  models: PricingModel[]
  onModelClick: (modelName: string) => void
  priceRate?: number
  usdExchangeRate?: number
  tokenUnit?: TokenUnit
  showRechargePrice?: boolean
  gptGroups?: string[]
}

export function ModelCardGrid(props: ModelCardGridProps) {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const pageSize = DEFAULT_PRICING_PAGE_SIZE
  const tokenUnit = props.tokenUnit ?? DEFAULT_TOKEN_UNIT
  const totalPages = Math.max(1, Math.ceil(props.models.length / pageSize))
  const currentPage = Math.min(page, totalPages)
  const gptGroups = props.gptGroups ?? []

  const perfQuery = useQuery({
    queryKey: ['perf-metrics-summary', 24],
    queryFn: () => getPerfMetricsSummary(24),
    staleTime: 60 * 1000,
    retry: false,
  })

  const monitorQuery = useQuery({
    queryKey: ['model-monitor-samples-all'],
    queryFn: getAllModelMonitorSamples,
    staleTime: 60 * 1000,
    refetchInterval: 60 * 1000,
    retry: false,
  })

  const pagedModels = useMemo(() => {
    const start = (currentPage - 1) * pageSize
    return props.models.slice(start, start + pageSize)
  }, [currentPage, pageSize, props.models])

  // 把当前页的模型分成普通模型组与 GPT 分组模型组，
  // 以便在两组之间渲染一条带文字的分隔线。
  const { normalModels, gptModels } = useMemo(() => {
    const normal: PricingModel[] = []
    const gpt: PricingModel[] = []
    for (const model of pagedModels) {
      if (isGptGroupModel(model, gptGroups)) {
        gpt.push(model)
      } else {
        normal.push(model)
      }
    }
    return { normalModels: normal, gptModels: gpt }
  }, [pagedModels, gptGroups])

  const perfMap = useMemo(() => {
    const map = new Map<string, ModelPerfBadgeData>()
    for (const model of perfQuery.data?.data?.models ?? []) {
      map.set(model.model_name, model)
    }
    return map
  }, [perfQuery.data])

  const monitorMap = useMemo(() => {
    const map = new Map<string, ModelMonitorSample[]>()
    const raw = monitorQuery.data
    if (!raw) return map
    for (const [modelName, samples] of Object.entries(raw)) {
      map.set(modelName, samples)
    }
    return map
  }, [monitorQuery.data])

  if (props.models.length === 0) {
    return null
  }

  const renderCard = (model: PricingModel) => (
    <ModelCard
      key={model.id ?? model.model_name}
      model={model}
      tokenUnit={tokenUnit}
      priceRate={props.priceRate}
      usdExchangeRate={props.usdExchangeRate}
      showRechargePrice={props.showRechargePrice}
      perf={perfMap.get(model.model_name || '')}
      monitorSamples={monitorMap.get(model.model_name || '')}
      onClick={() => props.onModelClick(model.model_name || '')}
    />
  )

  return (
    <div className='space-y-4 sm:space-y-5'>
      <div className='grid grid-cols-1 gap-3 sm:gap-4 md:grid-cols-2 lg:grid-cols-3'>
        {normalModels.map(renderCard)}
        {normalModels.length > 0 && gptModels.length > 0 && (
          <div className='col-span-full my-6 flex items-center gap-3'>
            <div className='h-px flex-1 bg-border' />
            <span className='text-muted-foreground text-sm font-medium whitespace-nowrap'>
              {t('GPT Dedicated Groups')}
            </span>
            <div className='h-px flex-1 bg-border' />
          </div>
        )}
        {gptModels.map(renderCard)}
      </div>

      {totalPages > 1 && (
        <div className='text-muted-foreground flex flex-col items-center justify-between gap-3 border-t px-4 py-3 text-sm sm:flex-row'>
          <p className='text-muted-foreground'>
            {t('Page {{current}} of {{total}}', {
              current: currentPage,
              total: totalPages,
            })}
          </p>
          <div className='flex items-center gap-2'>
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={() => setPage((current) => Math.max(1, current - 1))}
              disabled={currentPage <= 1}
              className='gap-1.5'
            >
              <ChevronLeft className='size-4' />
              {t('Previous')}
            </Button>
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={() =>
                setPage((current) => Math.min(totalPages, current + 1))
              }
              disabled={currentPage >= totalPages}
              className='gap-1.5'
            >
              {t('Next')}
              <ChevronRight className='size-4' />
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
