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
import { useState, useEffect, useCallback } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { RefreshCw, ChevronLeft, ChevronRight, Clock, User, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { NativeSelect } from '@/components/ui/native-select'
import { Empty } from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { TICKET_STATUS_MAP, TICKET_STATUS_FILTER_OPTIONS } from '../constants'
import { getUserTickets, getAllTickets, deleteTicket } from '../api'
import type { Ticket, TicketStatus } from '../types'

interface TicketListProps {
  isAdmin?: boolean
}

export function TicketList({ isAdmin = false }: TicketListProps) {
  const navigate = useNavigate()
  const [tickets, setTickets] = useState<Ticket[]>([])
  const [loading, setLoading] = useState(true)
  const [statusFilter, setStatusFilter] = useState('')
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [deleteTarget, setDeleteTarget] = useState<Ticket | null>(null)
  const [deleting, setDeleting] = useState(false)
  const pageSize = 20

  const fetchTickets = useCallback(async () => {
    setLoading(true)
    try {
      const params = { page, page_size: pageSize, status: statusFilter || undefined }
      const res = isAdmin ? await getAllTickets(params) : await getUserTickets(params)
      if (res.success && res.data) {
        setTickets(res.data.tickets)
        setTotal(res.data.total)
      }
    } finally {
      setLoading(false)
    }
  }, [statusFilter, page, isAdmin])

  useEffect(() => {
    fetchTickets()
  }, [fetchTickets])

  const handleDelete = async () => {
    if (!deleteTarget) return
    setDeleting(true)
    try {
      const res = await deleteTicket(deleteTarget.id)
      if (res.success) {
        toast.success('工单已删除')
        setDeleteTarget(null)
        fetchTickets()
      }
    } finally {
      setDeleting(false)
    }
  }

  const totalPages = Math.ceil(total / pageSize)

  const formatDate = (timestamp: number) => {
    return new Date(timestamp * 1000).toLocaleString()
  }

  const getStatusBadge = (status: string) => {
    const config = TICKET_STATUS_MAP[status as TicketStatus]
    if (!config) return <Badge variant='outline'>{status}</Badge>
    const variant = config.variant as 'default' | 'success' | 'warning' | 'destructive' | 'outline'
    return <Badge variant={variant}>{config.label}</Badge>
  }

  return (
    <>
    <Card>
      <CardHeader className='flex flex-row items-center justify-between'>
        <CardTitle>{isAdmin ? '所有工单' : '我的工单'}</CardTitle>
        <div className='flex items-center gap-2'>
          <NativeSelect
            value={statusFilter}
            onChange={(e) => {
              setStatusFilter(e.target.value)
              setPage(1)
            }}
            className='w-32'
          >
            {TICKET_STATUS_FILTER_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </NativeSelect>
          <Button variant='outline' size='icon' onClick={fetchTickets} title='刷新'>
            <RefreshCw className='h-4 w-4' />
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        {loading ? (
          <div className='space-y-3'>
            {[1, 2, 3, 4, 5].map((i) => (
              <Skeleton key={i} className='h-20 w-full rounded-lg' />
            ))}
          </div>
        ) : tickets.length === 0 ? (
          <Empty description='暂无工单' />
        ) : (
          <>
            <div className='space-y-2'>
              {tickets.map((ticket) => (
                <div
                  key={ticket.id}
                  onClick={() =>
                    navigate({
                      to: '/tickets/$ticketId',
                      params: { ticketId: String(ticket.id) },
                    })
                  }
                  className='flex flex-col sm:flex-row sm:items-center justify-between gap-2 rounded-lg border p-4 cursor-pointer hover:bg-muted/50 transition-colors'
                >
                  <div className='flex-1 min-w-0'>
                    <div className='flex items-center gap-2 mb-1'>
                      <span className='text-xs text-muted-foreground font-mono'>
                        #{ticket.id}
                      </span>
                      <h3 className='font-medium text-sm truncate'>{ticket.title}</h3>
                    </div>
                    <div className='flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground'>
                      <span className='flex items-center gap-1'>
                        <User className='h-3 w-3' />
                        {ticket.username}
                      </span>
                      <span className='flex items-center gap-1'>
                        <Clock className='h-3 w-3' />
                        {formatDate(ticket.created_at)}
                      </span>
                      {ticket.replies && ticket.replies.length > 0 && (
                        <span>
                          {ticket.replies.length} 条回复
                        </span>
                      )}
                    </div>
                  </div>
                  <div className='flex-shrink-0 flex items-center gap-2'>
                    {getStatusBadge(ticket.status)}
                    {isAdmin && (
                      <Button
                        variant='destructive'
                        size='icon'
                        className='h-7 w-7'
                        onClick={(e) => {
                          e.stopPropagation()
                          setDeleteTarget(ticket)
                        }}
                      >
                        <Trash2 className='h-3.5 w-3.5' />
                      </Button>
                    )}
                  </div>
                </div>
              ))}
            </div>

            {totalPages > 1 && (
              <div className='flex items-center justify-between mt-4 pt-4 border-t'>
                <span className='text-sm text-muted-foreground'>
                  共 {total} 条工单
                </span>
                <div className='flex items-center gap-2'>
                  <Button
                    variant='outline'
                    size='sm'
                    onClick={() => setPage((p) => Math.max(1, p - 1))}
                    disabled={page <= 1}
                  >
                    <ChevronLeft className='h-4 w-4' />
                  </Button>
                  <span className='text-sm'>
                    {page} / {totalPages}
                  </span>
                  <Button
                    variant='outline'
                    size='sm'
                    onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                    disabled={page >= totalPages}
                  >
                    <ChevronRight className='h-4 w-4' />
                  </Button>
                </div>
              </div>
            )}
          </>
        )}
      </CardContent>
    </Card>

    <ConfirmDialog
      destructive
      open={deleteTarget !== null}
      onOpenChange={(open) => {
        if (!open) setDeleteTarget(null)
      }}
      handleConfirm={handleDelete}
      isLoading={deleting}
      className='max-w-md'
      title='删除工单？'
      desc={
        deleteTarget ? (
          <>
            确定要删除工单 <strong>#{deleteTarget.id}</strong> 吗？
            <br />
            删除后将同时移除该工单的所有回复和附件图片，此操作无法撤销。
          </>
        ) : (
          ''
        )
      }
      confirmText='删除'
    />
    </>
  )
}
