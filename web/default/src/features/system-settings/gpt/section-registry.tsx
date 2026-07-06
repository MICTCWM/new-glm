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
import { createSectionRegistry } from '../utils/section-registry'
import { GptChannelsSection } from './components/gpt-channels-section'
import { GptGroupsSection } from './components/gpt-groups-section'

const GPT_SECTIONS = [
  {
    id: 'channels',
    titleKey: 'GPT Channels',
    descriptionKey:
      'GPT channel management, only users with GPT mode enabled can use these channels',
    build: () => <GptChannelsSection />,
  },
  {
    id: 'groups',
    titleKey: 'GPT Group Settings',
    descriptionKey: 'Configure dedicated groups for GPT mode users',
    build: () => <GptGroupsSection />,
  },
] as const

export type GptSectionId = (typeof GPT_SECTIONS)[number]['id']

const gptRegistry = createSectionRegistry<GptSectionId, Record<string, never>>({
  sections: GPT_SECTIONS,
  defaultSection: 'channels',
  basePath: '/system-settings/gpt',
  urlStyle: 'path',
})

export const GPT_SECTION_IDS = gptRegistry.sectionIds
export const GPT_DEFAULT_SECTION = gptRegistry.defaultSection
export const getGptSectionNavItems = gptRegistry.getSectionNavItems
export const getGptSectionContent = gptRegistry.getSectionContent
