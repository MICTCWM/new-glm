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
import { createFileRoute, redirect } from '@tanstack/react-router'
import { Main } from '@/components/layout'
import { TicketDetail } from '@/features/tickets/components/ticket-detail'
import { useAuthStore } from '@/stores/auth-store'

export const Route = createFileRoute('/_authenticated/tickets/$ticketId')({
  beforeLoad: ({ params }) => {
    const { auth } = useAuthStore.getState()
    if (!auth.user) {
      throw redirect({ to: '/sign-in' })
    }
    const id = Number(params.ticketId)
    if (isNaN(id) || id <= 0) {
      throw redirect({ to: '/tickets' })
    }
  },
  component: TicketDetailPage,
})

function TicketDetailPage() {
  const { ticketId } = Route.useParams()
  const id = Number(ticketId)

  return (
    <Main>
      <TicketDetail ticketId={id} />
    </Main>
  )
}
