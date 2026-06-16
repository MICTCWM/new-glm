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
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { ArrowLeft, Clock, User, CheckCircle, Send, Paperclip } from 'lucide-react'
import { useNavigate } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Textarea } from '@/components/ui/textarea'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Empty } from '@/components/ui/empty'
import { Separator } from '@/components/ui/separator'
import { TICKET_STATUS_MAP } from '../constants'
import { getTicket, addTicketReply, updateTicketStatus } from '../api'
import { useAuthStore } from '@/stores/auth-store'
import type { Ticket, TicketStatus } from '../types'

interface TicketDetailProps {
  ticketId: number
}

export function TicketDetail({ ticketId }: TicketDetailProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { auth } = useAuthStore()
  const [ticket, setTicket] = useState<Ticket | null>(null)
  const [loading, setLoading] = useState(true)
  const [replyContent, setReplyContent] = useState('')
  const [closeOnReply, setCloseOnReply] = useState(false)
  const [submitting, setSubmitting] = useState(false)

  const isAdmin = auth.user && auth.user.role >= 10

  const fetchDetail = useCallback(async () => {
    setLoading(true)
    try {
      const res = await getTicket(ticketId)
      if (res.success && res.data) {
        setTicket(res.data)
      }
    } finally {
      setLoading(false)
    }
  }, [ticketId])

  useEffect(() => {
    fetchDetail()
  }, [fetchDetail])

  const handleReply = async () => {
    if (!replyContent.trim()) return
    setSubmitting(true)
    try {
      const res = await addTicketReply(ticketId, {
        content: replyContent.trim(),
        close_on_reply: closeOnReply,
      })
      if (res.success) {
        toast.success(t('Reply sent successfully!'))
        setReplyContent('')
        setCloseOnReply(false)
        fetchDetail()
      }
    } finally {
      setSubmitting(false)
    }
  }

  const handleMarkClosed = async () => {
    try {
      const res = await updateTicketStatus(ticketId, 'closed')
      if (res.success) {
        toast.success(t('Ticket marked as completed'))
        fetchDetail()
      }
    } catch {
      /* error handled by interceptor */
    }
  }

  const handleMarkOpen = async () => {
    try {
      const res = await updateTicketStatus(ticketId, 'open')
      if (res.success) {
        toast.success(t('Ticket reopened'))
        fetchDetail()
      }
    } catch {
      /* error handled by interceptor */
    }
  }

  const formatDate = (timestamp: number) => {
    return new Date(timestamp * 1000).toLocaleString()
  }

  const getStatusBadge = (status: string) => {
    const config = TICKET_STATUS_MAP[status as TicketStatus]
    if (!config) return <Badge variant='outline'>{status}</Badge>
    const variant = config.variant as 'default' | 'success' | 'warning' | 'destructive' | 'outline'
    return <Badge variant={variant}>{t(config.label)}</Badge>
  }

  if (loading) {
    return (
      <div className='space-y-4'>
        <Skeleton className='h-8 w-48' />
        <Skeleton className='h-40 w-full rounded-lg' />
        <Skeleton className='h-32 w-full rounded-lg' />
      </div>
    )
  }

  if (!ticket) {
    return <Empty description={t('Ticket not found')} />
  }

  return (
    <div className='space-y-4'>
      {/* Back button */}
      <Button variant='ghost' onClick={() => navigate({ to: '/tickets' })} className='-ml-2'>
        <ArrowLeft className='h-4 w-4 mr-2' />
        {t('Back to Tickets')}
      </Button>

      {/* Ticket header */}
      <Card>
        <CardHeader>
          <div className='flex flex-col sm:flex-row sm:items-center justify-between gap-2'>
            <div>
              <CardTitle className='text-xl flex items-center gap-2'>
                <span className='text-muted-foreground font-mono text-base'>#{ticket.id}</span>
                {ticket.title}
              </CardTitle>
              <CardDescription className='flex flex-wrap items-center gap-x-4 gap-y-1 mt-1'>
                <span className='flex items-center gap-1'>
                  <User className='h-3.5 w-3.5' />
                  {ticket.username}
                </span>
                <span className='flex items-center gap-1'>
                  <Clock className='h-3.5 w-3.5' />
                  {formatDate(ticket.created_at)}
                </span>
              </CardDescription>
            </div>
            <div className='flex items-center gap-2'>
              {getStatusBadge(ticket.status)}
              {isAdmin && ticket.status !== 'closed' && (
                <Button
                  variant='outline'
                  size='sm'
                  onClick={handleMarkClosed}
                >
                  <CheckCircle className='h-4 w-4 mr-1' />
                  {t('Mark Completed')}
                </Button>
              )}
              {isAdmin && ticket.status === 'closed' && (
                <Button
                  variant='outline'
                  size='sm'
                  onClick={handleMarkOpen}
                >
                  {t('Reopen')}
                </Button>
              )}
            </div>
          </div>
        </CardHeader>
        <CardContent className='space-y-4'>
          {/* Content */}
          <div className='prose prose-sm dark:prose-invert max-w-none'>
            <p className='whitespace-pre-wrap'>{ticket.content}</p>
          </div>

          {/* Images */}
          {ticket.images && ticket.images.length > 0 && (
            <div>
              <Separator className='my-3' />
              <div className='flex items-center gap-2 mb-2 text-sm font-medium'>
                <Paperclip className='h-4 w-4' />
                {t('Attachments')} ({ticket.images.length})
              </div>
              <div className='flex flex-wrap gap-2'>
                {ticket.images.map((img) => (
                  <a
                    key={img.id}
                    href={img.file_path}
                    target='_blank'
                    rel='noopener noreferrer'
                    className='block h-24 w-24 rounded-lg border overflow-hidden hover:ring-2 ring-primary transition-all'
                  >
                    <img
                      src={img.file_path}
                      alt={img.filename}
                      className='h-full w-full object-cover'
                    />
                  </a>
                ))}
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Replies */}
      {ticket.replies && ticket.replies.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className='text-base'>{t('Replies')}</CardTitle>
          </CardHeader>
          <CardContent className='space-y-4'>
            {ticket.replies.map((reply) => (
              <div key={reply.id} className='rounded-lg bg-muted/50 p-4'>
                <div className='flex items-center justify-between mb-2'>
                  <div className='flex items-center gap-3 text-sm'>
                    <span className='font-medium'>{reply.username}</span>
                    <span className='text-muted-foreground text-xs'>
                      {formatDate(reply.created_at)}
                    </span>
                  </div>
                </div>
                <p className='text-sm whitespace-pre-wrap'>{reply.content}</p>
              </div>
            ))}
          </CardContent>
        </Card>
      )}

      {/* Admin reply form */}
      {isAdmin && (
        <Card>
          <CardHeader>
            <CardTitle className='text-base'>{t('Reply to Ticket')}</CardTitle>
          </CardHeader>
          <CardContent className='space-y-3'>
            <Textarea
              placeholder={t('Enter your reply...')}
              value={replyContent}
              onChange={(e) => setReplyContent(e.target.value)}
              rows={4}
              disabled={submitting}
            />
            <div className='flex items-center gap-2'>
              <Checkbox
                id='close-on-reply'
                checked={closeOnReply}
                onCheckedChange={(checked) => setCloseOnReply(!!checked)}
                disabled={submitting}
              />
              <Label htmlFor='close-on-reply' className='text-sm cursor-pointer'>
                {t('Mark as completed after replying')}
              </Label>
            </div>
            <Button onClick={handleReply} disabled={submitting || !replyContent.trim()}>
              {submitting ? (
                <span className='flex items-center gap-2'>
                  <span className='animate-spin'>⏳</span> {t('Sending...')}
                </span>
              ) : (
                <span className='flex items-center gap-2'>
                  <Send className='h-4 w-4' /> {t('Send Reply')}
                </span>
              )}
            </Button>
          </CardContent>
        </Card>
      )}

      {/* Non-admin user: show message if no replies */}
      {!isAdmin && (!ticket.replies || ticket.replies.length === 0) && (
        <Card>
          <CardContent className='py-8 text-center text-muted-foreground'>
            <p>{t('No replies yet. An administrator will respond to your ticket soon.')}</p>
          </CardContent>
        </Card>
      )}
    </div>
  )
}
