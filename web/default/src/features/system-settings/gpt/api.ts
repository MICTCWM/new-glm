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
