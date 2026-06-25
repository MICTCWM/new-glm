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
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'

interface IconCardProps {
  /** The icon identifier to render via getLobeIcon */
  iconName: string | undefined | null
  /** Icon size in pixels, defaults to 32 */
  size?: number
  /** Additional CSS classes to merge */
  className?: string
  /** When true, hides the card from the accessibility tree (e.g. for decorative duplicate elements) */
  'aria-hidden'?: boolean | 'true' | 'false'
}

/**
 * Reusable icon card component with glass morphism effect.
 *
 * Renders an icon inside a glass-style card with a radial glow hover animation.
 * The visual glow element is decorative only and hidden from screen readers.
 */
export function IconCard({
  iconName,
  size = 32,
  className,
  'aria-hidden': ariaHidden,
}: IconCardProps) {
  const isHidden = ariaHidden === true || ariaHidden === 'true'
  const accessibleLabel = iconName ? `Icon card: ${iconName}` : 'Icon card'

  return (
    <div
      role={isHidden ? undefined : 'img'}
      aria-label={isHidden ? undefined : accessibleLabel}
      aria-hidden={isHidden || undefined}
      className={cn(
        'glass-1 group/card',
        'relative overflow-hidden rounded-2xl border p-5',
        'transition-all duration-500 hover:scale-105',
        className
      )}
    >
      {/* Radial glow hover effect — decorative only */}
      <div
        aria-hidden='true'
        className='absolute -top-8 left-1/2 h-16 w-32 -translate-x-1/2 rounded-full bg-radial from-amber-500/10 to-amber-500/0 opacity-0 blur-xl transition-opacity duration-500 group-hover/card:opacity-100'
      />
      <div className='relative flex items-center justify-center'>
        {getLobeIcon(iconName, size)}
      </div>
    </div>
  )
}

IconCard.displayName = 'IconCard'
