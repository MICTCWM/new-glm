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
import { api } from '@/lib/api'
import type { ModelMonitorSample, PricingData } from './types'

// ----------------------------------------------------------------------------
// Pricing APIs
// ----------------------------------------------------------------------------

// Get model pricing data
export async function getPricing(): Promise<PricingData> {
  const res = await api.get('/api/pricing')
  return res.data
}

// ----------------------------------------------------------------------------
// Model Monitor APIs
// ----------------------------------------------------------------------------

// Get monitor samples for a single model (last ~30 minutes, ascending by time)
export async function getModelMonitorSamples(
  modelName: string
): Promise<ModelMonitorSample[]> {
  const res = await api.get(
    `/api/model-monitor/samples?model=${encodeURIComponent(modelName)}`
  )
  return res.data.data
}

// Get monitor samples for all models in one shot (used by the card grid)
export async function getAllModelMonitorSamples(): Promise<
  Record<string, ModelMonitorSample[]>
> {
  const res = await api.get('/api/model-monitor/samples/all')
  return res.data.data
}
