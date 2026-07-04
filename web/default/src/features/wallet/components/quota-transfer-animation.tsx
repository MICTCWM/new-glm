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
import { useEffect, useRef } from 'react'

// ============================================================================
// Types
// ============================================================================

export type TransferColor = 'primary' | 'accent'
export type TransferDirection = 'toGpt' | 'toBase'

interface QuotaTransferAnimationProps {
  /** Current value of the source side (base quota) */
  fromValue: number
  /** Current value of the target side (GPT quota) */
  toValue: number
  /** Label for the source side */
  fromLabel: string
  /** Label for the target side */
  toLabel: string
  /** Color theme for the source side */
  fromColor: TransferColor
  /** Color theme for the target side */
  toColor: TransferColor
  /** Whether a transfer is currently in progress */
  isTransferring: boolean
  /** Direction of the current transfer */
  transferDirection: TransferDirection
  /** Ref to the source card DOM element (for particle targeting) */
  fromRef: React.RefObject<HTMLDivElement | null>
  /** Ref to the target card DOM element (for particle targeting) */
  toRef: React.RefObject<HTMLDivElement | null>
  /** Formatter for the source value display */
  formatFromValue: (value: number) => string
  /** Formatter for the target value display */
  formatToValue: (value: number) => string
}

// ============================================================================
// Constants
// ============================================================================

const ANIMATION_DURATION = 1100
const PARTICLE_COUNT = 24

// ============================================================================
// Helpers
// ============================================================================

function colorVar(color: TransferColor): string {
  return color === 'primary' ? 'var(--primary)' : 'var(--accent)'
}

/**
 * Spring physics number animation (direct DOM manipulation, matching demo v3).
 * Uses a damped cosine oscillation on top of easeOutCubic for a lively bounce.
 */
function animateValueSpring(
  el: HTMLElement,
  from: number,
  to: number,
  duration: number,
  formatter: (v: number) => string,
  isRising: boolean,
  cancelled: { current: boolean }
): void {
  const start = performance.now()
  let lastScale = 1

  const frame = (now: number) => {
    if (cancelled.current) {
      el.style.transform = ''
      return
    }
    const t = Math.min((now - start) / duration, 1)
    const decay = Math.exp(-4 * t)
    const osc = Math.cos(12 * t)
    const easeMain = 1 - Math.pow(1 - t, 3)
    const current = from + (to - from) * easeMain
    el.textContent = formatter(current)

    const scale = 1 + decay * Math.abs(osc) * 0.06 * (isRising ? 1 : -1)
    if (Math.abs(scale - lastScale) > 0.001) {
      el.style.transform = `scale(${scale})`
      lastScale = scale
    }

    if (t < 1) {
      requestAnimationFrame(frame)
    } else {
      el.style.transform = ''
    }
  }
  requestAnimationFrame(frame)
}

/**
 * Spawn spiral particles flowing from one card to another using quadratic
 * Bézier curves with a decaying spiral offset (matching demo v3).
 */
function spawnParticles(
  fromEl: HTMLElement,
  toEl: HTMLElement,
  isToGpt: boolean,
  created: Set<HTMLElement>,
  cancelled: { current: boolean }
): void {
  const fr = fromEl.getBoundingClientRect()
  const tr = toEl.getBoundingClientRect()
  const fx = fr.left + fr.width / 2
  const fy = fr.top + fr.height / 2
  const tx = tr.left + tr.width / 2
  const ty = tr.top + tr.height / 2
  const color = isToGpt ? 'var(--primary)' : 'var(--accent)'

  for (let i = 0; i < PARTICLE_COUNT; i++) {
    setTimeout(() => {
      if (cancelled.current) return
      const p = document.createElement('div')
      const size = 3 + Math.random() * 7
      p.style.cssText = `position:fixed;width:${size}px;height:${size}px;border-radius:50%;background:${color};box-shadow:0 0 ${size * 3}px ${color},0 0 ${size * 6}px ${color};pointer-events:none;z-index:999;`
      document.body.appendChild(p)
      created.add(p)

      const dur = 800 + Math.random() * 600
      const arcHeight = 40 + Math.random() * 80
      const swirl = (Math.random() - 0.5) * 100
      const startAngle = Math.random() * Math.PI * 2
      const midX = (fx + tx) / 2 + swirl
      const midY = (fy + ty) / 2 - arcHeight
      const startTime = performance.now()

      const animateP = (now: number) => {
        if (cancelled.current) {
          p.remove()
          created.delete(p)
          return
        }
        const t = Math.min((now - startTime) / dur, 1)
        const ease = 1 - Math.pow(1 - t, 2.5)

        const x = (1 - ease) * (1 - ease) * fx + 2 * (1 - ease) * ease * midX + ease * ease * tx
        const y = (1 - ease) * (1 - ease) * fy + 2 * (1 - ease) * ease * midY + ease * ease * ty

        const spiralR = (1 - ease) * 15
        const spiralA = startAngle + ease * Math.PI * 3
        const sx = x + Math.cos(spiralA) * spiralR
        const sy = y + Math.sin(spiralA) * spiralR

        p.style.left = `${sx - size / 2}px`
        p.style.top = `${sy - size / 2}px`
        p.style.opacity = ease < 0.1 ? ease * 10 : (1 - ease) * 1.2
        p.style.transform = `rotate(${ease * 720}deg) scale(${0.5 + ease * 0.8})`

        if (t < 1) {
          requestAnimationFrame(animateP)
        } else {
          p.remove()
          created.delete(p)
        }
      }
      requestAnimationFrame(animateP)
    }, i * 35)
  }
}

/**
 * Expand concentric ripples on the target card (matching demo v3).
 */
function spawnRipples(
  el: HTMLElement,
  isToGpt: boolean,
  created: Set<HTMLElement>,
  cancelled: { current: boolean }
): void {
  const rect = el.getBoundingClientRect()
  const cx = rect.left + rect.width / 2
  const cy = rect.top + rect.height / 2
  const color = isToGpt ? 'var(--success)' : 'var(--primary)'

  for (let i = 0; i < 3; i++) {
    setTimeout(() => {
      if (cancelled.current) return
      const r = document.createElement('div')
      r.style.cssText = `position:fixed;left:${cx}px;top:${cy}px;border:2px solid ${color};border-radius:50%;transform:translate(-50%,-50%);pointer-events:none;z-index:998;opacity:0;`
      document.body.appendChild(r)
      created.add(r)

      r.animate(
        [
          { width: '0px', height: '0px', opacity: 0.7, borderWidth: '3px' },
          { width: '160px', height: '160px', opacity: 0, borderWidth: '1px' },
        ],
        { duration: 900, easing: 'cubic-bezier(0.33, 1, 0.68, 1)', fill: 'forwards' }
      ).onfinish = () => {
        r.remove()
        created.delete(r)
      }
    }, i * 180)
  }
}

/**
 * Animate a continuous flow of light blobs through the energy pipe.
 */
function flowPipe(
  pipe: HTMLElement,
  isToGpt: boolean,
  created: Set<HTMLElement>,
  cancelled: { current: boolean }
): void {
  const color = isToGpt ? 'var(--primary)' : 'var(--accent)'
  const pipeRect = pipe.getBoundingClientRect()
  const pipeH = pipeRect.height
  let flowCount = 0

  const interval = setInterval(() => {
    if (cancelled.current || flowCount >= 5) {
      clearInterval(interval)
      return
    }

    const flow = document.createElement('div')
    flow.style.cssText = `position:absolute;width:100%;height:30px;border-radius:2px;background:linear-gradient(${isToGpt ? 'to bottom' : 'to top'},transparent,${color},transparent);box-shadow:0 0 16px ${color};opacity:0;pointer-events:none;`
    flow.style.top = isToGpt ? '-30px' : `${pipeH}px`
    pipe.appendChild(flow)
    created.add(flow)

    flow.animate(
      [
        { opacity: 0, top: isToGpt ? '-30px' : `${pipeH}px` },
        { opacity: 1, offset: 0.2 },
        { opacity: 1, offset: 0.8 },
        { opacity: 0, top: isToGpt ? `${pipeH}px` : '-30px' },
      ],
      { duration: 600, easing: 'linear', fill: 'forwards' }
    ).onfinish = () => {
      flow.remove()
      created.delete(flow)
    }

    flowCount++
  }, 120)
}

/**
 * Float a delta badge (e.g. +$10.00) near a value display.
 */
function showDelta(
  badge: HTMLElement,
  text: string,
  isNeg: boolean
): void {
  badge.textContent = text
  badge.style.color = isNeg ? 'var(--destructive)' : 'var(--success)'
  badge.animate(
    [
      { opacity: 0, transform: 'translateY(0) scale(0.8)' },
      { opacity: 1, transform: 'translateY(-10px) scale(1.1)', offset: 0.25 },
      { opacity: 1, transform: 'translateY(-16px) scale(1)', offset: 0.6 },
      { opacity: 0, transform: 'translateY(-32px) scale(0.9)' },
    ],
    { duration: 1500, easing: 'cubic-bezier(0.33, 1, 0.68, 1)', fill: 'forwards' }
  )
}

// ============================================================================
// Component
// ============================================================================

export function QuotaTransferAnimation(props: QuotaTransferAnimationProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const fromValueRef = useRef<HTMLSpanElement>(null)
  const toValueRef = useRef<HTMLSpanElement>(null)
  const fromDeltaRef = useRef<HTMLDivElement>(null)
  const toDeltaRef = useRef<HTMLDivElement>(null)
  const pipeRef = useRef<HTMLDivElement>(null)
  const fromGlowRef = useRef<HTMLDivElement>(null)
  const toGlowRef = useRef<HTMLDivElement>(null)

  // Track the last values seen while idle (used as animation start points)
  const lastIdleFromRef = useRef(props.fromValue)
  const lastIdleToRef = useRef(props.toValue)
  const wasTransferringRef = useRef(false)

  // Track created DOM elements for cleanup
  const createdElementsRef = useRef<Set<HTMLElement>>(new Set())
  const cancelledRef = useRef({ current: false })

  // Update displayed values when not transferring
  useEffect(() => {
    if (props.isTransferring) return
    if (fromValueRef.current) {
      fromValueRef.current.textContent = props.formatFromValue(props.fromValue)
    }
    if (toValueRef.current) {
      toValueRef.current.textContent = props.formatToValue(props.toValue)
    }
    lastIdleFromRef.current = props.fromValue
    lastIdleToRef.current = props.toValue
  }, [props.fromValue, props.toValue, props.isTransferring, props.formatFromValue, props.formatToValue])

  // Trigger the full animation sequence when isTransferring turns true
  useEffect(() => {
    if (!props.isTransferring || wasTransferringRef.current) {
      wasTransferringRef.current = props.isTransferring
      return
    }
    wasTransferringRef.current = true

    const cancelled = cancelledRef.current
    cancelled.current = false

    const isToGpt = props.transferDirection === 'toGpt'
    const startFrom = lastIdleFromRef.current
    const startTo = lastIdleToRef.current
    const endFrom = props.fromValue
    const endTo = props.toValue

    const fromCard = props.fromRef.current
    const toCard = props.toRef.current
    const fromEl = fromValueRef.current
    const toEl = toValueRef.current
    const pipe = pipeRef.current
    const fromGlow = fromGlowRef.current
    const toGlow = toGlowRef.current

    if (!fromEl || !toEl || !fromCard || !toCard || !pipe) return

    // Determine source/target based on direction
    const sourceCard = isToGpt ? fromCard : toCard
    const targetCard = isToGpt ? toCard : fromCard

    // Glow on source (outgoing) and target (incoming)
    sourceCard.style.borderColor = 'color-mix(in oklch, var(--destructive) 40%, transparent)'
    sourceCard.style.boxShadow = '0 0 40px color-mix(in oklch, var(--destructive) 15%, transparent)'

    // Background glow: light up source, then move to target
    const sourceGlow = isToGpt ? fromGlow : toGlow
    const targetGlow = isToGpt ? toGlow : fromGlow
    const sourceRect = sourceCard.getBoundingClientRect()
    const targetRect = targetCard.getBoundingClientRect()

    if (sourceGlow) {
      sourceGlow.style.background = colorVar(props.fromColor)
      sourceGlow.style.left = `${sourceRect.left + sourceRect.width / 2 - 150}px`
      sourceGlow.style.top = `${sourceRect.top + sourceRect.height / 2 - 150}px`
      sourceGlow.style.opacity = '0.25'
      sourceGlow.style.transform = 'scale(1)'
    }

    setTimeout(() => {
      if (cancelled.current) return
      if (sourceGlow) sourceGlow.style.opacity = '0'
      if (targetGlow) {
        targetGlow.style.background = colorVar(props.toColor)
        targetGlow.style.left = `${targetRect.left + targetRect.width / 2 - 150}px`
        targetGlow.style.top = `${targetRect.top + targetRect.height / 2 - 150}px`
        targetGlow.style.opacity = '0.3'
        targetGlow.style.transform = 'scale(1.2)'
      }
      setTimeout(() => {
        if (cancelled.current) return
        if (targetGlow) {
          targetGlow.style.opacity = '0'
          targetGlow.style.transform = 'scale(0.8)'
        }
      }, 800)
    }, 300)

    // Start particle flow, energy pipe, and value springs simultaneously
    spawnParticles(sourceCard, targetCard, isToGpt, createdElementsRef.current, cancelled)
    flowPipe(pipe, isToGpt, createdElementsRef.current, cancelled)

    const fromRising = endFrom > startFrom
    const toRising = endTo > startTo
    animateValueSpring(fromEl, startFrom, endFrom, ANIMATION_DURATION, props.formatFromValue, fromRising, cancelled)
    animateValueSpring(toEl, startTo, endTo, ANIMATION_DURATION, props.formatToValue, toRising, cancelled)

    // Delta badges
    const deltaFrom = endFrom - startFrom
    const deltaTo = endTo - startTo
    if (fromDeltaRef.current && deltaFrom !== 0) {
      showDelta(fromDeltaRef.current, `${deltaFrom >= 0 ? '+' : ''}${props.formatFromValue(deltaFrom)}`, deltaFrom < 0)
    }
    if (toDeltaRef.current && deltaTo !== 0) {
      showDelta(toDeltaRef.current, `${deltaTo >= 0 ? '+' : ''}${props.formatToValue(deltaTo)}`, deltaTo < 0)
    }

    // After 350ms, glow target card and ripple
    setTimeout(() => {
      if (cancelled.current) return
      targetCard.style.borderColor = 'color-mix(in oklch, var(--success) 40%, transparent)'
      targetCard.style.boxShadow = '0 0 40px color-mix(in oklch, var(--success) 18%, transparent)'
      spawnRipples(targetCard, isToGpt, createdElementsRef.current, cancelled)
    }, 350)

    // Cleanup after the full sequence
    const totalDuration = ANIMATION_DURATION + 400
    const cleanupTimer = setTimeout(() => {
      if (cancelled.current) return
      sourceCard.style.borderColor = ''
      sourceCard.style.boxShadow = ''
      targetCard.style.borderColor = ''
      targetCard.style.boxShadow = ''
    }, totalDuration)

    return () => {
      cancelled.current = true
      clearTimeout(cleanupTimer)
      // Remove any lingering dynamically-created elements
      createdElementsRef.current.forEach((el) => {
        if (el.parentNode) el.parentNode.removeChild(el)
      })
      createdElementsRef.current.clear()
      // Reset card styles
      sourceCard.style.borderColor = ''
      sourceCard.style.boxShadow = ''
      targetCard.style.borderColor = ''
      targetCard.style.boxShadow = ''
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [props.isTransferring])

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      cancelledRef.current.current = true
      createdElementsRef.current.forEach((el) => {
        if (el.parentNode) el.parentNode.removeChild(el)
      })
      createdElementsRef.current.clear()
    }
  }, [])

  const fromColorVar = colorVar(props.fromColor)
  const toColorVar = colorVar(props.toColor)

  return (
    <div ref={containerRef} className='relative overflow-visible rounded-lg border bg-muted/10'>
      {/* Background glow elements */}
      <div
        ref={fromGlowRef}
        className='pointer-events-none fixed z-0 size-[300px] rounded-full opacity-0'
        style={{ filter: 'blur(80px)', transition: 'opacity 0.6s, transform 0.8s cubic-bezier(0.33, 1, 0.68, 1)' }}
      />
      <div
        ref={toGlowRef}
        className='pointer-events-none fixed z-0 size-[300px] rounded-full opacity-0'
        style={{ filter: 'blur(80px)', transition: 'opacity 0.6s, transform 0.8s cubic-bezier(0.33, 1, 0.68, 1)' }}
      />

      <div className='relative z-10 flex items-stretch gap-3 p-3 sm:gap-4 sm:p-4'>
        {/* From side */}
        <div className='flex flex-1 flex-col gap-1.5'>
          <div className='text-muted-foreground truncate text-[10px] font-medium tracking-wider uppercase sm:text-xs'>
            {props.fromLabel}
          </div>
          <div className='relative'>
            <span
              ref={fromValueRef}
              className='inline-block font-mono text-base font-bold tabular-nums sm:text-xl'
              style={{ color: fromColorVar, transition: 'text-shadow 0.3s' }}
            />
            <div
              ref={fromDeltaRef}
              className='pointer-events-none absolute -top-1 right-0 font-mono text-xs font-bold tabular-nums opacity-0'
            />
          </div>
        </div>

        {/* Energy pipe */}
        <div
          ref={pipeRef}
          className='relative my-2 w-1.5 shrink-0 overflow-hidden rounded-full'
          style={{ background: 'color-mix(in oklch, var(--muted-foreground) 8%, transparent)' }}
        />

        {/* To side */}
        <div className='flex flex-1 flex-col items-end gap-1.5'>
          <div className='text-muted-foreground truncate text-[10px] font-medium tracking-wider uppercase sm:text-xs'>
            {props.toLabel}
          </div>
          <div className='relative'>
            <span
              ref={toValueRef}
              className='inline-block font-mono text-base font-bold tabular-nums sm:text-xl'
              style={{ color: toColorVar, transition: 'text-shadow 0.3s' }}
            />
            <div
              ref={toDeltaRef}
              className='pointer-events-none absolute -top-1 right-0 font-mono text-xs font-bold tabular-nums opacity-0'
            />
          </div>
        </div>
      </div>
    </div>
  )
}
