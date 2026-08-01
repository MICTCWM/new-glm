/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Software License as published by
the Free Software Foundation, either version 3 of the License, or (at your
option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Software License for more details.

You should have received a copy of the GNU Affero General Software License
along with this program. If not, see <https://www.gnu.org/licenses/>.
*/
import { SectionPageLayout } from '@/components/layout'
import { AbilityTable } from './components/ability-table'

export function Ability() {
  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>Channel Abilities</SectionPageLayout.Title>
      <SectionPageLayout.Description>
        View models and their routing abilities across channels.
      </SectionPageLayout.Description>
      <SectionPageLayout.Content>
        <AbilityTable />
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
