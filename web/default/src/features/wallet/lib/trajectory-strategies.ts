import { TrajectoryType } from './animation-config'

export interface Point {
  x: number
  y: number
}

export interface TrajectoryStrategy {
  calculate(from: Point, to: Point, progress: number, config: any): Point
}

export class ParabolaTrajectory implements TrajectoryStrategy {
  calculate(from: Point, to: Point, progress: number, config: any): Point {
    // 修复1：根据相对位置动态调整抛物线方向
    const direction = to.y < from.y ? -1 : 1
    
    // 修复2：根据屏幕高度动态调整抛物线高度
    const screenHeight = window.innerHeight
    const maxSafeHeight = Math.min(
      config.height,
      screenHeight * config.maxHeightRatio
    )
    
    // 抛物线轨迹
    const parabolaX = from.x + (to.x - from.x) * progress
    const parabolaY = 
      from.y + (to.y - from.y) * progress - 
      direction * Math.sin(progress * Math.PI) * maxSafeHeight
    
    // 水平散开效果
    const spreadX = (Math.random() - 0.5) * config.spreadRange * progress
    
    return {
      x: parabolaX + spreadX,
      y: parabolaY,
    }
  }
}

export class TrajectoryStrategyFactory {
  private static strategies: Map<TrajectoryType, TrajectoryStrategy> = new Map([
    [TrajectoryType.PARABOLA, new ParabolaTrajectory()],
  ])
  
  static get(type: TrajectoryType): TrajectoryStrategy {
    const strategy = this.strategies.get(type)
    if (!strategy) {
      throw new Error(`Unknown trajectory type: ${type}`)
    }
    return strategy
  }
}
