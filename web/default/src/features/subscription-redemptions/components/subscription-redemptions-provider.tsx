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
import React, { useState } from 'react'
import useDialogState from '@/hooks/use-dialog'
import {
  type SubscriptionRedemption,
  type SubscriptionRedemptionsDialogType,
} from '../types'

type SubscriptionRedemptionsContextType = {
  open: SubscriptionRedemptionsDialogType | null
  setOpen: (str: SubscriptionRedemptionsDialogType | null) => void
  currentRow: SubscriptionRedemption | null
  setCurrentRow: React.Dispatch<React.SetStateAction<SubscriptionRedemption | null>>
  refreshTrigger: number
  triggerRefresh: () => void
}

const SubscriptionRedemptionsContext = React.createContext<SubscriptionRedemptionsContextType | null>(
  null
)

export function SubscriptionRedemptionsProvider({
  children,
}: {
  children: React.ReactNode
}) {
  const [open, setOpen] = useDialogState<SubscriptionRedemptionsDialogType>(null)
  const [currentRow, setCurrentRow] = useState<SubscriptionRedemption | null>(null)
  const [refreshTrigger, setRefreshTrigger] = useState(0)

  const triggerRefresh = () => setRefreshTrigger((prev) => prev + 1)

  return (
    <SubscriptionRedemptionsContext
      value={{
        open,
        setOpen,
        currentRow,
        setCurrentRow,
        refreshTrigger,
        triggerRefresh,
      }}
    >
      {children}
    </SubscriptionRedemptionsContext>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export const useSubscriptionRedemptions = () => {
  const subscriptionRedemptionsContext = React.useContext(SubscriptionRedemptionsContext)

  if (!subscriptionRedemptionsContext) {
    throw new Error(
      'useSubscriptionRedemptions has to be used within <SubscriptionRedemptionsProvider>'
    )
  }

  return subscriptionRedemptionsContext
}