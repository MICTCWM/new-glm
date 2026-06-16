import { useQuery } from '@tanstack/react-query'
import { Clock } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { api } from '@/lib/api'
import { Skeleton } from '@/components/ui/skeleton'

async function fetchQueueStatus(): Promise<{ success: boolean; queue_count: number }> {
  const res = await api.get('/dashboard/queue-status')
  return res.data
}

export function QueueStatusCard() {
  const { t } = useTranslation()

  const query = useQuery({
    queryKey: ['queue-status'],
    queryFn: fetchQueueStatus,
    refetchInterval: 5000, // poll every 5s
    staleTime: 4000,
    retry: false,
  })

  const queueCount = query.data?.queue_count ?? 0
  const loading = query.isLoading
  const hasQueue = queueCount > 0

  return (
    <section className='bg-card h-full overflow-hidden rounded-2xl border shadow-xs'>
      <div className='flex items-center gap-2 border-b px-4 py-3 sm:px-5'>
        <Clock className='text-muted-foreground/60 size-4 shrink-0' aria-hidden='true' />
        <h3 className='text-sm font-semibold'>{t('Queued requests')}</h3>
        <span className='text-muted-foreground ml-auto text-xs'>
          {t('RPM queue')}
        </span>
      </div>

      <div className='flex items-center justify-center p-6'>
        {loading ? (
          <Skeleton className='h-16 w-32' />
        ) : (
          <div className='flex flex-col items-center gap-2'>
            <span
              className={cn(
                'font-mono text-4xl font-bold tabular-nums transition-colors',
                hasQueue
                  ? 'text-warning animate-pulse'
                  : 'text-muted-foreground'
              )}
            >
              {queueCount}
            </span>
            <span className='text-muted-foreground text-xs'>
              {hasQueue
                ? t('Requests waiting in queue')
                : t('No queued requests')}
            </span>
          </div>
        )}
      </div>
    </section>
  )
}
