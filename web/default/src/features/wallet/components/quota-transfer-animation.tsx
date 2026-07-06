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
import { getAnimationConfig, AnimationConfig } from '../lib/animation-config'
import { getPerformanceLevel } from '../lib/performance-detector'

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
  /** Formatter for the source value display */
  formatFromValue: (value: number) => string
  /** Formatter for the target value display */
  formatToValue: (value: number) => string
  /** 转换前的源值（用作动画起点，不传则用 fromValue） */
  startFrom?: number
  /** 转换前的目标值（用作动画起点，不传则用 toValue） */
  startTo?: number
}

// ============================================================================
// Helpers
// ============================================================================

/**
 * 根据额度差值动态计算动画时长
 * 使用对数增长，额度越多动画时间越长，最高封顶 20 秒
 * @param delta - 额度差值（绝对值）
 * @param config - 动画配置
 * @returns 动画时长（毫秒）
 */
export function calculateAnimationDuration(
  delta: number,
  config: AnimationConfig
): number {
  const absDelta = Math.abs(delta)
  const logFactor = Math.log1p(absDelta / 1000)
  const calculatedDuration =
    config.duration.base + logFactor * config.duration.logScale
  return Math.min(calculatedDuration, config.duration.max)
}

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
 * Float a delta badge (e.g. +$10.00) near a value display.
 */
function showDelta(badge: HTMLElement, text: string, isNeg: boolean): void {
  badge.textContent = text
  badge.style.color = isNeg ? 'var(--destructive)' : 'var(--success)'
  badge.animate(
    [
      { opacity: 0, transform: 'translateY(0) scale(0.8)' },
      { opacity: 1, transform: 'translateY(-10px) scale(1.1)', offset: 0.25 },
      { opacity: 1, transform: 'translateY(-16px) scale(1)', offset: 0.6 },
      { opacity: 0, transform: 'translateY(-32px) scale(0.9)' },
    ],
    {
      duration: 1500,
      easing: 'cubic-bezier(0.33, 1, 0.68, 1)',
      fill: 'forwards',
    }
  )
}

/**
 * 进度条流光动画：在管道中显示一个单向流动的光斑
 * toGpt（基础→GPT）时从右向左流动，toBase（GPT→基础）时从左向右流动
 */
function flowPipe(
  pipe: HTMLElement,
  isToGpt: boolean,
): void {
  const color = isToGpt ? 'var(--primary)' : 'var(--accent)'
  const pipeRect = pipe.getBoundingClientRect()
  const pipeW = pipeRect.width

  const flow = document.createElement('div')
  flow.style.cssText = `
    position:absolute;
    top:0;
    height:100%;
    width:30px;
    border-radius:2px;
    background:linear-gradient(${isToGpt ? 'to right' : 'to left'},transparent,${color},transparent);
    pointer-events:none;
    opacity:0;
  `

  pipe.appendChild(flow)

  flow.animate(
    [
      { opacity: 0, left: isToGpt ? '-30px' : `${pipeW}px` },
      { opacity: 0.8, offset: 0.15 },
      { opacity: 0.8, offset: 0.85 },
      { opacity: 0, left: isToGpt ? `${pipeW}px` : '-30px' },
    ],
    { duration: 900, easing: 'ease-in-out', fill: 'forwards' }
  ).onfinish = () => {
    flow.remove()
  }
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

  const config = getAnimationConfig(getPerformanceLevel())

  const lastIdleFromRef = useRef(props.startFrom ?? props.fromValue)
  const lastIdleToRef = useRef(props.startTo ?? props.toValue)
  const wasTransferringRef = useRef(false)

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
  }, [
    props.fromValue,
    props.toValue,
    props.isTransferring,
    props.formatFromValue,
    props.formatToValue,
  ])

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
    const startFrom = props.startFrom ?? lastIdleFromRef.current
    const startTo = props.startTo ?? lastIdleToRef.current
    if (props.startFrom !== undefined)
      lastIdleFromRef.current = props.startFrom
    if (props.startTo !== undefined) lastIdleToRef.current = props.startTo
    const endFrom = props.fromValue
    const endTo = props.toValue

    const fromEl = fromValueRef.current
    const toEl = toValueRef.current
    const pipe = pipeRef.current

    if (!fromEl || !toEl || !pipe) return

    const fromRising = endFrom > startFrom
    const toRising = endTo > startTo
    const deltaFrom = endFrom - startFrom
    const deltaTo = endTo - startTo
    const maxDelta = Math.max(Math.abs(deltaFrom), Math.abs(deltaTo))
    const animationDuration = calculateAnimationDuration(maxDelta, config)

    animateValueSpring(
      fromEl,
      startFrom,
      endFrom,
      animationDuration,
      props.formatFromValue,
      fromRising,
      cancelled
    )
    animateValueSpring(
      toEl,
      startTo,
      endTo,
      animationDuration,
      props.formatToValue,
      toRising,
      cancelled
    )

    flowPipe(pipe, isToGpt)

    if (fromDeltaRef.current && deltaFrom !== 0) {
      showDelta(
        fromDeltaRef.current,
        `${deltaFrom >= 0 ? '+' : ''}${props.formatFromValue(deltaFrom)}`,
        deltaFrom < 0
      )
    }
    if (toDeltaRef.current && deltaTo !== 0) {
      showDelta(
        toDeltaRef.current,
        `${deltaTo >= 0 ? '+' : ''}${props.formatToValue(deltaTo)}`,
        deltaTo < 0
      )
    }

    const totalDuration = animationDuration + 400
    const cleanupTimer = setTimeout(() => {
      if (cancelled.current) return
    }, totalDuration)

    return () => {
      cancelled.current = true
      clearTimeout(cleanupTimer)
      createdElementsRef.current.forEach((el) => {
        if (el.parentNode) el.parentNode.removeChild(el)
      })
      createdElementsRef.current.clear()
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
    <div
      ref={containerRef}
      className='relative overflow-visible rounded-lg border bg-muted/10'
    >
      <div className='relative z-10 flex flex-col gap-2 p-3 sm:gap-3 sm:p-4'>
        {/* Top row: Base Balance (left) + GPT Quota (right) */}
        <div className='flex items-center justify-between gap-4'>
          {/* From side (left) */}
          <div className='flex flex-col gap-1'>
            <div className='text-muted-foreground truncate text-[10px] font-medium tracking-wider uppercase sm:text-xs'>
              {props.fromLabel}
            </div>
            <div className='relative'>
              <span
                ref={fromValueRef}
                className='inline-block font-mono text-base font-bold tabular-nums sm:text-xl'
                style={{
                  color: fromColorVar,
                  transition: 'text-shadow 0.3s',
                }}
              />
              <div
                ref={fromDeltaRef}
                className='pointer-events-none absolute -top-1 right-0 font-mono text-xs font-bold tabular-nums opacity-0'
              />
            </div>
          </div>

          {/* To side (right) */}
          <div className='flex flex-col items-end gap-1'>
            <div className='text-muted-foreground truncate text-[10px] font-medium tracking-wider uppercase sm:text-xs'>
              {props.toLabel}
            </div>
            <div className='relative'>
              <span
                ref={toValueRef}
                className='inline-block font-mono text-base font-bold tabular-nums sm:text-xl'
                style={{
                  color: toColorVar,
                  transition: 'text-shadow 0.3s',
                }}
              />
              <div
                ref={toDeltaRef}
                className='pointer-events-none absolute -top-1 right-0 font-mono text-xs font-bold tabular-nums opacity-0'
              />
            </div>
          </div>
        </div>

        {/* Horizontal energy pipe divider */}
        <div
          ref={pipeRef}
          className='relative h-0.5 w-full shrink-0 overflow-hidden rounded-full'
          style={{
            background:
              'color-mix(in oklch, var(--muted-foreground) 8%, transparent)',
          }}
        />
      </div>
    </div>
  )
}