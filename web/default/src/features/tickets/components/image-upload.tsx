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
import { useState, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { ImagePlus, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { ALLOWED_IMAGE_TYPES, MAX_IMAGE_SIZE, MAX_IMAGES_PER_TICKET } from '../constants'

interface ImageUploadProps {
  images: File[]
  onChange: (images: File[]) => void
  disabled?: boolean
}

export function ImageUpload({ images, onChange, disabled }: ImageUploadProps) {
  const { t } = useTranslation()
  const inputRef = useRef<HTMLInputElement>(null)
  const [error, setError] = useState<string | null>(null)

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files || [])
    setError(null)

    if (images.length + files.length > MAX_IMAGES_PER_TICKET) {
      setError(t('Maximum {{count}} images allowed', { count: MAX_IMAGES_PER_TICKET }))
      return
    }

    for (const file of files) {
      if (!ALLOWED_IMAGE_TYPES.includes(file.type)) {
        setError(t('Unsupported image format. Supported: JPG, PNG, GIF, WebP'))
        return
      }
      if (file.size > MAX_IMAGE_SIZE) {
        setError(t('Image size cannot exceed 10MB'))
        return
      }
    }

    onChange([...images, ...files])
    // Reset input so the same file can be selected again
    if (inputRef.current) inputRef.current.value = ''
  }

  const removeImage = (index: number) => {
    const newImages = images.filter((_, i) => i !== index)
    onChange(newImages)
  }

  return (
    <div className='space-y-3'>
      <div className='flex flex-wrap gap-3'>
        {images.map((file, index) => (
          <div
            key={`${file.name}-${index}`}
            className='relative group h-20 w-20 rounded-lg border overflow-hidden flex-shrink-0'
          >
            <img
              src={URL.createObjectURL(file)}
              alt={file.name}
              className='h-full w-full object-cover'
            />
            <button
              type='button'
              onClick={() => removeImage(index)}
              disabled={disabled}
              className='absolute top-0.5 right-0.5 rounded-full bg-destructive text-destructive-foreground p-0.5 opacity-0 group-hover:opacity-100 transition-opacity'
            >
              <X className='h-3 w-3' />
            </button>
          </div>
        ))}
        {images.length < MAX_IMAGES_PER_TICKET && (
          <button
            type='button'
            onClick={() => inputRef.current?.click()}
            disabled={disabled}
            className='flex h-20 w-20 flex-col items-center justify-center gap-1 rounded-lg border border-dashed text-muted-foreground hover:border-primary hover:text-primary transition-colors disabled:opacity-50 disabled:cursor-not-allowed'
          >
            <ImagePlus className='h-5 w-5' />
            <span className='text-[10px]'>{t('Add Image')}</span>
          </button>
        )}
      </div>
      <input
        ref={inputRef}
        type='file'
        accept={ALLOWED_IMAGE_TYPES.join(',')}
        multiple
        onChange={handleFileSelect}
        className='hidden'
      />
      {error && <p className='text-sm text-destructive'>{error}</p>}
      <p className='text-xs text-muted-foreground'>
        {t('Supports JPG, PNG, GIF, WebP. Maximum {{count}} images, each up to 10MB.', {
          count: MAX_IMAGES_PER_TICKET,
        })}
      </p>
    </div>
  )
}
