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
  const [title, setTitle] = useState('')
  const [content, setContent] = useState('')
  const [images, setImages] = useState<File[]>([])
  const [submitting, setSubmitting] = useState(false)
  const [errors, setErrors] = useState<{ title?: string; content?: string }>({})

  const validate = (): boolean => {
    const newErrors: { title?: string; content?: string } = {}
    if (!title.trim()) {
      newErrors.title = '请输入标题'
    }
    if (!content.trim()) {
      newErrors.content = '请输入内容'
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
        toast.success('工单提交成功！')
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
        <CardTitle>提交工单</CardTitle>
        <CardDescription>
          请详细描述您遇到的问题或疑问，我们会尽快回复您。
        </CardDescription>
      </CardHeader>
      <CardContent
        className='space-y-4'
        onKeyDown={(e) => {
          if (e.key === 'Enter' && !e.shiftKey && e.target instanceof HTMLInputElement) {
            e.preventDefault()
            handleSubmit()
          }
        }}
      >
        <div className='space-y-2'>
          <Label htmlFor='ticket-title'>标题 *</Label>
          <Input
            id='ticket-title'
            maxLength={255}
            placeholder='请输入工单标题'
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
          <Label htmlFor='ticket-content'>描述 *</Label>
          <Textarea
            id='ticket-content'
            placeholder='请详细描述您遇到的问题...'
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
          <Label>附件（可选）</Label>
          <ImageUpload images={images} onChange={setImages} disabled={submitting} />
        </div>

        <Button onClick={handleSubmit} disabled={submitting} className='w-full sm:w-auto'>
          {submitting ? (
            <span className='flex items-center gap-2'>
              <span className='animate-spin'>⏳</span> 提交中...
            </span>
          ) : (
            <span className='flex items-center gap-2'>
              <Send className='h-4 w-4' /> 提交工单
            </span>
          )}
        </Button>
      </CardContent>
    </Card>
  )
}
