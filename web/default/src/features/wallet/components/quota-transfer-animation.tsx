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
import { getAnimationConfig, AnimationConfig, TrajectoryType } from '../lib/animation-config'
import { getPerformanceLevel } from '../lib/performance-detector'
import { TrajectoryStrategyFactory } from '../lib/trajectory-strategies'

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
  toRef?: React.RefObject<HTMLDivElement | null>
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
 * Spawn parabolic particles flowing from one card to another.
 * Supports dual-source (top/bottom) with automatic fallback for small cards.
 * 增强版：粒子到达目标时产生水波效果，形成两条持续流动的粒子线
 */
function spawnParticles(
  fromEl: HTMLElement,
  toEl: HTMLElement,
  isToGpt: boolean,
  created: Set<HTMLElement>,
  cancelled: { current: boolean },
  config: AnimationConfig,
  /** 额度差值用于动态调整粒子密度 */
  maxDelta: number = 0
): void {
  const rect = fromEl.getBoundingClientRect()
  const cardHeight = rect.height

  // 根据额度差值动态调整粒子数量
  const densityMultiplier =
    maxDelta > 0
      ? Math.min(3, 1 + Math.log1p(maxDelta / 10000) * 0.3)
      : 1
  const adjustedCount = Math.round(
    config.particle.count * densityMultiplier * config.particle.densityMultiplier
  )
  const adjustedInterval = Math.max(
    15,
    config.particle.spawnInterval / (densityMultiplier * 0.7)
  )

  // 检查卡片高度，决定使用单来源还是双来源
  if (cardHeight < 60) {
    const fromCenter = {
      x: rect.left + rect.width / 2,
      y: rect.top + rect.height / 2,
    }
    spawnParticlesFromPoint(
      fromCenter,
      toEl,
      isToGpt,
      created,
      cancelled,
      config,
      adjustedCount,
      adjustedInterval
    )
  } else {
    const safeOffset = Math.min(20, cardHeight / 4)
    const fromTop = {
      x: rect.left + rect.width / 2,
      y: rect.top + safeOffset,
    }
    const fromBottom = {
      x: rect.left + rect.width / 2,
      y: rect.bottom - safeOffset,
    }

    const halfCount = Math.floor(adjustedCount / 2)
    for (let i = 0; i < halfCount; i++) {
      setTimeout(() => {
        spawnSingleParticle(
          fromTop,
          toEl,
          isToGpt,
          created,
          cancelled,
          config,
          true
        )
      }, i * adjustedInterval)
    }
    for (let i = 0; i < halfCount; i++) {
      setTimeout(() => {
        spawnSingleParticle(
          fromBottom,
          toEl,
          isToGpt,
          created,
          cancelled,
          config,
          false
        )
      }, i * adjustedInterval + 12)
    }
  }
}

function spawnParticlesFromPoint(
  from: { x: number; y: number },
  toEl: HTMLElement,
  isToGpt: boolean,
  created: Set<HTMLElement>,
  cancelled: { current: boolean },
  config: AnimationConfig,
  count: number,
  interval: number
): void {
  for (let i = 0; i < count; i++) {
    setTimeout(() => {
      spawnSingleParticle(
        from,
        toEl,
        isToGpt,
        created,
        cancelled,
        config,
        true
      )
    }, i * interval)
  }
}

function spawnSingleParticle(
  from: { x: number; y: number },
  toEl: HTMLElement,
  isToGpt: boolean,
  created: Set<HTMLElement>,
  cancelled: { current: boolean },
  config: AnimationConfig,
  isHighTrajectory: boolean = true
): void {
  if (cancelled.current) return

  const toRect = toEl.getBoundingClientRect()
  const to = {
    x: toRect.left + toRect.width / 2,
    y: toRect.top + toRect.height / 2,
  }

  const trajectory = TrajectoryStrategyFactory.get(TrajectoryType.PARABOLA)

  const particle = document.createElement('div')
  const size =
    config.particle.minSize +
    Math.random() * (config.particle.maxSize - config.particle.minSize)
  const color = isToGpt ? 'var(--primary)' : 'var(--accent)'
  const glowIntensity = isHighTrajectory ? size * 1.5 : size

  particle.style.cssText = `
    position: fixed;
    width: ${size}px;
    height: ${size}px;
    background: ${color};
    border-radius: 50%;
    pointer-events: none;
    z-index: 999;
    box-shadow: 0 0 ${glowIntensity}px ${color};
  `

  document.body.appendChild(particle)
  created.add(particle)

  const durationMultiplier = isHighTrajectory ? 1 : 0.85
  const duration =
    (config.particle.minDuration +
      Math.random() *
        (config.particle.maxDuration - config.particle.minDuration)) *
    durationMultiplier

  const startTime = performance.now()
  let rippleTriggered = false

  const animate = (currentTime: number) => {
    if (cancelled.current) {
      particle.remove()
      created.delete(particle)
      return
    }

    const elapsed = currentTime - startTime
    const progress = Math.min(elapsed / duration, 1)
    const easedProgress = 1 - Math.pow(1 - progress, 3)

    const baseHeight = isHighTrajectory
      ? config.trajectory.parabola.minHeight +
        Math.random() *
          (config.trajectory.parabola.maxHeight -
            config.trajectory.parabola.minHeight)
      : config.trajectory.parabola.minHeight * 0.5 +
        Math.random() *
          (config.trajectory.parabola.maxHeight * 0.5 -
            config.trajectory.parabola.minHeight * 0.4)

    const position = trajectory.calculate(from, to, easedProgress, {
      height: baseHeight,
      maxHeightRatio: config.trajectory.parabola.maxHeightRatio,
      spreadRange: isHighTrajectory
        ? config.trajectory.parabola.spreadRange
        : config.trajectory.parabola.spreadRange * 0.5,
    })

    particle.style.left = position.x + 'px'
    particle.style.top = position.y + 'px'
    particle.style.opacity = (1 - progress * 0.3).toString()
    const arrivalScale = progress > 0.85 ? 1 + (progress - 0.85) * 3 : 1
    particle.style.transform = `scale(${arrivalScale})`

    if (progress >= 0.95 && !rippleTriggered && !cancelled.current) {
      rippleTriggered = true
      spawnMiniRipple(toEl, isToGpt, created, cancelled, config)
    }

    if (progress < 1) {
      requestAnimationFrame(animate)
    } else {
      particle.remove()
      created.delete(particle)
    }
  }

  requestAnimationFrame(animate)
}

/**
 * 单个粒子到达时触发的微水波
 */
function spawnMiniRipple(
  el: HTMLElement,
  isToGpt: boolean,
  created: Set<HTMLElement>,
  cancelled: { current: boolean },
  config: AnimationConfig
): void {
  if (cancelled.current || !config.ripple.enhanced.enabled) return

  const rect = el.getBoundingClientRect()
  const cx = rect.left + rect.width * (0.3 + Math.random() * 0.4)
  const cy = rect.top + rect.height * (0.3 + Math.random() * 0.4)
  const color = isToGpt ? 'var(--success)' : 'var(--primary)'

  const r = document.createElement('div')
  const size = config.ripple.enhanced.maxSize * (0.3 + Math.random() * 0.5)
  r.style.cssText = `
    position:fixed;
    left:${cx}px;
    top:${cy}px;
    border:2px solid ${color};
    border-radius:50%;
    transform:translate(-50%,-50%);
    pointer-events:none;
    z-index:998;
    opacity:0;
  `
  document.body.appendChild(r)
  created.add(r)

  r.animate(
    [
      { width: '2px', height: '2px', opacity: 0.6, borderWidth: '2px' },
      {
        width: `${size}px`,
        height: `${size}px`,
        opacity: 0,
        borderWidth: '0.5px',
      },
    ],
    {
      duration: config.ripple.enhanced.duration,
      easing: 'cubic-bezier(0.33, 1, 0.68, 1)',
      fill: 'forwards',
    }
  ).onfinish = () => {
    r.remove()
    created.delete(r)
  }
}

/**
 * Expand concentric ripples on the target card.
 * 增强版：层层叠叠的水波效果
 */
function spawnRipples(
  el: HTMLElement,
  isToGpt: boolean,
  created: Set<HTMLElement>,
  cancelled: { current: boolean },
  config: AnimationConfig
): void {
  const rect = el.getBoundingClientRect()
  const cx = rect.left + rect.width / 2
  const cy = rect.top + rect.height / 2
  const color = isToGpt ? 'var(--success)' : 'var(--primary)'

  // 普通水波
  for (let i = 0; i < config.ripple.count; i++) {
    setTimeout(() => {
      if (cancelled.current) return
      const r = document.createElement('div')
      r.style.cssText = `position:fixed;left:${cx}px;top:${cy}px;border:2px solid ${color};border-radius:50%;transform:translate(-50%,-50%);pointer-events:none;z-index:998;opacity:0;`
      document.body.appendChild(r)
      created.add(r)

      r.animate(
        [
          { width: '0px', height: '0px', opacity: 0.7, borderWidth: '3px' },
          {
            width: config.ripple.maxSize + 'px',
            height: config.ripple.maxSize + 'px',
            opacity: 0,
            borderWidth: '1px',
          },
        ],
        {
          duration: config.ripple.duration,
          easing: 'cubic-bezier(0.33, 1, 0.68, 1)',
          fill: 'forwards',
        }
      ).onfinish = () => {
        r.remove()
        created.delete(r)
      }
    }, i * config.ripple.interval)
  }

  // 增强水波：层层叠叠连续发射
  if (config.ripple.enhanced.enabled) {
    for (let wave = 0; wave < config.ripple.enhanced.waveCount; wave++) {
      setTimeout(() => {
        if (cancelled.current) return
        for (let j = 0; j < config.ripple.enhanced.ripplesPerWave; j++) {
          setTimeout(() => {
            if (cancelled.current) return
            const r = document.createElement('div')
            const offsetX = (Math.random() - 0.5) * 20
            const offsetY = (Math.random() - 0.5) * 20
            const size =
              config.ripple.enhanced.maxSize * (0.6 + Math.random() * 0.4)
            r.style.cssText = `
              position:fixed;
              left:${cx + offsetX}px;
              top:${cy + offsetY}px;
              border:2px solid ${color};
              border-radius:50%;
              transform:translate(-50%,-50%);
              pointer-events:none;
              z-index:998;
              opacity:0;
            `
            document.body.appendChild(r)
            created.add(r)

            r.animate(
              [
                {
                  width: '0px',
                  height: '0px',
                  opacity: 0.8,
                  borderWidth: '3px',
                },
                {
                  width: `${size}px`,
                  height: `${size}px`,
                  opacity: 0,
                  borderWidth: '1px',
                },
              ],
              {
                duration: config.ripple.enhanced.duration,
                easing: 'cubic-bezier(0.33, 1, 0.68, 1)',
                fill: 'forwards',
              }
            ).onfinish = () => {
              r.remove()
              created.delete(r)
            }
          }, j * config.ripple.enhanced.interval)
        }
      }, wave * config.ripple.enhanced.waveInterval)
    }
  }
}

/**
 * Animate a continuous horizontal flow of light blobs through the energy pipe.
 * 横向流动：toGpt 时从左边流向右边（to right），toBase 时从右边流向左边（to left）
 */
function flowPipe(
  pipe: HTMLElement,
  isToGpt: boolean,
  created: Set<HTMLElement>,
  cancelled: { current: boolean }
): void {
  const color = isToGpt ? 'var(--primary)' : 'var(--accent)'
  const pipeRect = pipe.getBoundingClientRect()
  const pipeW = pipeRect.width
  let flowCount = 0

  const interval = setInterval(() => {
    if (cancelled.current || flowCount >= 5) {
      clearInterval(interval)
      return
    }

    const flow = document.createElement('div')
    flow.style.cssText = `
      position:absolute;
      top:0;
      height:100%;
      width:30px;
      border-radius:2px;
      background:linear-gradient(${isToGpt ? 'to right' : 'to left'},transparent,${color},transparent);
      box-shadow:0 0 16px ${color};
      opacity:0;
      pointer-events:none;
    `
    flow.style.left = isToGpt ? '-30px' : `${pipeW}px`
    pipe.appendChild(flow)
    created.add(flow)

    flow.animate(
      [
        { opacity: 0, left: isToGpt ? '-30px' : `${pipeW}px` },
        { opacity: 1, offset: 0.2 },
        { opacity: 1, offset: 0.8 },
        { opacity: 0, left: isToGpt ? `${pipeW}px` : '-30px' },
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

    const fromCard = props.fromRef.current
    const toCard = props.toRef?.current
    const fromEl = fromValueRef.current
    const toEl = toValueRef.current
    const pipe = pipeRef.current

    if (!fromEl || !toEl || !fromCard || !pipe) return

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

    flowPipe(pipe, isToGpt, createdElementsRef.current, cancelled)
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

    if (!toCard) return

    const sourceCard = isToGpt ? fromCard : toCard
    const targetCard = isToGpt ? toCard : fromCard
    const fromGlow = fromGlowRef.current
    const toGlow = toGlowRef.current

    sourceCard.style.borderColor =
      'color-mix(in oklch, var(--destructive) 40%, transparent)'
    sourceCard.style.boxShadow =
      '0 0 40px color-mix(in oklch, var(--destructive) 15%, transparent)'

    const sourceGlow = isToGpt ? fromGlow : toGlow
    const targetGlow = isToGpt ? toGlow : fromGlow
    const sourceRect = sourceCard.getBoundingClientRect()
    const targetRect = targetCard.getBoundingClientRect()

    if (sourceGlow) {
      sourceGlow.style.background = colorVar(props.fromColor)
      sourceGlow.style.left = `${
        sourceRect.left + sourceRect.width / 2 - 150
      }px`
      sourceGlow.style.top = `${
        sourceRect.top + sourceRect.height / 2 - 150
      }px`
      sourceGlow.style.opacity = '0.25'
      sourceGlow.style.transform = 'scale(1)'
    }

    setTimeout(() => {
      if (cancelled.current) return
      if (sourceGlow) sourceGlow.style.opacity = '0'
      if (targetGlow) {
        targetGlow.style.background = colorVar(props.toColor)
        targetGlow.style.left = `${
          targetRect.left + targetRect.width / 2 - 150
        }px`
        targetGlow.style.top = `${
          targetRect.top + targetRect.height / 2 - 150
        }px`
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

    spawnParticles(
      sourceCard,
      targetCard,
      isToGpt,
      createdElementsRef.current,
      cancelled,
      config,
      maxDelta
    )

    setTimeout(() => {
      if (cancelled.current) return
      targetCard.style.borderColor =
        'color-mix(in oklch, var(--success) 40%, transparent)'
      targetCard.style.boxShadow =
        '0 0 40px color-mix(in oklch, var(--success) 18%, transparent)'
      spawnRipples(
        targetCard,
        isToGpt,
        createdElementsRef.current,
        cancelled,
        config
      )
    }, 350)

    const totalDuration = animationDuration + 400
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
      createdElementsRef.current.forEach((el) => {
        if (el.parentNode) el.parentNode.removeChild(el)
      })
      createdElementsRef.current.clear()
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
    <div
      ref={containerRef}
      className='relative overflow-visible rounded-lg border bg-muted/10'
    >
      {/* Background glow elements */}
      <div
        ref={fromGlowRef}
        className='pointer-events-none fixed z-0 size-[300px] rounded-full opacity-0'
        style={{
          filter: 'blur(80px)',
          transition:
            'opacity 0.6s, transform 0.8s cubic-bezier(0.33, 1, 0.68, 1)',
        }}
      />
      <div
        ref={toGlowRef}
        className='pointer-events-none fixed z-0 size-[300px] rounded-full opacity-0'
        style={{
          filter: 'blur(80px)',
          transition:
            'opacity 0.6s, transform 0.8s cubic-bezier(0.33, 1, 0.68, 1)',
        }}
      />

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