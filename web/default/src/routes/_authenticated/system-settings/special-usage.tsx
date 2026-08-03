import { createFileRoute } from '@tanstack/react-router'
import { SpecialUsageMonitor } from '@/features/special-usage'

export const Route = createFileRoute('/_authenticated/system-settings/special-usage')({
  component: SpecialUsageMonitor,
})
