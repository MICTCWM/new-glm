import { PerformanceLevel } from './animation-config'

/**
 * 检测设备性能等级
 */
export function detectPerformanceLevel(): PerformanceLevel {
  const cores = navigator.hardwareConcurrency || 4
  const memory = (navigator as any).deviceMemory || 4
  const isMobile = /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(
    navigator.userAgent
  )
  const isLowEndMobile = isMobile && (cores < 4 || memory < 4)
  const prefersReducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  
  if (prefersReducedMotion) {
    return PerformanceLevel.LOW
  }
  
  if (isLowEndMobile) {
    return PerformanceLevel.LOW
  }
  
  if (isMobile) {
    return PerformanceLevel.MEDIUM
  }
  
  if (cores >= 8 && memory >= 8) {
    return PerformanceLevel.HIGH
  }
  
  return PerformanceLevel.MEDIUM
}

let cachedPerformanceLevel: PerformanceLevel | null = null

export function getPerformanceLevel(): PerformanceLevel {
  if (cachedPerformanceLevel === null) {
    cachedPerformanceLevel = detectPerformanceLevel()
  }
  return cachedPerformanceLevel
}
