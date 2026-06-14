/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useSubscriptionRedemptions } from './subscription-redemptions-provider'
import { SubscriptionRedemptionsMutateDrawer } from './subscription-redemptions-mutate-drawer'
import { SubscriptionRedemptionsDeleteDialog } from './subscription-redemptions-delete-dialog'

export function SubscriptionRedemptionsDialogs() {
  const { open, setOpen, currentRow } = useSubscriptionRedemptions()

  return (
    <>
      <SubscriptionRedemptionsMutateDrawer
        open={open === 'create'}
        onOpenChange={(v) => setOpen(v ? 'create' : null)}
      />
      <SubscriptionRedemptionsMutateDrawer
        open={open === 'update'}
        onOpenChange={(v) => setOpen(v ? 'update' : null)}
        currentRow={currentRow ?? undefined}
      />
      <SubscriptionRedemptionsDeleteDialog
        open={open === 'delete'}
        onOpenChange={(v) => setOpen(v ? 'delete' : null)}
        currentRow={currentRow}
      />
    </>
  )
}