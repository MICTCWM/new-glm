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
import type { TicketStatus } from './types'

export const TICKET_STATUS_MAP: Record<
  TicketStatus,
  { label: string; variant: 'default' | 'success' | 'warning' | 'destructive' | 'outline' }
> = {
  open: { label: '待处理', variant: 'warning' },
  closed: { label: '已完成', variant: 'success' },
  replied: { label: '已回复', variant: 'default' },
}

export const TICKET_STATUS_FILTER_OPTIONS = [
  { value: '', label: '全部' },
  { value: 'open', label: '待处理' },
  { value: 'replied', label: '已回复' },
  { value: 'closed', label: '已完成' },
] as const

export const MAX_IMAGE_SIZE = 10 * 1024 * 1024 // 10MB
export const MAX_IMAGES_PER_TICKET = 5
export const ALLOWED_IMAGE_TYPES = ['image/jpeg', 'image/png', 'image/gif', 'image/webp']
