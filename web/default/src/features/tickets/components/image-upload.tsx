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
import { useState, useRef, useEffect } from 'react'
import { ImagePlus, X } from 'lucide-react'
import { ALLOWED_IMAGE_TYPES, MAX_IMAGE_SIZE, MAX_IMAGES_PER_TICKET } from '../constants'

interface ImageUploadProps {
  images: File[]
  onChange: (images: File[]) => void
  disabled?: boolean
}

export function ImageUpload({ images, onChange, disabled }: ImageUploadProps) {
  const inputRef = useRef<HTMLInputElement>(null)
  const [error, setError] = useState<string | null>(null)

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files || [])
    setError(null)

    if (images.length + files.length > MAX_IMAGES_PER_TICKET) {
      setError(`最多上传 ${MAX_IMAGES_PER_TICKET} 张图片`)
      return
    }

    for (const file of files) {
      if (!ALLOWED_IMAGE_TYPES.includes(file.type)) {
        setError('不支持的图片格式，支持：JPG、PNG、GIF、WebP')
        return
      }
      if (file.size > MAX_IMAGE_SIZE) {
        setError('图片大小不能超过 10MB')
        return
      }
    }

    onChange([...images, ...files])
    // Reset input so the same file can be selected again
    if (inputRef.current) inputRef.current.value = ''
  }

  // Track blob URLs for cleanup
  const blobUrlsRef = useRef<Set<string>>(new Set())

  // Clean up blob URLs on unmount
  useEffect(() => {
    return () => {
      blobUrlsRef.current.forEach((url) => URL.revokeObjectURL(url))
      blobUrlsRef.current.clear()
    }
  }, [])

  // Cache blob URLs to avoid re-creating on every render and enable cleanup
  const blobUrlCacheRef = useRef<Map<File, string>>(new Map())

  // Get or create a blob URL for a file
  const getBlobUrl = (file: File): string => {
    if (blobUrlCacheRef.current.has(file)) {
      return blobUrlCacheRef.current.get(file)!
    }
    const url = URL.createObjectURL(file)
    blobUrlCacheRef.current.set(file, url)
    return url
  }

  // Revoke blob URLs for removed files and clear cache on unmount
  const cleanupBlobUrls = (newFiles: File[]) => {
    const newSet = new Set(newFiles)
    blobUrlCacheRef.current.forEach((url, file) => {
      if (!newSet.has(file)) {
        URL.revokeObjectURL(url)
        blobUrlCacheRef.current.delete(file)
      }
    })
  }

  useEffect(() => {
    return () => {
      blobUrlCacheRef.current.forEach((url) => URL.revokeObjectURL(url))
      blobUrlCacheRef.current.clear()
    }
  }, [])

  const removeImage = (index: number) => {
    const newImages = images.filter((_, i) => i !== index)
    cleanupBlobUrls(newImages)
    onChange(newImages)
  }

  return (
    <div className='space-y-3'>
      <div className='flex flex-wrap gap-3'>
        {images.map((file, index) => (
          <div
            key={`${file.name}-${index}-${file.size}`}
            className='relative group h-20 w-20 rounded-lg border overflow-hidden flex-shrink-0'
          >
            <img
              src={getBlobUrl(file)}
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
            <span className='text-[10px]'>添加图片</span>
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
        支持 JPG、PNG、GIF、WebP 格式，最多 {MAX_IMAGES_PER_TICKET} 张，每张不超过 10MB。
      </p>
    </div>
  )
}
