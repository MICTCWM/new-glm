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
import { useState, useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from '@tanstack/react-router'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { getStatus } from '@/lib/api'

export function NotInServiceAreaError() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [isRetrying, setIsRetrying] = useState(false)
  const isMountedRef = useRef(true)

  useEffect(() => {
    return () => {
      isMountedRef.current = false
    }
  }, [])

  const handleRetry = async () => {
    setIsRetrying(true)
    try {
      const status = await getStatus()
      
      // 验证响应有效性
      if (!isMountedRef.current) return
      
      if (!status || typeof status.region_blocked !== 'boolean') {
        toast.error(t('Invalid response from server, please try again'))
        return
      }

      if (status.region_blocked) {
        toast.error(
          t('Still not in service area. Please ensure you are connected from a supported region.'),
          { duration: 5000 }
        )
      } else {
        toast.success(t('Access granted, redirecting...'))
        setTimeout(() => {
          if (isMountedRef.current) {
            navigate({ to: '/' })
          }
        }, 500)
      }
    } catch (error) {
      if (!isMountedRef.current) return
      
      console.error('Status check failed:', error)
      const errorMessage = error instanceof Error 
        ? error.message 
        : t('Network error, please check your connection and try again')
      toast.error(errorMessage)
    } finally {
      if (isMountedRef.current) {
        setIsRetrying(false)
      }
    }
  }
  return (
    <div className='h-svh'>
      <div className='m-auto flex h-full w-full flex-col items-center justify-center gap-2'>
        <h1 className='text-[7rem] leading-tight font-bold'>451</h1>
        <span className='font-medium'>
          {t('Not In Service Area')}
        </span>
        <p className='text-muted-foreground text-center'>
          {t('Sorry, this service is not available in your region.')} <br />
          {t('If you believe this is a mistake, please contact the administrator.')}
        </p>
        <div className='mt-6 flex gap-4'>
          <Button 
            variant='outline' 
            onClick={handleRetry}
            disabled={isRetrying}
          >
            {isRetrying ? t('Checking...') : t('Retry')}
          </Button>
        </div>
      </div>
    </div>
  )
}
