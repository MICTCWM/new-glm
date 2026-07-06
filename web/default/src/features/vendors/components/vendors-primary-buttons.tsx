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
import { Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { useIsAdmin } from '@/hooks/use-admin'
import { VendorMutateDialog } from './vendor-mutate-dialog'

/**
 * Top action buttons for the vendors page.
 * Only admin users can see the "Add Vendor" button.
 */
export function VendorsPrimaryButtons() {
  const { t } = useTranslation()
  const isAdmin = useIsAdmin()
  const [createOpen, setCreateOpen] = useState(false)

  if (!isAdmin) {
    return null
  }

  return (
    <div className='flex items-center gap-2'>
      <Button onClick={() => setCreateOpen(true)} size='sm'>
        <Plus className='h-4 w-4' />
        {t('Add Vendor')}
      </Button>
      <VendorMutateDialog open={createOpen} onOpenChange={setCreateOpen} />
    </div>
  )
}
