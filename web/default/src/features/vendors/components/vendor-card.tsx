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
import { memo } from 'react'
import { Check, Pencil, Star, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'
import { SUPPLY_TYPE, type Vendor } from '../types'

interface VendorCardProps {
  vendor: Vendor
  onEdit?: (vendor: Vendor) => void
  onDelete?: (vendor: Vendor) => void
  canManage?: boolean
}

/**
 * Star badge indicating the supply type of a vendor.
 * - Self-supplied (supply_type=0): a single star ★
 * - Partner-supplied (supply_type=1): a star overlaid with a check ★✓
 */
function SupplyTypeBadge(props: { supplyType: number }) {
  const { t } = useTranslation()
  const isPartner = props.supplyType === SUPPLY_TYPE.PARTNER
  const tooltip = isPartner
    ? t('Third-party partner vendor, stable and reliable')
    : t('Self-supplied vendor, non-third-party, more stable')

  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger
          render={
            <span
              className='inline-flex cursor-help items-center'
              aria-label={tooltip}
            />
          }
        >
          <span className='relative inline-flex size-4 items-center justify-center'>
            <Star className='fill-amber-400 text-amber-400 size-4' />
            {isPartner && (
              <Check className='absolute inset-0 m-auto size-3 text-amber-950' />
            )}
          </span>
        </TooltipTrigger>
        <TooltipContent>
          <p>{tooltip}</p>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}

function VendorCardBase(props: VendorCardProps) {
  const { t } = useTranslation()
  const vendor = props.vendor
  const vendorIcon = vendor.icon ? getLobeIcon(vendor.icon, 28) : null
  const initial = vendor.name?.charAt(0).toUpperCase() || '?'
  const canManage = props.canManage ?? false

  return (
    <Card className='group/vendor h-full transition-shadow hover:shadow-md'>
      <CardHeader>
        <CardTitle className='flex items-center gap-2.5'>
          <div className='bg-muted/40 flex size-9 shrink-0 items-center justify-center rounded-lg'>
            {vendorIcon || (
              <span className='text-muted-foreground text-sm font-bold'>
                {initial}
              </span>
            )}
          </div>
          <span className='truncate font-semibold'>{vendor.name}</span>
          <SupplyTypeBadge supplyType={vendor.supply_type ?? SUPPLY_TYPE.SELF} />
        </CardTitle>
        <CardAction>
          {canManage && (
            <div className='flex items-center gap-1 opacity-0 transition-opacity group-hover/vendor:opacity-100'>
              <Button
                type='button'
                variant='ghost'
                size='icon'
                className='size-7'
                onClick={() => props.onEdit?.(vendor)}
                aria-label={t('Edit Vendor')}
              >
                <Pencil className='size-3.5' />
              </Button>
              <Button
                type='button'
                variant='ghost'
                size='icon'
                className={cn(
                  'text-destructive hover:text-destructive size-7'
                )}
                onClick={() => props.onDelete?.(vendor)}
                aria-label={t('Delete')}
              >
                <Trash2 className='size-3.5' />
              </Button>
            </div>
          )}
        </CardAction>
      </CardHeader>
      <CardContent>
        <p
          className='text-muted-foreground line-clamp-3 text-sm'
          title={vendor.description || ''}
        >
          {vendor.description || t('No description')}
        </p>
      </CardContent>
    </Card>
  )
}

export const VendorCard = memo(VendorCardBase)
