import { useEffect, useState } from 'react'
import { Clock, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { api } from '@/lib/api'
import {
  SidebarGroup,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuItem,
} from '@/components/ui/sidebar'

type QueueItem = {
  request_id?: string
  username?: string
  user_id?: number
  group?: string
  model_name?: string
  prompt_tokens?: number
  enqueue_time?: number
  wait_seconds?: number
}

type QueueStatusResponse = {
  success: boolean
  queue_count: number
  queue_items?: QueueItem[]
}

function formatTokens(tokens?: number) {
  if (!tokens || tokens <= 0) return '0'
  if (tokens >= 1000) return `${(tokens / 1000).toFixed(1)}k`
  return String(tokens)
}

export function RpmQueueStatus() {
  const { t } = useTranslation()
  const [items, setItems] = useState<QueueItem[]>([])

  useEffect(() => {
    let mounted = true

    const load = async () => {
      try {
        const res = await api.get<QueueStatusResponse>(
          '/dashboard/queue-status',
          {
            disableDuplicate: true,
            skipBusinessError: true,
          } as Record<string, unknown>
        )
        if (!mounted) return
        setItems(res.data.queue_items || [])
      } catch {
        if (mounted) setItems([])
      }
    }

    load()
    const timer = window.setInterval(load, 5000)
    return () => {
      mounted = false
      window.clearInterval(timer)
    }
  }, [])

  if (items.length === 0) return null

  return (
    <SidebarGroup className='px-2 py-1'>
      <SidebarGroupLabel className='text-muted-foreground/70 flex items-center gap-1.5 px-2 text-[11px] font-medium tracking-wider uppercase'>
        <Loader2 className='size-3 animate-spin' />
        {t('Queued Requests')}
      </SidebarGroupLabel>
      <SidebarMenu>
        {items.slice(0, 5).map((item, index) => (
          <SidebarMenuItem key={item.request_id || index}>
            <div className='mx-1 rounded-md border border-border/70 px-2 py-1.5 text-xs'>
              <div className='flex items-center justify-between gap-2'>
                <span className='truncate font-medium'>
                  {item.username || `#${item.user_id || '-'}`}
                </span>
                <span className='text-muted-foreground flex shrink-0 items-center gap-1'>
                  <Clock className='size-3' />
                  {item.wait_seconds || 0}s
                </span>
              </div>
              <div className='text-muted-foreground mt-1 truncate'>
                {item.model_name || '-'}
              </div>
              <div className='text-muted-foreground mt-1'>
                {t('Input tokens')}: {formatTokens(item.prompt_tokens)}
              </div>
            </div>
          </SidebarMenuItem>
        ))}
      </SidebarMenu>
    </SidebarGroup>
  )
}
