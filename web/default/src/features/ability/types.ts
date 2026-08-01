/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Software License as published by
the Free Software Foundation, either version 3 of the License, or (at your
option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Software License for more details.

You should have received a copy of the GNU Affero General Software License
along with this program. If not, see <https://www.gnu.org/licenses/>.
*/

export interface Ability {
  id: number
  group: string
  model: string
  channel_id: number
  channel_type: number
  channel_name: string
  channel_setting?: string | null
  priority: number
  weight: number
  enabled: boolean
  tag: string
  created_time: number
  updated_time: number
}

export interface GetAbilityParams {
  p?: number
  page_size?: number
  keyword?: string
  group?: string
  model?: string
  channel_id?: number
  only_enabled?: boolean
}

export interface GetAbilityResponse {
  success: boolean
  message?: string
  data?: {
    items: Ability[]
    total: number
    page: number
    page_size: number
  }
}
