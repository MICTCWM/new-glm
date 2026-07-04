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
import { ArrowLeft, ArrowRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

interface TransferModeDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSelect: (direction: 'toGpt' | 'toBase') => void
}

export function TransferModeDialog({
  open,
  onOpenChange,
  onSelect,
}: TransferModeDialogProps) {
  const { t } = useTranslation()

  const handleSelect = (direction: 'toGpt' | 'toBase') => {
    onSelect(direction)
    onOpenChange(false)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-md'>
        <DialogHeader>
          <DialogTitle className='text-xl font-semibold'>
            {t('Transfer Direction')}
          </DialogTitle>
          <DialogDescription>
            {t('Choose a transfer direction')}
          </DialogDescription>
        </DialogHeader>

        <div className='grid gap-3'>
          <button
            type='button'
            onClick={() => handleSelect('toGpt')}
            className='group flex items-center gap-3 rounded-lg border p-4 text-left transition-colors hover:bg-accent hover:text-accent-foreground'
          >
            <div className='text-primary flex size-10 shrink-0 items-center justify-center rounded-lg border'>
              <ArrowRight className='size-5' />
            </div>
            <div className='flex-1'>
              <div className='text-sm font-semibold'>
                {t('Base → GPT')}
              </div>
              <div className='text-muted-foreground text-xs'>
                {t('Convert base balance to GPT-exclusive balance')}
              </div>
            </div>
          </button>

          <button
            type='button'
            onClick={() => handleSelect('toBase')}
            className='group flex items-center gap-3 rounded-lg border p-4 text-left transition-colors hover:bg-accent hover:text-accent-foreground'
          >
            <div className='text-accent flex size-10 shrink-0 items-center justify-center rounded-lg border'>
              <ArrowLeft className='size-5' />
            </div>
            <div className='flex-1'>
              <div className='text-sm font-semibold'>
                {t('GPT → Base')}
              </div>
              <div className='text-muted-foreground text-xs'>
                {t('Convert GPT-exclusive balance to base balance')}
              </div>
            </div>
          </button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
