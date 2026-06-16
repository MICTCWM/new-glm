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
import type { Ticket, TicketListResponse, UploadImageResponse } from './types'

// ============================================================================
// Ticket APIs
// ============================================================================

interface GetTicketsParams {
  status?: string
  page?: number
  page_size?: number
}

/**
 * Get current user's tickets
 */
export async function getUserTickets(
  params: GetTicketsParams = {}
): Promise<{ success: boolean; message?: string; data?: TicketListResponse }> {
  const { status = '', page = 1, page_size = 20 } = params
  let url = `/api/ticket/self?page=${page}&page_size=${page_size}`
  if (status) url += `&status=${status}`
  const res = await api.get(url)
  return res.data
}

/**
 * Get all tickets (admin only)
 */
export async function getAllTickets(
  params: GetTicketsParams = {}
): Promise<{ success: boolean; message?: string; data?: TicketListResponse }> {
  const { status = '', page = 1, page_size = 20 } = params
  let url = `/api/ticket?page=${page}&page_size=${page_size}`
  if (status) url += `&status=${status}`
  const res = await api.get(url)
  return res.data
}

/**
 * Get ticket detail
 */
export async function getTicket(
  id: number
): Promise<{ success: boolean; message?: string; data?: Ticket }> {
  const res = await api.get(`/api/ticket/${id}`)
  return res.data
}

/**
 * Create a new ticket with optional images
 */
export async function createTicket(data: {
  title: string
  content: string
  images?: File[]
}): Promise<{ success: boolean; message?: string; data?: Ticket }> {
  const formData = new FormData()
  formData.append('title', data.title)
  formData.append('content', data.content)
  if (data.images) {
    data.images.forEach((file) => {
      formData.append('images', file)
    })
  }
  const res = await api.post('/api/ticket', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return res.data
}

/**
 * Add reply to a ticket (admin only)
 */
export async function addTicketReply(
  ticketId: number,
  data: { content: string; close_on_reply?: boolean }
): Promise<{ success: boolean; message?: string }> {
  const res = await api.post(`/api/ticket/${ticketId}/reply`, data)
  return res.data
}

/**
 * Update ticket status (admin only)
 */
export async function updateTicketStatus(
  ticketId: number,
  status: string
): Promise<{ success: boolean; message?: string }> {
  const res = await api.put(`/api/ticket/${ticketId}/status`, { status })
  return res.data
}

/**
 * Upload a single image
 */
export async function uploadTicketImage(
  file: File
): Promise<{ success: boolean; message?: string; data?: UploadImageResponse }> {
  const formData = new FormData()
  formData.append('file', file)
  const res = await api.post('/api/ticket/upload', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return res.data
}
