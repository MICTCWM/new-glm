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
import type {
  SystemOptionsResponse,
  UpdateOptionResponse,
} from '../types'
import { safeJsonParse } from '../utils/json-parser'
import type { GptGroupSettings } from './types'

// 获取 GPT 渠道列表
export async function getGptChannels() {
  return api.get('/api/channel/gpt')
}

// 批量设置 GPT 渠道
export async function setGptChannels(
  channelIds: number[],
  gptModeRequired: boolean
) {
  return api.put('/api/channel/gpt', {
    channel_ids: channelIds,
    gpt_mode_required: gptModeRequired,
  })
}

// 获取所有渠道列表（复用现有 API）
export async function getAllChannels() {
  return api.get('/api/channel', { params: { p: 1, page_size: 1000 } })
}

// 获取 GPT 分组配置（从所有 option 中筛选 GptGroupRatio / GptUserUsableGroups）
export async function getGptGroupSettings(): Promise<GptGroupSettings> {
  const res = await api.get<SystemOptionsResponse>('/api/option/')
  const options = res.data?.data ?? []
  let groupRatio: Record<string, number> = {}
  let userUsableGroups: Record<string, string> = {}
  for (const option of options) {
    if (option.key === 'GptGroupRatio') {
      groupRatio = safeJsonParse<Record<string, number>>(option.value, {
        fallback: {},
        silent: true,
        context: 'GPT group ratios',
      })
    } else if (option.key === 'GptUserUsableGroups') {
      userUsableGroups = safeJsonParse<Record<string, string>>(option.value, {
        fallback: {},
        silent: true,
        context: 'GPT user usable groups',
      })
    }
  }
  return { groupRatio, userUsableGroups }
}

// 更新 GPT 分组配置（分别写入 GptGroupRatio / GptUserUsableGroups 两个 option）
export async function updateGptGroupSettings(
  groupRatio: Record<string, number>,
  userUsableGroups: Record<string, string>
): Promise<void> {
  const results = await Promise.all([
    api.put<UpdateOptionResponse>('/api/option/', {
      key: 'GptGroupRatio',
      value: JSON.stringify(groupRatio, null, 2),
    }),
    api.put<UpdateOptionResponse>('/api/option/', {
      key: 'GptUserUsableGroups',
      value: JSON.stringify(userUsableGroups, null, 2),
    }),
  ])
  // 校验两个响应的 success 字段，axios 拦截器在 success=false 时仅 toast 不会 reject
  for (const res of results) {
    if (!res.data?.success) {
      throw new Error(
        res.data?.message || 'Failed to update GPT group settings'
      )
    }
  }
}
