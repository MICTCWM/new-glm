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
import { z } from 'zod'

// ============================================================================
// Ticket Schema & Types
// ============================================================================

export const ticketImageSchema = z.object({
  id: z.number(),
  ticket_id: z.number(),
  filename: z.string(),
  file_path: z.string(),
  file_size: z.number(),
  created_at: z.number(),
})
export type TicketImage = z.infer<typeof ticketImageSchema>

export const ticketReplySchema = z.object({
  id: z.number(),
  ticket_id: z.number(),
  user_id: z.number(),
  content: z.string(),
  created_at: z.number(),
  username: z.string(),
})
export type TicketReply = z.infer<typeof ticketReplySchema>

export const ticketSchema = z.object({
  id: z.number(),
  user_id: z.number(),
  title: z.string(),
  content: z.string(),
  status: z.enum(['open', 'closed', 'replied']),
  created_at: z.number(),
  updated_at: z.number(),
  username: z.string(),
  images: z.array(ticketImageSchema).optional(),
  replies: z.array(ticketReplySchema).optional(),
})
export type Ticket = z.infer<typeof ticketSchema>

export const ticketListResponseSchema = z.object({
  tickets: z.array(ticketSchema),
  total: z.number(),
  page: z.number(),
  page_size: z.number(),
})

export interface TicketListResponse {
  tickets: Ticket[]
  total: number
  page: number
  page_size: number
}

export interface TicketFormData {
  title: string
  content: string
  images: File[]
}

export interface TicketReplyData {
  content: string
  close_on_reply?: boolean
}

export interface UploadImageResponse {
  filename: string
  filepath: string
  filesize: number
}

export type TicketStatus = 'open' | 'closed' | 'replied'
