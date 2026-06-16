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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Send } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { ImageUpload } from './image-upload'
import { createTicket } from '../api'

interface TicketFormProps {
  onSuccess?: () => void
}

export function TicketForm({ onSuccess }: TicketFormProps) {
  const { t } = useTranslation()
  const [title, setTitle] = useState('')
  const [content, setContent] = useState('')
  const [images, setImages] = useState<File[]>([])
  const [submitting, setSubmitting] = useState(false)
  const [errors, setErrors] = useState<{ title?: string; content?: string }>({})

  const validate = (): boolean => {
    const newErrors: { title?: string; content?: string } = {}
    if (!title.trim()) {
      newErrors.title = t('Title is required')
    }
    if (!content.trim()) {
      newErrors.content = t('Content is required')
    }
    setErrors(newErrors)
    return Object.keys(newErrors).length === 0
  }

  const handleSubmit = async () => {
    if (!validate()) return

    setSubmitting(true)
    try {
      const res = await createTicket({
        title: title.trim(),
        content: content.trim(),
        images: images.length > 0 ? images : undefined,
      })
      if (res.success) {
        toast.success(t('Ticket submitted successfully!'))
        setTitle('')
        setContent('')
        setImages([])
        setErrors({})
        onSuccess?.()
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Submit a Ticket')}</CardTitle>
        <CardDescription>
          {t('Describe your issue or question in detail. We will get back to you as soon as possible.')}
        </CardDescription>
      </CardHeader>
      <CardContent className='space-y-4'>
        <div className='space-y-2'>
          <Label htmlFor='ticket-title'>{t('Title')} *</Label>
          <Input
            id='ticket-title'
            placeholder={t('Enter a brief title for your ticket')}
            value={title}
            onChange={(e) => {
              setTitle(e.target.value)
              if (errors.title) setErrors((prev) => ({ ...prev, title: undefined }))
            }}
            disabled={submitting}
          />
          {errors.title && <p className='text-sm text-destructive'>{errors.title}</p>}
        </div>

        <div className='space-y-2'>
          <Label htmlFor='ticket-content'>{t('Description')} *</Label>
          <Textarea
            id='ticket-content'
            placeholder={t('Describe your issue in detail...')}
            value={content}
            onChange={(e) => {
              setContent(e.target.value)
              if (errors.content) setErrors((prev) => ({ ...prev, content: undefined }))
            }}
            rows={6}
            disabled={submitting}
          />
          {errors.content && <p className='text-sm text-destructive'>{errors.content}</p>}
        </div>

        <div className='space-y-2'>
          <Label>{t('Attachments (optional)')}</Label>
          <ImageUpload images={images} onChange={setImages} disabled={submitting} />
        </div>

        <Button onClick={handleSubmit} disabled={submitting} className='w-full sm:w-auto'>
          {submitting ? (
            <span className='flex items-center gap-2'>
              <span className='animate-spin'>⏳</span> {t('Submitting...')}
            </span>
          ) : (
            <span className='flex items-center gap-2'>
              <Send className='h-4 w-4' /> {t('Submit Ticket')}
            </span>
          )}
        </Button>
      </CardContent>
    </Card>
  )
}
