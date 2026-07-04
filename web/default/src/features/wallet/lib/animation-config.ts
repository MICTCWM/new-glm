/**
 * 动画性能等级
 */
export enum PerformanceLevel {
  LOW = 'low',
  MEDIUM = 'medium',
  HIGH = 'high',
}

/**
 * 粒子轨迹类型
 */
export enum TrajectoryType {
  SPIRAL = 'spiral',
  PARABOLA = 'parabola',
}

/**
 * 动画配置接口
 */
export interface AnimationConfig {
  particle: {
    count: number
    spawnInterval: number
    minSize: number
    maxSize: number
    minDuration: number
    maxDuration: number
    /** 根据额度差动态调整：额度大时密度更高 */
    densityMultiplier: number
  }
  trajectory: {
    type: TrajectoryType
    parabola: {
      minHeight: number
      maxHeight: number
      maxHeightRatio: number
      spreadRange: number
    }
  }
  ripple: {
    count: number
    maxSize: number
    duration: number
    interval: number
    /** 增强水波：粒子到达后触发层层叠叠的水波 */
    enhanced: {
      enabled: boolean
      /** 每波水波数量（会连续发射多波） */
      waveCount: number
      /** 波与波之间的间隔(ms) */
      waveInterval: number
      /** 每波中的 ripple 数量 */
      ripplesPerWave: number
      /** 最大扩散半径 */
      maxSize: number
      /** 单个水波动画时长 */
      duration: number
      /** 水波扩散间隔 */
      interval: number
    }
  }
  duration: {
    base: number
    max: number
    logScale: number
  }
}

/**
 * 根据性能等级获取动画配置
 */
/**
 * 根据性能等级获取动画配置
 */
export function getAnimationConfig(level: PerformanceLevel): AnimationConfig {
  const configs: Record<PerformanceLevel, AnimationConfig> = {
    [PerformanceLevel.LOW]: {
      particle: {
        count: 12,
        spawnInterval: 50,
        minSize: 3,
        maxSize: 6,
        minDuration: 600,
        maxDuration: 1000,
        densityMultiplier: 1,
      },
      trajectory: {
        type: TrajectoryType.PARABOLA,
        parabola: {
          minHeight: 80,
          maxHeight: 120,
          maxHeightRatio: 0.2,
          spreadRange: 30,
        },
      },
      ripple: {
        count: 2,
        maxSize: 160,
        duration: 500,
        interval: 200,
        enhanced: {
          enabled: true,
          waveCount: 3,
          waveInterval: 200,
          ripplesPerWave: 2,
          maxSize: 120,
          duration: 400,
          interval: 150,
        },
      },
      duration: {
        base: 800,
        max: 10000,
        logScale: 1500,
      },
    },
    [PerformanceLevel.MEDIUM]: {
      particle: {
        count: 18,
        spawnInterval: 40,
        minSize: 3,
        maxSize: 7,
        minDuration: 700,
        maxDuration: 1200,
        densityMultiplier: 1.5,
      },
      trajectory: {
        type: TrajectoryType.PARABOLA,
        parabola: {
          minHeight: 100,
          maxHeight: 180,
          maxHeightRatio: 0.25,
          spreadRange: 45,
        },
      },
      ripple: {
        count: 3,
        maxSize: 180,
        duration: 600,
        interval: 180,
        enhanced: {
          enabled: true,
          waveCount: 4,
          waveInterval: 180,
          ripplesPerWave: 3,
          maxSize: 160,
          duration: 500,
          interval: 130,
        },
      },
      duration: {
        base: 1000,
        max: 15000,
        logScale: 2000,
      },
    },
    [PerformanceLevel.HIGH]: {
      particle: {
        count: 24,
        spawnInterval: 35,
        minSize: 3,
        maxSize: 8,
        minDuration: 800,
        maxDuration: 1400,
        densityMultiplier: 2,
      },
      trajectory: {
        type: TrajectoryType.PARABOLA,
        parabola: {
          minHeight: 120,
          maxHeight: 200,
          maxHeightRatio: 0.3,
          spreadRange: 60,
        },
      },
      ripple: {
        count: 4,
        maxSize: 200,
        duration: 600,
        interval: 160,
        enhanced: {
          enabled: true,
          waveCount: 5,
          waveInterval: 160,
          ripplesPerWave: 4,
          maxSize: 200,
          duration: 600,
          interval: 120,
        },
      },
      duration: {
        base: 1000,
        max: 20000,
        logScale: 2000,
      },
    },
  }
  
  return configs[level]
}
